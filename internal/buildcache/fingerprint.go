package buildcache

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
)

type fingerprintHeader struct {
	Schema          int             `json:"schema"`
	Config          types.AppConfig `json:"config"`
	UtilityPath     string          `json:"utilityPath"`
	UtilitySize     int64           `json:"utilitySize"`
	UtilitySHA256   string          `json:"utilitySHA256"`
	UtilityModified int64           `json:"utilityModified"`
}

func Fingerprint(appPath string, cfg *types.AppConfig, outputPath, utilityPath string) (string, error) {
	return FingerprintContext(context.Background(), appPath, cfg, outputPath, utilityPath)
}

// FingerprintContext computes the authoritative content fingerprint. Cancellation
// does not change the fingerprint format or allow metadata to replace file reads.
func FingerprintContext(ctx context.Context, appPath string, cfg *types.AppConfig, outputPath, utilityPath string) (string, error) {
	f, err := NewFingerprinter(ctx, utilityPath)
	if err != nil {
		return "", err
	}
	return f.Fingerprint(ctx, appPath, cfg, outputPath)
}

// Fingerprinter holds one authoritative utility identity for a single build or
// inventory operation. Discard it afterwards so the next operation rehashes the
// tool. Source contents are always hashed independently for every application.
// Each application rechecks tool size/mtime and rehashes on change. A concurrent
// same-size/same-mtime tool edit retains the initially observed identity until
// the next operation. Fingerprinter is not safe for concurrent use.
type Fingerprinter struct {
	utilityPath     string
	utilitySize     int64
	utilityModified int64
	utilitySHA256   string
	openTool        func(string) (io.ReadCloser, error)
}

// NewFingerprinter reads and hashes the packaging utility exactly once.
func NewFingerprinter(ctx context.Context, utilityPath string) (*Fingerprinter, error) {
	return newFingerprinter(ctx, utilityPath, func(path string) (io.ReadCloser, error) { return os.Open(path) })
}

func newFingerprinter(ctx context.Context, utilityPath string, open func(string) (io.ReadCloser, error)) (*Fingerprinter, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	utilityInfo, err := os.Stat(utilityPath)
	if err != nil {
		return nil, fmt.Errorf("inspect packaging utility: %w", err)
	}
	tool, err := open(utilityPath)
	if err != nil {
		return nil, err
	}
	toolHash := sha256.New()
	n, copyErr := io.Copy(toolHash, contextReader{ctx: ctx, reader: tool})
	RecordToolHash(ctx, n)
	closeErr := tool.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Fingerprinter{
		utilityPath:     filepath.Clean(utilityPath),
		utilitySize:     utilityInfo.Size(),
		utilityModified: utilityInfo.ModTime().UnixNano(),
		utilitySHA256:   hex.EncodeToString(toolHash.Sum(nil)),
		openTool:        open,
	}, nil
}

// Fingerprint hashes an application's source using the captured tool identity.
// Its digest format is exactly the same as the standalone Fingerprint function.
func (f *Fingerprinter) Fingerprint(ctx context.Context, appPath string, cfg *types.AppConfig, outputPath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	info, err := os.Stat(f.utilityPath)
	if err != nil {
		return "", fmt.Errorf("inspect packaging utility: %w", err)
	}
	if info.Size() != f.utilitySize || info.ModTime().UnixNano() != f.utilityModified {
		updated, err := newFingerprinter(ctx, f.utilityPath, f.openTool)
		if err != nil {
			return "", err
		}
		*f = *updated
	}
	header, err := json.Marshal(fingerprintHeader{
		Schema:          currentSchema,
		Config:          *cfg,
		UtilityPath:     f.utilityPath,
		UtilitySize:     f.utilitySize,
		UtilityModified: f.utilityModified,
		UtilitySHA256:   f.utilitySHA256,
	})
	if err != nil {
		return "", fmt.Errorf("encode build inputs: %w", err)
	}
	hasher := sha256.New()
	writeChunk(hasher, header)

	sourcePath := filepath.Join(appPath, pathutil.Native(cfg.SourceDirectory))
	var paths []string
	err = filepath.WalkDir(sourcePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != sourcePath && (entry.Name() == ".git" || samePath(path, outputPath)) {
				return filepath.SkipDir
			}
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan source directory: %w", err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		relative, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return "", fmt.Errorf("resolve source file path: %w", err)
		}
		writeChunk(hasher, []byte(filepath.ToSlash(relative)))
		info, err := os.Lstat(path)
		if err != nil {
			return "", fmt.Errorf("inspect source file %q: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("source entry %q is a symbolic link; use regular source files", relative)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("source entry %q is not a regular file", relative)
		}
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("open source file %q: %w", relative, err)
		}
		_ = binary.Write(hasher, binary.LittleEndian, uint64(info.Size()))
		n, copyErr := io.Copy(hasher, contextReader{ctx: ctx, reader: file})
		RecordSourceHash(ctx, n)
		closeErr := file.Close()
		if copyErr != nil {
			return "", fmt.Errorf("hash source file %q: %w", relative, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close source file %q: %w", relative, closeErr)
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func writeChunk(hasher hash.Hash, value []byte) {
	_ = binary.Write(hasher, binary.LittleEndian, uint64(len(value)))
	_, _ = hasher.Write(value)
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
}

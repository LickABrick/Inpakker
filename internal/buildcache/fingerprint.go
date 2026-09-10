package buildcache

import (
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
	utilityInfo, err := os.Stat(utilityPath)
	if err != nil {
		return "", fmt.Errorf("inspect packaging utility: %w", err)
	}
	tool, err := os.Open(utilityPath)
	if err != nil {
		return "", err
	}
	toolHash := sha256.New()
	_, copyErr := io.Copy(toolHash, tool)
	closeErr := tool.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	header, err := json.Marshal(fingerprintHeader{
		Schema:          currentSchema,
		Config:          *cfg,
		UtilityPath:     filepath.Clean(utilityPath),
		UtilitySize:     utilityInfo.Size(),
		UtilityModified: utilityInfo.ModTime().UnixNano(),
		UtilitySHA256:   hex.EncodeToString(toolHash.Sum(nil)),
	})
	if err != nil {
		return "", fmt.Errorf("encode build inputs: %w", err)
	}
	hasher := sha256.New()
	writeChunk(hasher, header)

	sourcePath := filepath.Join(appPath, pathutil.Native(cfg.SourceDirectory))
	var paths []string
	err = filepath.WalkDir(sourcePath, func(path string, entry fs.DirEntry, walkErr error) error {
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
		_, copyErr := io.Copy(hasher, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", fmt.Errorf("hash source file %q: %w", relative, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close source file %q: %w", relative, closeErr)
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
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

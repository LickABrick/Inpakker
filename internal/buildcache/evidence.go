package buildcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/LickABrick/inpakker/internal/atomicfile"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
)

const (
	EvidenceFileName = "inspection-evidence.json"
	evidenceSchema = 1
)

// Metadata is a cheap snapshot of effective inputs, not a content fingerprint.
// Equal size/mtime content edits can escape it. Never use it to skip a build.
// Files counts inspected source files (the utility is not included).
// MetadataDigest reads no source or utility file contents.
type Metadata struct {
	Digest string `json:"digest"`
	Files int `json:"files"`
}

type metadataFile struct {
	Path string `json:"path"`
	Size int64 `json:"size"`
	Modified int64 `json:"modified"`
	Mode uint32 `json:"mode"`
}

// MetadataDigest honors the authoritative scan's output/.git exclusions and
// rejects source symlinks, special files and paths outside the application.
func MetadataDigest(ctx context.Context, appPath string, cfg *types.AppConfig, outputPath, utilityPath string) (Metadata, error) {
	if err := ctx.Err(); err != nil { return Metadata{}, err }
	if cfg == nil { return Metadata{}, fmt.Errorf("inspect metadata: application config is nil") }
	if !pathutil.ValidRelative(cfg.SourceDirectory) { return Metadata{}, fmt.Errorf("inspect metadata: invalid source directory %q", cfg.SourceDirectory) }
	appRoot, err := filepath.Abs(appPath)
	if err != nil { return Metadata{}, err }
	sourcePath := filepath.Join(appPath, pathutil.Native(cfg.SourceDirectory))
	if err := pathutil.Within(appPath, sourcePath); err != nil { return Metadata{}, fmt.Errorf("inspect source directory: %w", err) }
	rootInfo, err := os.Lstat(sourcePath)
	if err != nil { return Metadata{}, fmt.Errorf("inspect source directory: %w", err) }
	if !rootInfo.IsDir() { return Metadata{}, fmt.Errorf("source directory is not a regular directory") }
	utilityInfo, err := os.Stat(utilityPath)
	if err != nil { return Metadata{}, fmt.Errorf("inspect packaging utility: %w", err) }
	if !utilityInfo.Mode().IsRegular() { return Metadata{}, fmt.Errorf("packaging utility is not a regular file") }
	utilityRoot, err := filepath.Abs(utilityPath)
	if err != nil { return Metadata{}, err }
	outputRoot, err := filepath.Abs(outputPath)
	if err != nil { return Metadata{}, err }
	header, err := json.Marshal(struct {
		Schema int
		Config types.AppConfig
		AppPath string
		OutputPath string
		Utility metadataFile
	}{evidenceSchema, *cfg, appRoot, outputRoot, metadataFile{utilityRoot, utilityInfo.Size(), utilityInfo.ModTime().UnixNano(), uint32(utilityInfo.Mode())}})
	if err != nil { return Metadata{}, fmt.Errorf("encode inspection metadata: %w", err) }
	hasher := sha256.New()
	writeChunk(hasher, header)
	count := 0
	err = filepath.WalkDir(sourcePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil { return err }
		if walkErr != nil { return walkErr }
		if entry.IsDir() {
			if path != sourcePath && (entry.Name() == ".git" || samePath(path, outputPath)) { return filepath.SkipDir }
			if path != sourcePath && !pathutil.ValidName(entry.Name()) { return fmt.Errorf("invalid source directory %q", path) }
			return nil
		}
		relative, err := filepath.Rel(sourcePath, path)
		if err != nil { return err }
		if !pathutil.ValidRelative(relative) { return fmt.Errorf("invalid source path %q", relative) }
		info, err := os.Lstat(path)
		if err != nil { return err }
		if info.Mode()&os.ModeSymlink != 0 { return fmt.Errorf("source entry %q is a symbolic link; use regular source files", relative) }
		if !info.Mode().IsRegular() { return fmt.Errorf("source entry %q is not a regular file", relative) }
		data, err := json.Marshal(metadataFile{filepath.ToSlash(relative), info.Size(), info.ModTime().UnixNano(), uint32(info.Mode())})
		if err != nil { return err }
		writeChunk(hasher, data)
		count++
		return nil
	})
	if err != nil { return Metadata{}, fmt.Errorf("scan source metadata: %w", err) }
	if err := ctx.Err(); err != nil { return Metadata{}, err }
	return Metadata{Digest: hex.EncodeToString(hasher.Sum(nil)), Files: count}, nil
}

// InspectionEvidence is disposable user-local state, separate from successful
// builds. It may describe a changed input fingerprint as well as a current one.
// Callers must serialize mutation/save, as with Cache, and bypass all evidence
// operations when --no-cache is selected.
type InspectionEvidence struct {
	Schema int `json:"schemaVersion"`
	Apps map[string]EvidenceEntry `json:"apps"`
}

type EvidenceEntry struct {
	MetadataDigest string `json:"metadataDigest"`
	Fingerprint string `json:"fingerprint"`
}

func LoadEvidence(ctx context.Context, stateDirectory string) (*InspectionEvidence, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	path := filepath.Join(stateDirectory, EvidenceFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) { return &InspectionEvidence{Schema: evidenceSchema, Apps: make(map[string]EvidenceEntry)}, nil }
	if err != nil { return nil, fmt.Errorf("read inspection evidence: %w", err) }
	if err := ctx.Err(); err != nil { return nil, err }
	var evidence InspectionEvidence
	if err := json.Unmarshal(data, &evidence); err != nil { return nil, fmt.Errorf("read inspection evidence %q: %w", path, err) }
	if evidence.Schema != evidenceSchema || evidence.Apps == nil { return nil, fmt.Errorf("unsupported inspection evidence schemaVersion %d", evidence.Schema) }
	return &evidence, nil
}

// Lookup returns the last authoritative fingerprint only when metadata matches.
// The caller must still check Cache.Current for the expected artifact(s). A hit
// is a fast status hint, not proof that file contents are unchanged.
func (e *InspectionEvidence) Lookup(key string, metadata Metadata) (string, bool) {
	if e == nil || metadata.Digest == "" { return "", false }
	entry, ok := e.Apps[key]
	return entry.Fingerprint, ok && entry.Fingerprint != "" && entry.MetadataDigest == metadata.Digest
}

// Record associates a successful authoritative FingerprintContext result with
// metadata captured immediately before and after that inspection/build. If
// inputs visibly changed during the operation, old evidence is invalidated.
// Metadata cannot detect all concurrent edits; builds must always hash content.
func (e *InspectionEvidence) Record(key string, before, after Metadata, fingerprint string) bool {
	if before.Digest == "" || before != after || fingerprint == "" {
		delete(e.Apps, key)
		return false
	}
	if e.Apps == nil { e.Apps = make(map[string]EvidenceEntry) }
	e.Apps[key] = EvidenceEntry{MetadataDigest: after.Digest, Fingerprint: fingerprint}
	return true
}

// Save atomically persists evidence under the caller's user-local state path.
// Cancellation is checked before committing; an in-progress atomic write is not
// interruptible and may complete if cancellation arrives during the write.
func (e *InspectionEvidence) Save(ctx context.Context, stateDirectory string) error {
	if err := ctx.Err(); err != nil { return err }
	return atomicfile.JSON(filepath.Join(stateDirectory, EvidenceFileName), e, 0600)
}

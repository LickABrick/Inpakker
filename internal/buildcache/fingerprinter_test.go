package buildcache

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprinterReusesToolHashAndPreservesDigest(t *testing.T) {
	app, cfg, output, tool := evidenceFixture(t)
	ctx := context.Background()
	opens, bytesRead := 0, int64(0)
	f, err := newFingerprinter(ctx, tool, func(path string) (io.ReadCloser, error) {
		opens++
		file, err := os.Open(path)
		if err != nil { return nil, err }
		return &countedTool{ReadCloser: file, bytes: &bytesRead}, nil
	})
	if err != nil { t.Fatal(err) }
	// Independently reproduce the original single-file digest, including header
	// field ordering, chunk framing and little-endian size encoding.
	info, err := os.Stat(tool)
	if err != nil { t.Fatal(err) }
	toolBytes, err := os.ReadFile(tool)
	if err != nil { t.Fatal(err) }
	toolHash := sha256.Sum256(toolBytes)
	header, err := json.Marshal(fingerprintHeader{
		Schema: currentSchema, Config: *cfg,
		UtilityPath: filepath.Clean(tool), UtilitySize: info.Size(),
		UtilitySHA256: hex.EncodeToString(toolHash[:]), UtilityModified: info.ModTime().UnixNano(),
	})
	if err != nil { t.Fatal(err) }
	h := sha256.New()
	writeChunk(h, header)
	writeChunk(h, []byte("setup.ps1"))
	_ = binary.Write(h, binary.LittleEndian, uint64(len("first")))
	_, _ = h.Write([]byte("first"))
	want := hex.EncodeToString(h.Sum(nil))
	for i := 0; i < 3; i++ {
		got, err := f.Fingerprint(ctx, app, cfg, output)
		if err != nil { t.Fatal(err) }
		if got != want { t.Fatalf("shared digest = %q, original = %q", got, want) }
	}
	if opens != 1 || bytesRead != int64(len(toolBytes)) { t.Fatalf("tool opens = %d, bytes = %d", opens, bytesRead) }
	for _, standalone := range []func() (string, error){
		func() (string, error) { return Fingerprint(app, cfg, output, tool) },
		func() (string, error) { return FingerprintContext(ctx, app, cfg, output, tool) },
	} {
		got, err := standalone()
		if err != nil { t.Fatal(err) }
		if got != want { t.Fatalf("standalone digest = %q, original = %q", got, want) }
	}
	writeFile(t, filepath.Join(app, "source", "setup.ps1"), "other")
	changed, err := f.Fingerprint(ctx, app, cfg, output)
	if err != nil { t.Fatal(err) }
	if changed == want { t.Fatal("shared fingerprinter reused source content") }
	if opens != 1 { t.Fatal("shared fingerprinter reopened tool") }
	writeFile(t, tool, "utility-with-new-size")
	got, err := f.Fingerprint(ctx, app, cfg, output)
	if err != nil { t.Fatal(err) }
	wantUpdated, err := FingerprintContext(ctx, app, cfg, output, tool)
	if err != nil { t.Fatal(err) }
	if got != wantUpdated { t.Fatal("changed utility identity was not refreshed") }
	if opens != 2 { t.Fatalf("tool opens after modification = %d", opens) }
}

type countedTool struct {
	io.ReadCloser
	bytes *int64
}

func (r *countedTool) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	*r.bytes += int64(n)
	return n, err
}

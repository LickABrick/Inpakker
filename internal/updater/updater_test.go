package updater

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	selfupdate "github.com/creativeprojects/go-selfupdate/update"
)

func TestCheckUsesDailyCacheAndReportsNewerVersion(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeRelease(t, w, serverURL(r), "0.3.0")
	}))
	defer server.Close()

	service := testService(t, server.URL, now)
	first, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("first Check returned %v", err)
	}
	if !first.Available || first.LatestVersion != "0.3.0" || first.Cached {
		t.Fatalf("first result = %#v", first)
	}
	second, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("second Check returned %v", err)
	}
	if !second.Available || !second.Cached {
		t.Fatalf("second result = %#v", second)
	}
	if requests != 1 {
		t.Fatalf("release endpoint called %d times, want 1", requests)
	}
}

func TestCheckRequiresSignedReleaseAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := releaseResponse{TagName: "v0.3.0", HTMLURL: "https://example.test/release"}
		payload.Assets = append(payload.Assets,
			struct {
				Name string `json:"name"`
				URL  string `json:"browser_download_url"`
			}{Name: "inpakker_v0.3.0_windows_amd64.zip", URL: "https://example.test/archive"},
			struct {
				Name string `json:"name"`
				URL  string `json:"browser_download_url"`
			}{Name: "inpakker_v0.3.0_checksums.txt", URL: "https://example.test/checksums"},
		)
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()
	service := testService(t, server.URL, time.Now())
	if _, err := service.Check(context.Background(), false); err == nil {
		t.Fatal("Check accepted a release without a signature")
	}
}

func TestInstallVerifiesSignedArchiveBeforeApplying(t *testing.T) {
	certificate, privateKey := signingMaterial(t)
	archive := makeArchive(t, []byte("new executable"))
	archiveName := "inpakker_v0.3.0_windows_amd64.zip"
	digest := sha256.Sum256(archive)
	checksums := []byte(hex.EncodeToString(digest[:]) + "  " + archiveName + "\n")
	signature := sign(t, privateKey, checksums)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/archive":
			_, _ = w.Write(archive)
		case "/checksums":
			_, _ = w.Write(checksums)
		case "/signature":
			_, _ = w.Write(signature)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	applied := false
	service := testService(t, server.URL, time.Now())
	service.Certificate = certificate
	service.Executable = func() (string, error) { return filepath.Join(t.TempDir(), "inpakker.exe"), nil }
	service.Apply = func(reader io.Reader, options selfupdate.Options) error {
		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		if string(data) != "new executable" || filepath.Base(options.TargetPath) != "inpakker.exe" {
			t.Fatalf("unexpected apply input %q to %q", data, options.TargetPath)
		}
		applied = true
		return nil
	}
	result := Result{Available: true, release: &release{
		ArchiveName: archiveName,
		ArchiveURL:  server.URL + "/archive", ChecksumURL: server.URL + "/checksums", SignatureURL: server.URL + "/signature",
	}}
	var phases []string
	if err := service.Install(context.Background(), result, func(event Event) { phases = append(phases, event.Phase) }); err != nil {
		t.Fatalf("Install returned %v", err)
	}
	if !applied || len(phases) != 5 {
		t.Fatalf("applied = %v, phases = %v", applied, phases)
	}
}

func TestInstallRejectsTamperedChecksumManifest(t *testing.T) {
	certificate, privateKey := signingMaterial(t)
	archive := makeArchive(t, []byte("new executable"))
	checksums := []byte("not the correct checksum  inpakker_v0.3.0_windows_amd64.zip\n")
	signature := sign(t, privateKey, checksums)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/archive":
			_, _ = w.Write(archive)
		case "/checksums":
			_, _ = w.Write(checksums)
		case "/signature":
			_, _ = w.Write(signature)
		}
	}))
	defer server.Close()
	service := testService(t, server.URL, time.Now())
	service.Certificate = certificate
	service.Apply = func(io.Reader, selfupdate.Options) error {
		t.Fatal("Apply was called for a bad checksum")
		return nil
	}
	result := Result{Available: true, release: &release{
		ArchiveName: "inpakker_v0.3.0_windows_amd64.zip",
		ArchiveURL:  server.URL + "/archive", ChecksumURL: server.URL + "/checksums", SignatureURL: server.URL + "/signature",
	}}
	if err := service.Install(context.Background(), result, nil); err == nil {
		t.Fatal("Install accepted a bad checksum")
	}
}

func TestDevelopmentBuildDoesNotContactGitHub(t *testing.T) {
	service := New("dev")
	service.APIURL = "::invalid::"
	if _, err := service.Check(context.Background(), false); err != ErrDevelopmentBuild {
		t.Fatalf("Check error = %v, want ErrDevelopmentBuild", err)
	}
}

func testService(t *testing.T, endpoint string, now time.Time) *Service {
	t.Helper()
	service := New("0.2.0")
	service.APIURL = endpoint
	service.StatePath = filepath.Join(t.TempDir(), "update-state.json")
	service.Now = func() time.Time { return now }
	service.GOOS, service.GOARCH = "windows", "amd64"
	return service
}

func writeRelease(t *testing.T, writer http.ResponseWriter, baseURL, version string) {
	t.Helper()
	archive := "inpakker_v" + version + "_windows_amd64.zip"
	checksums := "inpakker_v" + version + "_checksums.txt"
	payload := releaseResponse{TagName: "v" + version, HTMLURL: "https://example.test/releases/v" + version}
	for _, item := range []struct{ name, path string }{
		{archive, "/archive"}, {checksums, "/checksums"}, {checksums + ".sig", "/signature"},
	} {
		payload.Assets = append(payload.Assets, struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{Name: item.name, URL: baseURL + item.path})
	}
	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		t.Fatalf("encode release: %v", err)
	}
}

func serverURL(request *http.Request) string {
	return "http://" + request.Host
}

func makeArchive(t *testing.T, executable []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	entry, err := archive.Create("inpakker.exe")
	if err == nil {
		_, err = entry.Write(executable)
	}
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	return output.Bytes()
}

func signingMaterial(t *testing.T) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Inpakker Test Signing"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), key
}

func sign(t *testing.T, key *ecdsa.PrivateKey, contents []byte) []byte {
	t.Helper()
	digest := sha256.Sum256(contents)
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	result, err := asn1.Marshal(ecdsaSignature{R: r, S: s})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

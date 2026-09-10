package updater

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/Masterminds/semver/v3"
	selfupdate "github.com/creativeprojects/go-selfupdate/update"
)

const (
	RepositoryURL = "https://github.com/LickABrick/Inpakker"
	APIURL        = "https://api.github.com/repos/LickABrick/Inpakker/releases/latest"
	CheckInterval = 24 * time.Hour
	maxMetadata   = 2 << 20
	maxArchive    = 100 << 20
	maxChecksum   = 1 << 20
	maxSignature  = 64 << 10
)

var (
	ErrDevelopmentBuild = errors.New("development builds cannot be updated automatically")
	ErrUnsupported      = errors.New("no update artifact is published for this platform")
	//go:embed release-signing-cert.pem
	releaseCertificate []byte
)

type Event struct {
	Current int
	Total   int
	Phase   string
}

type Result struct {
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion,omitempty"`
	Available      bool      `json:"available"`
	ReleaseURL     string    `json:"releaseUrl,omitempty"`
	CheckedAt      time.Time `json:"checkedAt"`
	Cached         bool      `json:"cached"`
	release        *release
}

type Service struct {
	CurrentVersion string
	Client         *http.Client
	APIURL         string
	StatePath      string
	Now            func() time.Time
	GOOS           string
	GOARCH         string
	Certificate    []byte
	Executable     func() (string, error)
	Apply          func(io.Reader, selfupdate.Options) error
}

type release struct {
	Version      string
	URL          string
	ArchiveName  string
	ArchiveURL   string
	ChecksumName string
	ChecksumURL  string
	SignatureURL string
}

type releaseResponse struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

type state struct {
	CheckedAt     time.Time `json:"checkedAt"`
	LatestVersion string    `json:"latestVersion,omitempty"`
	ReleaseURL    string    `json:"releaseUrl,omitempty"`
	CLINotifiedAt time.Time `json:"cliNotifiedAt,omitempty"`
}

type ecdsaSignature struct {
	R *big.Int
	S *big.Int
}

func New(currentVersion string) *Service {
	return &Service{
		CurrentVersion: currentVersion,
		Client:         &http.Client{Timeout: 5 * time.Second},
		APIURL:         APIURL,
		Now:            time.Now,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		Certificate:    releaseCertificate,
		Executable:     os.Executable,
		Apply:          selfupdate.Apply,
	}
}

func (s *Service) Check(ctx context.Context, useDailyCache bool) (Result, error) {
	current, err := parseCurrent(s.CurrentVersion)
	if err != nil {
		return Result{}, err
	}
	now := s.now().UTC()
	existing, hasExisting := s.loadState()
	if useDailyCache {
		if hasExisting && now.Sub(existing.CheckedAt) >= 0 && now.Sub(existing.CheckedAt) < CheckInterval {
			return resultFromState(current, existing, true), nil
		}
	}

	latest, err := s.fetchLatest(ctx)
	checked := existing
	checked.CheckedAt = now
	if err != nil {
		_ = s.saveState(checked)
		return Result{}, err
	}
	latestVersion, err := semver.NewVersion(latest.Version)
	if err != nil {
		return Result{}, fmt.Errorf("parse latest release version %q: %w", latest.Version, err)
	}
	if latestVersion.GreaterThan(current) {
		if err := validateReleaseAssets(latest); err != nil {
			_ = s.saveState(checked)
			return Result{}, err
		}
	}
	checked.LatestVersion, checked.ReleaseURL = latest.Version, latest.URL
	if err := s.saveState(checked); err != nil {
		return Result{}, err
	}
	result := resultFromState(current, checked, false)
	result.release = latest
	return result, nil
}

func (s *Service) MarkCLINotified() error {
	current, ok := s.loadState()
	if !ok {
		return nil
	}
	current.CLINotifiedAt = s.now().UTC()
	return s.saveState(current)
}

func (s *Service) CLINoticeDue() bool {
	current, ok := s.loadState()
	if !ok || current.LatestVersion == "" {
		return false
	}
	return current.CLINotifiedAt.IsZero() || s.now().UTC().Sub(current.CLINotifiedAt) >= CheckInterval
}

func (s *Service) Install(ctx context.Context, result Result, onProgress func(Event)) error {
	if result.release == nil {
		return errors.New("fresh release metadata is required before updating")
	}
	if !result.Available {
		return errors.New("no newer release is available")
	}
	emit(onProgress, 0, 4, "downloading package")
	archive, err := s.download(ctx, result.release.ArchiveURL, maxArchive)
	if err != nil {
		return fmt.Errorf("download update package: %w", err)
	}
	emit(onProgress, 1, 4, "downloading verification files")
	checksums, err := s.download(ctx, result.release.ChecksumURL, maxChecksum)
	if err != nil {
		return fmt.Errorf("download checksum manifest: %w", err)
	}
	signature, err := s.download(ctx, result.release.SignatureURL, maxSignature)
	if err != nil {
		return fmt.Errorf("download checksum signature: %w", err)
	}
	emit(onProgress, 2, 4, "verifying signature and checksum")
	if err := verifySignature(s.Certificate, checksums, signature); err != nil {
		return err
	}
	if err := verifyChecksum(result.release.ArchiveName, archive, checksums); err != nil {
		return err
	}
	binary, err := executableFromArchive(archive, s.GOOS)
	if err != nil {
		return err
	}
	target, err := s.executable()()
	if err != nil {
		return fmt.Errorf("locate running executable: %w", err)
	}
	emit(onProgress, 3, 4, "installing update")
	if err := s.apply()(bytes.NewReader(binary), selfupdate.Options{TargetPath: target}); err != nil {
		if rollbackErr := selfupdate.RollbackError(err); rollbackErr != nil {
			return fmt.Errorf("install update and restore previous executable: %v (rollback failed: %w)", err, rollbackErr)
		}
		return fmt.Errorf("install update: %w", err)
	}
	emit(onProgress, 4, 4, "update installed")
	return nil
}

func (s *Service) fetchLatest(ctx context.Context) (*release, error) {
	if s.GOOS != "windows" || s.GOARCH != "amd64" {
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupported, s.GOOS, s.GOARCH)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "inpakker/"+s.CurrentVersion)
	response, err := s.client().Do(request)
	if err != nil {
		return nil, fmt.Errorf("check GitHub releases: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check GitHub releases: server returned %s", response.Status)
	}
	var payload releaseResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxMetadata))
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode GitHub release: %w", err)
	}
	if payload.Draft || payload.Prerelease {
		return nil, errors.New("GitHub returned a draft or prerelease as the latest stable release")
	}
	version, err := semver.NewVersion(payload.TagName)
	if err != nil {
		return nil, fmt.Errorf("parse latest release version %q: %w", payload.TagName, err)
	}
	versionText := version.String()
	wanted := map[string]*string{}
	archiveName := fmt.Sprintf("inpakker_v%s_%s_%s.zip", versionText, s.GOOS, s.GOARCH)
	checksumName := fmt.Sprintf("inpakker_v%s_checksums.txt", versionText)
	signatureName := checksumName + ".sig"
	result := &release{Version: versionText, URL: payload.HTMLURL, ArchiveName: archiveName, ChecksumName: checksumName}
	wanted[archiveName] = &result.ArchiveURL
	wanted[checksumName] = &result.ChecksumURL
	wanted[signatureName] = &result.SignatureURL
	for _, asset := range payload.Assets {
		if destination, ok := wanted[asset.Name]; ok {
			*destination = asset.URL
		}
	}
	return result, nil
}

func validateReleaseAssets(candidate *release) error {
	required := map[string]string{
		candidate.ArchiveName:           candidate.ArchiveURL,
		candidate.ChecksumName:          candidate.ChecksumURL,
		candidate.ChecksumName + ".sig": candidate.SignatureURL,
	}
	for name, value := range required {
		if value == "" {
			return fmt.Errorf("latest release is missing required asset %q", name)
		}
	}
	return nil
}

func (s *Service) download(ctx context.Context, url string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "inpakker/"+s.CurrentVersion)
	response, err := s.client().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", response.Status)
	}
	reader := io.LimitReader(response.Body, limit+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("download exceeds %d bytes", limit)
	}
	return data, nil
}

func verifySignature(certificatePEM, checksums, signature []byte) error {
	block, _ := pem.Decode(certificatePEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return errors.New("embedded release-signing certificate is invalid")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse release-signing certificate: %w", err)
	}
	publicKey, ok := certificate.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("release-signing certificate does not contain an ECDSA key")
	}
	var parsed ecdsaSignature
	rest, err := asn1.Unmarshal(signature, &parsed)
	if err != nil || len(rest) != 0 || parsed.R == nil || parsed.S == nil {
		return errors.New("release checksum signature is malformed")
	}
	digest := sha256.Sum256(checksums)
	if !ecdsa.Verify(publicKey, digest[:], parsed.R, parsed.S) {
		return errors.New("release checksum signature is not trusted")
	}
	return nil
}

func verifyChecksum(name string, archive, checksums []byte) error {
	wanted := ""
	for line := range strings.SplitSeq(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			wanted = fields[0]
			break
		}
	}
	if wanted == "" {
		return fmt.Errorf("checksum manifest does not contain %q", name)
	}
	expected, err := hex.DecodeString(wanted)
	if err != nil || len(expected) != sha256.Size {
		return fmt.Errorf("checksum for %q is invalid", name)
	}
	actual := sha256.Sum256(archive)
	if !bytes.Equal(expected, actual[:]) {
		return errors.New("downloaded update does not match its checksum")
	}
	return nil
}

func executableFromArchive(archive []byte, goos string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open update archive: %w", err)
	}
	name := "inpakker"
	if goos == "windows" {
		name += ".exe"
	}
	for _, file := range reader.File {
		if file.Name != name || file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 > maxArchive {
			return nil, errors.New("executable in update archive is too large")
		}
		opened, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open executable in update archive: %w", err)
		}
		binary, readErr := io.ReadAll(io.LimitReader(opened, maxArchive+1))
		closeErr := opened.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read executable in update archive: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close executable in update archive: %w", closeErr)
		}
		if len(binary) == 0 || len(binary) > maxArchive {
			return nil, errors.New("executable in update archive has an invalid size")
		}
		return binary, nil
	}
	return nil, fmt.Errorf("update archive does not contain %q", name)
}

func resultFromState(current *semver.Version, cached state, fromCache bool) Result {
	result := Result{CurrentVersion: current.String(), LatestVersion: cached.LatestVersion, ReleaseURL: cached.ReleaseURL, CheckedAt: cached.CheckedAt, Cached: fromCache}
	if latest, err := semver.NewVersion(cached.LatestVersion); err == nil {
		result.Available = latest.GreaterThan(current)
	}
	return result
}

func parseCurrent(value string) (*semver.Version, error) {
	if value == "" || value == "dev" {
		return nil, ErrDevelopmentBuild
	}
	version, err := semver.NewVersion(value)
	if err != nil {
		return nil, fmt.Errorf("parse current version %q: %w", value, err)
	}
	return version, nil
}

func (s *Service) statePath() string {
	if s.StatePath != "" {
		return s.StatePath
	}
	root, err := config.Home()
	if err != nil {
		return ""
	}
	return filepath.Join(root, "update", "state.json")
}

func (s *Service) loadState() (state, bool) {
	path := s.statePath()
	if path == "" {
		return state{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state{}, false
	}
	var result state
	if json.Unmarshal(data, &result) != nil || result.CheckedAt.IsZero() {
		return state{}, false
	}
	return result, true
}

func (s *Service) saveState(value state) error {
	path := s.statePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create update cache directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode update cache: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".update-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create update cache: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect update cache: %w", err)
	}
	_, err = temporary.Write(data)
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}
	if err := replaceFile(temporaryName, path); err != nil {
		return fmt.Errorf("replace update cache: %w", err)
	}
	return nil
}

func (s *Service) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

func (s *Service) endpoint() string {
	if s.APIURL != "" {
		return s.APIURL
	}
	return APIURL
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) executable() func() (string, error) {
	if s.Executable != nil {
		return s.Executable
	}
	return os.Executable
}

func (s *Service) apply() func(io.Reader, selfupdate.Options) error {
	if s.Apply != nil {
		return s.Apply
	}
	return selfupdate.Apply
}

func emit(callback func(Event), current, total int, phase string) {
	if callback != nil {
		callback(Event{Current: current, Total: total, Phase: phase})
	}
}

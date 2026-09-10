// Package toolmanager owns user-level tool discovery and official upstream downloads.
package toolmanager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/LickABrick/inpakker/internal/atomicfile"
	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/types"
)

type Definition struct{ ID, Name, Filename, Repository, Path, LicenseURL string }

var Definitions = []Definition{
	{"content-prep", "Microsoft Win32 Content Prep Tool", "IntuneWinAppUtil.exe", "microsoft/Microsoft-Win32-Content-Prep-Tool", "IntuneWinAppUtil.exe", "https://github.com/microsoft/Microsoft-Win32-Content-Prep-Tool/blob/master/Microsoft%20License%20Terms%20For%20Win32%20Content%20Prep%20Tool.pdf"},
	{"decoder", "Package decoder", "IntuneWinAppUtilDecoder.exe", "okieselbach/Intune", "IntuneWinAppUtilDecoder/IntuneWinAppUtilDecoder/bin/Release/IntuneWinAppUtilDecoder.exe", "https://github.com/okieselbach/Intune"},
}

func DefinitionFor(id string) (Definition, error) {
	for _, d := range Definitions {
		if d.ID == id {
			return d, nil
		}
	}
	return Definition{}, fmt.Errorf("unknown tool %q; use content-prep or decoder", id)
}
func Tool(user *types.UserConfig, id string) *types.ToolConfig {
	if id == "content-prep" {
		return &user.Tools.ContentPrepTool
	}
	return &user.Tools.Decoder
}

type Status struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Config     types.ToolConfig `json:"config"`
	Valid      bool             `json:"valid"`
	Candidate  string           `json:"candidate,omitempty"`
	Origin     string           `json:"origin,omitempty"`
	LicenseURL string           `json:"licenseUrl"`
}
type Service struct {
	Client              *http.Client
	WorkingDirectory    string
	ExecutableDirectory string
	WorkspaceRoot       string
	LookPath            func(string) (string, error)
	APIBase             string // Injectable HTTP endpoint for tests; production uses api.github.com.
	RawBase             string
}

func valid(path string) bool {
	info, err := os.Stat(path)
	return path != "" && err == nil && info.Mode().IsRegular()
}
func List(user *types.UserConfig) []Status {
	result := []Status{}
	for _, d := range Definitions {
		t := *Tool(user, d.ID)
		result = append(result, Status{ID: d.ID, Name: d.Name, Config: t, Valid: valid(t.Path), LicenseURL: d.LicenseURL})
	}
	return result
}
func (s Service) Detect(ctx context.Context, user *types.UserConfig) ([]Status, error) {
	home, err := config.Home()
	if err != nil {
		return nil, err
	}
	cwd := s.WorkingDirectory
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	executableDir := s.ExecutableDirectory
	if executableDir == "" {
		exe, _ := os.Executable()
		executableDir = filepath.Dir(exe)
	}
	lookup := s.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	result := List(user)
	for i, d := range Definitions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if result[i].Valid {
			result[i].Candidate, result[i].Origin = result[i].Config.Path, "configured"
			continue
		}
		type candidate struct{ path, origin string }
		candidates := []candidate{}
		if path, err := lookup(d.Filename); err == nil {
			candidates = append(candidates, candidate{path, "PATH"})
		}
		managed := filepath.Join(home, "tools", d.ID)
		candidates = append(candidates, candidate{filepath.Join(managed, d.Filename), "managed"})
		// Only inspect the known version directory level, never recurse through user folders.
		entries, _ := os.ReadDir(managed)
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
		for _, entry := range entries {
			if entry.IsDir() {
				candidates = append(candidates, candidate{filepath.Join(managed, entry.Name(), d.Filename), "managed"})
			}
		}
		for _, location := range []candidate{{cwd, "current directory"}, {executableDir, "executable directory"}, {s.WorkspaceRoot, "workspace"}} {
			if location.path != "" {
				candidates = append(candidates, candidate{filepath.Join(location.path, d.Filename), location.origin})
			}
		}
		for _, candidate := range candidates {
			if valid(candidate.path) {
				abs, err := filepath.Abs(candidate.path)
				if err != nil {
					return nil, err
				}
				result[i].Candidate, result[i].Origin = abs, candidate.origin
				break
			}
		}
	}
	return result, nil
}

// ConfigureDetected never overwrites an explicitly configured path, including invalid paths.
func ConfigureDetected(statuses []Status) error {
	return config.UpdateUser(func(user *types.UserConfig) error {
		for _, status := range statuses {
			tool := Tool(user, status.ID)
			if tool.Path == "" && status.Candidate != "" {
				tool.Path = status.Candidate
			}
		}
		return nil
	})
}
func Set(id, path string) error {
	if _, err := DefinitionFor(id); err != nil {
		return err
	}
	if path != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		path = abs
		if !valid(path) {
			return errors.New("tool executable is unavailable or is not a file")
		}
	}
	return config.UpdateUser(func(user *types.UserConfig) error { *Tool(user, id) = types.ToolConfig{Path: path}; return nil })
}

func (s Service) Install(ctx context.Context, id string, acceptLicense bool) (types.ToolConfig, error) {
	d, err := DefinitionFor(id)
	if err != nil {
		return types.ToolConfig{}, err
	}
	if !acceptLicense {
		return types.ToolConfig{}, fmt.Errorf("accept upstream license terms before downloading %s; use --accept-license; %s", d.Name, d.LicenseURL)
	}
	api := s.APIBase
	if api == "" {
		api = "https://api.github.com"
	}
	raw := s.RawBase
	if raw == "" {
		raw = "https://raw.githubusercontent.com"
	}
	// Pin a commit before downloading so provenance cannot refer to a moving branch.
	metadata, err := s.download(ctx, api+"/repos/"+d.Repository+"/commits?path="+d.Path+"&per_page=1", 2<<20)
	if err != nil {
		return types.ToolConfig{}, err
	}
	var commits []struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(metadata, &commits); err != nil {
		return types.ToolConfig{}, err
	}
	if len(commits) == 0 || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commits[0].SHA) {
		return types.ToolConfig{}, errors.New("upstream returned no valid commit")
	}
	commit := commits[0].SHA
	url := raw + "/" + d.Repository + "/" + commit + "/" + d.Path
	data, err := s.download(ctx, url, 100<<20)
	if err != nil {
		return types.ToolConfig{}, err
	}
	if err := validateExecutable(data); err != nil {
		return types.ToolConfig{}, err
	}
	if err := ctx.Err(); err != nil {
		return types.ToolConfig{}, err
	}
	home, err := config.Home()
	if err != nil {
		return types.ToolConfig{}, err
	}
	path := filepath.Join(home, "tools", id, commit, d.Filename)
	if err := atomicfile.Write(path, data, 0700); err != nil {
		return types.ToolConfig{}, err
	}
	digest := sha256.Sum256(data)
	result := types.ToolConfig{Path: path, SourceURL: url, Version: commit, SHA256: hex.EncodeToString(digest[:])}
	if err := config.UpdateUser(func(user *types.UserConfig) error { *Tool(user, id) = result; return nil }); err != nil {
		return types.ToolConfig{}, err
	}
	return result, nil
}
func validateExecutable(data []byte) error {
	if len(data) < 64 || !bytes.Equal(data[:2], []byte("MZ")) {
		return errors.New("download is not a Windows executable")
	}
	file, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid Windows executable: %w", err)
	}
	defer file.Close()
	if file.OptionalHeader == nil || file.Characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE == 0 {
		return errors.New("download has no executable PE header")
	}
	for _, section := range file.Sections {
		if uint64(section.Offset)+uint64(section.Size) > uint64(len(data)) {
			return errors.New("truncated Windows executable")
		}
	}
	return nil
}
func (s Service) download(ctx context.Context, url string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Inpakker")
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream download failed: %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || int64(len(data)) > limit || (response.ContentLength >= 0 && response.ContentLength != int64(len(data))) {
		return nil, errors.New("empty, oversized or truncated upstream response")
	}
	return data, nil
}

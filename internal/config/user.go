package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/LickABrick/inpakker/internal/atomicfile"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
)

var userMu sync.Mutex

func Home() (string, error) {
	if value := os.Getenv("INPAKKER_HOME"); value != "" {
		return filepath.Abs(value)
	}
	if runtime.GOOS == "windows" {
		if value := os.Getenv("LOCALAPPDATA"); value != "" {
			return filepath.Join(value, "Inpakker"), nil
		}
		return "", errors.New("LOCALAPPDATA is unavailable; set INPAKKER_HOME")
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "Inpakker"), nil
}

func UserPath() (string, error) {
	root, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "config.json"), nil
}

func DefaultUser() types.UserConfig {
	return types.UserConfig{SchemaVersion: 1, WorkspaceDefaults: types.WorkspaceDefaults{
		ApplicationsDirectory: "apps", SourceDirectory: "source", OutputDirectory: "output",
	}, Workspaces: []types.WorkspaceRegistration{}}
}

// LoadUser is read-only; EnsureUser is the explicit first-run initializer.
func LoadUser() (*types.UserConfig, error) {
	path, err := UserPath()
	if err != nil {
		return nil, err
	}
	cfg := DefaultUser()
	cfg.SchemaVersion = 0
	if err := readJSON(path, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = DefaultUser()
			return &cfg, nil
		}
		return nil, err
	}
	if err := ValidateUser(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func EnsureUser() (*types.UserConfig, error) {
	userMu.Lock()
	defer userMu.Unlock()
	cfg, err := LoadUser()
	if err != nil {
		return nil, err
	}
	path, err := UserPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err = SaveUser(cfg)
		if err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// UpdateUser serializes in-process read/modify/write operations, including background detection.
func UpdateUser(update func(*types.UserConfig) error) error {
	userMu.Lock()
	defer userMu.Unlock()
	cfg, err := LoadUser()
	if err != nil {
		return err
	}
	if err := update(cfg); err != nil {
		return err
	}
	return SaveUser(cfg)
}

func SaveUser(cfg *types.UserConfig) error {
	if err := ValidateUser(cfg); err != nil {
		return err
	}
	path, err := UserPath()
	if err != nil {
		return err
	}
	return atomicfile.JSON(path, cfg, 0600)
}

func ValidateUser(cfg *types.UserConfig) error {
	if cfg == nil {
		return errors.New("user configuration is nil")
	}
	if err := schema(cfg.SchemaVersion); err != nil {
		return err
	}
	if err := validateDirectories(cfg.WorkspaceDefaults); err != nil {
		return err
	}
	for _, tool := range []types.ToolConfig{cfg.Tools.ContentPrepTool, cfg.Tools.Decoder} {
		if tool.Path != "" && !filepath.IsAbs(tool.Path) {
			return errors.New("tool path must be absolute")
		}
	}
	ids, paths := map[string]bool{}, map[string]bool{}
	for _, reg := range cfg.Workspaces {
		if !ValidUUID(reg.ID) {
			return errors.New("workspace registration has an invalid UUID")
		}
		if !filepath.IsAbs(reg.Path) {
			return errors.New("registered workspace path must be absolute")
		}
		path := strings.ToLower(filepath.Clean(reg.Path))
		if ids[reg.ID] || paths[path] {
			return errors.New("duplicate workspace registration")
		}
		ids[reg.ID], paths[path] = true, true
	}
	if cfg.ActiveWorkspaceID != "" && !ids[cfg.ActiveWorkspaceID] {
		return errors.New("active workspace is not registered")
	}
	return nil
}

func NewUUID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[0:4], id[4:6], id[6:8], id[8:10], id[10:16])
}

func ValidUUID(id string) bool {
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' || strings.ToLower(id) != id {
		return false
	}
	decoded, err := hex.DecodeString(strings.ReplaceAll(id, "-", ""))
	return err == nil && len(decoded) == 16 && id != "00000000-0000-0000-0000-000000000000"
}

func schema(version int) error {
	if version != types.SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d; expected %d", version, types.SchemaVersion)
	}
	return nil
}

func readJSON(path string, value any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("read %q: expected one JSON object", path)
	}
	return nil
}

func validateDirectories(d types.WorkspaceDefaults) error {
	for label, value := range map[string]string{"applicationsDirectory": d.ApplicationsDirectory, "sourceDirectory": d.SourceDirectory, "outputDirectory": d.OutputDirectory} {
		if !pathutil.IsSafeRelative(value) || !pathutil.ValidRelative(value) {
			return fmt.Errorf("%s must be a safe relative directory", label)
		}
	}
	return nil
}

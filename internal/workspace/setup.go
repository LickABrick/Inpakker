package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/types"
)

type SetupOptions struct {
	Root             string
	AppsDir          string
	OutputDir        string
	IntuneWinAppUtil string
	DecoderPath      string
	MuteUtility      bool
	CreateExample    bool
}

type SetupResult struct {
	Workspace *Workspace
	Example   *AppRef
}

func Initialize(options SetupOptions) (SetupResult, error) {
	if options.Root == "" {
		options.Root = "."
	}
	if options.AppsDir == "" {
		options.AppsDir = "apps"
	}
	if options.OutputDir == "" {
		options.OutputDir = "output"
	}
	for name, path := range map[string]string{"IntuneWinAppUtil.exe": options.IntuneWinAppUtil, "decoder": options.DecoderPath} {
		if path != "" && !filepath.IsAbs(path) {
			return SetupResult{}, fmt.Errorf("%s path must be absolute", name)
		}
	}
	absRoot, err := filepath.Abs(options.Root)
	if err != nil {
		return SetupResult{}, fmt.Errorf("resolve workspace directory: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return SetupResult{}, fmt.Errorf("create workspace directory: %w", err)
	}
	configPath := filepath.Join(absRoot, ConfigFile)
	if _, err := os.Stat(configPath); err == nil {
		return SetupResult{}, fmt.Errorf("workspace is already initialized at %q", configPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return SetupResult{}, fmt.Errorf("inspect workspace config: %w", err)
	}
	cfg := types.GlobalConfig{
		IntuneWinAppUtil:     options.IntuneWinAppUtil,
		DecoderPath:          options.DecoderPath,
		DefaultOutputDir:     options.OutputDir,
		AppsDir:              options.AppsDir,
		MuteIntuneWinAppUtil: options.MuteUtility,
	}
	if err := config.ValidateGlobal(&cfg); err != nil {
		return SetupResult{}, fmt.Errorf("validate workspace settings: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(absRoot, options.AppsDir), 0o755); err != nil {
		return SetupResult{}, fmt.Errorf("create applications directory: %w", err)
	}
	file, err := os.OpenFile(configPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return SetupResult{}, fmt.Errorf("create workspace config: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	writeErr := encoder.Encode(cfg)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(configPath)
		if writeErr != nil {
			return SetupResult{}, fmt.Errorf("write workspace config: %w", writeErr)
		}
		return SetupResult{}, fmt.Errorf("close workspace config: %w", closeErr)
	}
	ws := &Workspace{Root: absRoot, Config: cfg}
	result := SetupResult{Workspace: ws}
	if options.CreateExample {
		ref, err := ws.Create(CreateOptions{
			Name:        "example",
			DisplayName: "Inpakker Example",
			Source:      "source",
			SetupFile:   "install.ps1",
			OutputDir:   options.OutputDir,
		})
		if err != nil {
			return result, fmt.Errorf("create example application: %w", err)
		}
		script := []byte("Write-Host \"Hello from the Inpakker example package.\"\r\n")
		if err := os.WriteFile(filepath.Join(ref.Path, "source", "install.ps1"), script, 0o644); err != nil {
			return result, fmt.Errorf("write example setup script: %w", err)
		}
		result.Example = &ref
	}
	return result, nil
}

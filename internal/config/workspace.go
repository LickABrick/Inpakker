package config

import (
	"errors"
	"github.com/LickABrick/inpakker/internal/atomicfile"
	"github.com/LickABrick/inpakker/types"
	"strings"
)

func LoadWorkspace(path string) (*types.WorkspaceConfig, error) {
	var cfg types.WorkspaceConfig
	if err := readJSON(path, &cfg); err != nil {
		return nil, err
	}
	if err := ValidateWorkspace(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
func ValidateWorkspace(cfg *types.WorkspaceConfig) error {
	if cfg == nil {
		return errors.New("workspace configuration is nil")
	}
	if err := schema(cfg.SchemaVersion); err != nil {
		return err
	}
	if !ValidUUID(cfg.ID) {
		return errors.New("workspace id must be a UUID")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("workspace name is required")
	}
	return validateDirectories(types.WorkspaceDefaults{ApplicationsDirectory: cfg.ApplicationsDirectory, SourceDirectory: cfg.SourceDirectory, OutputDirectory: cfg.OutputDirectory})
}
func SaveWorkspace(path string, cfg *types.WorkspaceConfig) error {
	if err := ValidateWorkspace(cfg); err != nil {
		return err
	}
	return atomicfile.JSON(path, cfg, 0644)
}

package config

import (
	"errors"
	"fmt"
	"github.com/LickABrick/inpakker/internal/atomicfile"
	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
	"strings"
)

func LoadAppConfig(path string) (*types.AppConfig, error) {
	var cfg types.AppConfig
	if err := readJSON(path, &cfg); err != nil {
		return nil, err
	}
	if err := schema(cfg.SchemaVersion); err != nil {
		return nil, err
	}
	return &cfg, nil
}
func SaveApp(path string, cfg *types.AppConfig) error {
	if err := ValidateApp(cfg); err != nil {
		return err
	}
	return atomicfile.JSON(path, cfg, 0644)
}
func ValidateApp(cfg *types.AppConfig) error {
	if cfg == nil {
		return errors.New("application configuration is nil")
	}
	if err := schema(cfg.SchemaVersion); err != nil {
		return err
	}
	var messages []string
	if !ValidUUID(cfg.ID) {
		messages = append(messages, "application id must be a UUID")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		messages = append(messages, "name is required")
	}
	if strings.TrimSpace(cfg.SetupFile) == "" {
		messages = append(messages, "setupFile is required")
	}
	for label, value := range map[string]string{"sourceDirectory": cfg.SourceDirectory, "outputDirectory": cfg.OutputDirectory, "setupFile": cfg.SetupFile} {
		if value != "" && (!pathutil.IsSafeRelative(value) || !pathutil.ValidRelative(value)) {
			messages = append(messages, fmt.Sprintf("%s must be a relative path within the application directory", label))
		}
	}
	if len(messages) > 0 {
		return errors.New(strings.Join(messages, "; "))
	}
	return nil
}

func EffectiveApp(ws types.WorkspaceConfig, app types.AppConfig) types.EffectiveAppConfig {
	result := types.EffectiveAppConfig{AppConfig: app, SourceInherited: app.SourceDirectory == "", OutputInherited: app.OutputDirectory == ""}
	if result.SourceInherited {
		result.SourceDirectory = ws.SourceDirectory
	}
	if result.OutputInherited {
		result.OutputDirectory = ws.OutputDirectory
	}
	return result
}

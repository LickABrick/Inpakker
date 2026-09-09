package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
)

func LoadAppConfig(path string) (*types.AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg types.AppConfig
	err = json.Unmarshal(data, &cfg)
	return &cfg, err
}

func ValidateApp(cfg *types.AppConfig) error {
	if cfg == nil {
		return errors.New("app config is nil")
	}
	var messages []string
	required := []struct {
		name  string
		value string
	}{
		{"name", cfg.Name},
		{"displayName", cfg.DisplayName},
		{"source", cfg.Source},
		{"setupFile", cfg.SetupFile},
	}

	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			messages = append(messages, fmt.Sprintf("%s is required", field.name))
		}
	}

	paths := []struct {
		name  string
		value string
	}{
		{"source", cfg.Source},
		{"setupFile", cfg.SetupFile},
		{"outputDir", cfg.OutputDir},
	}
	for _, path := range paths {
		name, value := path.name, path.value
		if value != "" && !pathutil.IsSafeRelative(value) {
			messages = append(messages, fmt.Sprintf("%s must be a relative path within the app directory", name))
		}
	}

	if len(messages) > 0 {
		return errors.New(strings.Join(messages, "; "))
	}
	return nil
}

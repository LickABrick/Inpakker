package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/LickABrick/inpakker/internal/pathutil"
	"github.com/LickABrick/inpakker/types"
)

func LoadGlobalConfig(path string) (*types.GlobalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg types.GlobalConfig
	err = json.Unmarshal(data, &cfg)
	return &cfg, err
}

func ValidateGlobal(cfg *types.GlobalConfig) error {
	if cfg == nil {
		return errors.New("global config is nil")
	}
	var messages []string
	if cfg.AppsDir != "" && !pathutil.IsSafeRelative(cfg.AppsDir) {
		messages = append(messages, "appsDir must be a relative path within the workspace")
	}
	if cfg.DefaultOutputDir != "" && !pathutil.IsSafeRelative(cfg.DefaultOutputDir) {
		messages = append(messages, "defaultOutputDir must be a relative path within each app directory")
	}
	if len(messages) > 0 {
		return errors.New(strings.Join(messages, "; "))
	}
	return nil
}

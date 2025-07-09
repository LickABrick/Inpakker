package config

import (
	"encoding/json"
	"github.com/LickABrick/inpakker/types"
	"os"
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

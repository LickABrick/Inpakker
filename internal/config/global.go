package config

import (
	"encoding/json"
	"github.com/LickABrick/inpakker/types"
	"os"
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

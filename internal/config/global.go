package config

import (
	"github.com/yosuke-furukawa/json5/encoding/json5"
	"os"
	"github.com/LickABrick/inpakker/types"
)

func LoadGlobalConfig(path string) (*types.GlobalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg types.GlobalConfig
	err = json5.Unmarshal(data, &cfg)
	return &cfg, err
}

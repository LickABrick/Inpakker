package config

import (
	"os"

	"github.com/yosuke-furukawa/json5/encoding/json5"
	"github.com/LickABrick/inpakker/types"
)

func LoadAppConfig(path string) (*types.AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg types.AppConfig
	err = json5.Unmarshal(data, &cfg)
	return &cfg, err
}

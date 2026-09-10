package config

import (
	"fmt"
	"github.com/LickABrick/inpakker/types"
	"strconv"
)

var UserKeys = []string{"preferences.showToolOutput", "workspaceDefaults.applicationsDirectory", "workspaceDefaults.sourceDirectory", "workspaceDefaults.outputDirectory"}
var WorkspaceKeys = []string{"name", "applicationsDirectory", "sourceDirectory", "outputDirectory"}

func SetUserValue(cfg *types.UserConfig, key, value string) error {
	switch key {
	case "preferences.showToolOutput":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("showToolOutput expects true or false")
		}
		cfg.Preferences.ShowToolOutput = parsed
	case "workspaceDefaults.applicationsDirectory":
		cfg.WorkspaceDefaults.ApplicationsDirectory = value
	case "workspaceDefaults.sourceDirectory":
		cfg.WorkspaceDefaults.SourceDirectory = value
	case "workspaceDefaults.outputDirectory":
		cfg.WorkspaceDefaults.OutputDirectory = value
	default:
		return fmt.Errorf("unknown global setting %q", key)
	}
	return ValidateUser(cfg)
}
func SetWorkspaceValue(cfg *types.WorkspaceConfig, key, value string) error {
	switch key {
	case "name":
		cfg.Name = value
	case "applicationsDirectory":
		cfg.ApplicationsDirectory = value
	case "sourceDirectory":
		cfg.SourceDirectory = value
	case "outputDirectory":
		cfg.OutputDirectory = value
	default:
		return fmt.Errorf("unknown workspace setting %q", key)
	}
	return ValidateWorkspace(cfg)
}

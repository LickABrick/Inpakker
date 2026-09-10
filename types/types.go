package types

import "time"

const SchemaVersion = 1

type ToolConfig struct {
	Path      string `json:"path"`
	SourceURL string `json:"sourceUrl,omitempty"`
	Version   string `json:"version,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
}

type Tools struct {
	ContentPrepTool ToolConfig `json:"contentPrepTool"`
	Decoder         ToolConfig `json:"decoder"`
}

type Preferences struct {
	ShowToolOutput bool `json:"showToolOutput"`
}

type WorkspaceDefaults struct {
	ApplicationsDirectory string `json:"applicationsDirectory"`
	SourceDirectory       string `json:"sourceDirectory"`
	OutputDirectory       string `json:"outputDirectory"`
}

type WorkspaceRegistration struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	LastOpenedAt time.Time `json:"lastOpenedAt"`
}

type UserConfig struct {
	SchemaVersion     int                     `json:"schemaVersion"`
	Tools             Tools                   `json:"tools"`
	Preferences       Preferences             `json:"preferences"`
	WorkspaceDefaults WorkspaceDefaults       `json:"workspaceDefaults"`
	Workspaces        []WorkspaceRegistration `json:"workspaces"`
	ActiveWorkspaceID string                  `json:"activeWorkspaceId"`
}

type WorkspaceConfig struct {
	SchemaVersion         int    `json:"schemaVersion"`
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	ApplicationsDirectory string `json:"applicationsDirectory"`
	SourceDirectory       string `json:"sourceDirectory"`
	OutputDirectory       string `json:"outputDirectory"`
}

type AppConfig struct {
	SchemaVersion    int    `json:"schemaVersion"`
	ID               string `json:"id"`
	Name             string `json:"name"`
	SetupFile        string `json:"setupFile"`
	SourceDirectory  string `json:"sourceDirectory,omitempty"`
	OutputDirectory  string `json:"outputDirectory,omitempty"`
	InstallCommand   string `json:"installCommand,omitempty"`
	UninstallCommand string `json:"uninstallCommand,omitempty"`
}

// EffectiveAppConfig is resolved once from portable workspace and application settings.
type EffectiveAppConfig struct {
	AppConfig
	SourceInherited bool `json:"sourceInherited"`
	OutputInherited bool `json:"outputInherited"`
}

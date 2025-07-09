package types

type GlobalConfig struct {
	IntuneWinAppUtil string `json:"intunewinapputil"`
	IntuneWinAppUtilPath string `json:"intuneWinAppUtilPath"`
	DefaultOutputDir     string `json:"defaultOutputDir"`
	AppsDir              string `json:"appsDir"`
	MuteIntuneWinAppUtil bool   `json:"muteIntuneWinAppUtil"`
}

type AppConfig struct {
	Name             string `json:"name"`
	DisplayName      string `json:"displayName"`
	Source           string `json:"source"`
	SetupFile        string `json:"setupFile"`
	InstallCommand   string `json:"installCommand"`
	UninstallCommand string `json:"uninstallCommand"`
	OutputDir        string `json:"outputDir,omitempty"`
}

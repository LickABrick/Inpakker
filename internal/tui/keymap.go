package tui

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Applications, Palette, CheckUpdates, InstallUpdate, AddWorkspace, RelinkWorkspace, RemoveWorkspace, RenameWorkspace, DetectTools, DownloadTool, ClearTool key.Binding
	Workspaces, Settings, About, Folder                                                                                                                       key.Binding
	Up, Down, PageUp, PageDown, Open, Back, Escape, Search, NewApp, Build, BuildAll, BuildOptions                                                             key.Binding
	Validate, ValidateAll, Unpack, UnpackAll, Diagnostics, Refresh                                                                                            key.Binding
	Help, Quit, Cancel, Logs, LastResult                                                                                                                      key.Binding
	Actions                                                                                                                                                   key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Applications: binding([]string{"g"}, "g", "applications"), Palette: binding([]string{":"}, ":", "commands"),
		CheckUpdates: binding([]string{"c"}, "c", "check updates"), InstallUpdate: binding([]string{"u"}, "u", "install update"),
		AddWorkspace: binding([]string{"A"}, "A", "add workspace"), RelinkWorkspace: binding([]string{"r"}, "r", "relink"), RemoveWorkspace: binding([]string{"x"}, "x", "remove registration"), RenameWorkspace: binding([]string{"e"}, "e", "rename"),
		DetectTools: binding([]string{"d"}, "d", "detect tools"), DownloadTool: binding([]string{"I"}, "I", "download"), ClearTool: binding([]string{"x"}, "x", "clear configuration"),
		Actions:      binding([]string{"a"}, "a", "actions"),
		Workspaces:   binding([]string{"w"}, "w", "workspaces"),
		Settings:     binding([]string{"s"}, "s", "settings"),
		About:        binding([]string{"i"}, "i", "about"),
		Folder:       binding([]string{"o"}, "o", "folder"),
		Up:           binding([]string{"up", "k"}, "↑", "up"),
		Down:         binding([]string{"down", "j"}, "↓", "down"),
		PageUp:       binding([]string{"pgup"}, "pgup", "page up"),
		PageDown:     binding([]string{"pgdown"}, "pgdn", "page down"),
		Open:         binding([]string{"enter"}, "enter", "open"),
		Back:         binding([]string{"backspace"}, "backspace", "back"),
		Escape:       binding([]string{"esc"}, "esc", "back"),
		Search:       binding([]string{"/"}, "/", "filter"),
		NewApp:       binding([]string{"n"}, "n", "new"),
		Build:        binding([]string{"b"}, "b", "build"),
		BuildAll:     binding([]string{"ctrl+b"}, "ctrl+b", "build all"),
		BuildOptions: binding([]string{"B"}, "B", "build options"),
		Validate:     binding([]string{"v"}, "v", "validate"),
		ValidateAll:  binding([]string{"ctrl+v"}, "ctrl+v", "validate all"),
		Unpack:       binding([]string{"u"}, "u", "unpack"),
		UnpackAll:    binding([]string{"ctrl+u"}, "ctrl+u", "unpack all"),
		Diagnostics:  binding([]string{"d"}, "d", "workspace diagnostics"),
		Refresh:      binding([]string{"r"}, "r", "refresh"),
		Help:         binding([]string{"?"}, "?", "help"),
		Quit:         binding([]string{"q", "ctrl+c"}, "q", "quit"),
		Cancel:       binding([]string{"ctrl+c"}, "ctrl+c", "cancel"),
		Logs:         binding([]string{"l"}, "l", "view log"),
		LastResult:   binding([]string{"L"}, "L", "last result"),
	}
}

func binding(keys []string, helpKey, description string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(helpKey, description))
}

package tui

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Workspaces, Settings, About, Folder                                                           key.Binding
	Up, Down, PageUp, PageDown, Open, Back, Escape, Search, NewApp, Build, BuildAll, BuildOptions key.Binding
	Validate, ValidateAll, Unpack, UnpackAll, Diagnostics, Refresh                                key.Binding
	Help, Quit, Cancel, Logs                                                                      key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Workspaces:   binding([]string{"w"}, "w", "workspaces"),
		Settings:     binding([]string{"s"}, "s", "settings"),
		About:        binding([]string{"i"}, "i", "about"),
		Folder:       binding([]string{"o"}, "o", "folder"),
		Up:           binding([]string{"up", "k"}, "↑/k", "up"),
		Down:         binding([]string{"down", "j"}, "↓/j", "down"),
		PageUp:       binding([]string{"pgup"}, "pgup", "page up"),
		PageDown:     binding([]string{"pgdown"}, "pgdn", "page down"),
		Open:         binding([]string{"enter"}, "enter", "open"),
		Back:         binding([]string{"backspace"}, "backspace", "back"),
		Escape:       binding([]string{"esc"}, "esc", "back"),
		Search:       binding([]string{"/"}, "/", "search"),
		NewApp:       binding([]string{"n"}, "n", "new"),
		Build:        binding([]string{"b"}, "b", "build"),
		BuildAll:     binding([]string{"ctrl+b"}, "ctrl+b", "build all"),
		BuildOptions: binding([]string{"B"}, "B", "build options"),
		Validate:     binding([]string{"v"}, "v", "validate"),
		ValidateAll:  binding([]string{"ctrl+v"}, "ctrl+v", "validate all"),
		Unpack:       binding([]string{"u"}, "u", "unpack"),
		UnpackAll:    binding([]string{"ctrl+u"}, "ctrl+u", "unpack all"),
		Diagnostics:  binding([]string{"d"}, "d", "diagnostics"),
		Refresh:      binding([]string{"r"}, "r", "refresh"),
		Help:         binding([]string{"?"}, "?", "help"),
		Quit:         binding([]string{"q", "ctrl+c"}, "q", "quit"),
		Cancel:       binding([]string{"ctrl+c"}, "ctrl+c", "cancel"),
		Logs:         binding([]string{"l"}, "l", "view log"),
	}
}

func binding(keys []string, helpKey, description string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(helpKey, description))
}

type helpBindings struct {
	short []key.Binding
	full  [][]key.Binding
}

func (h helpBindings) ShortHelp() []key.Binding  { return h.short }
func (h helpBindings) FullHelp() [][]key.Binding { return h.full }

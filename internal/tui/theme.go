package tui

import (
	"image/color"

	"charm.land/bubbles/v2/help"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// Theme contains semantic colors and reusable component styles. Page code
// should choose a semantic style instead of introducing color literals.
type Theme struct {
	BrandColor          color.Color
	AccentColor         color.Color
	TextColor           color.Color
	MutedColor          color.Color
	SurfaceColor        color.Color
	SelectedColor       color.Color
	BorderColor         color.Color
	FocusedBorderColor  color.Color
	SuccessColor        color.Color
	WarningColor        color.Color
	DangerColor         color.Color
	DisabledColor       color.Color
	TopBar              lipgloss.Style
	Brand               lipgloss.Style
	Version             lipgloss.Style
	Update              lipgloss.Style
	WorkspaceBar        lipgloss.Style
	PageTitle           lipgloss.Style
	Breadcrumb          lipgloss.Style
	Text                lipgloss.Style
	TextMuted           lipgloss.Style
	Panel               lipgloss.Style
	PanelTitle          lipgloss.Style
	Modal               lipgloss.Style
	ModalTitle          lipgloss.Style
	TableHeader         lipgloss.Style
	TableCell           lipgloss.Style
	TableSelected       lipgloss.Style
	Help                lipgloss.Style
	StatusSuccess       lipgloss.Style
	StatusWarning       lipgloss.Style
	StatusError         lipgloss.Style
	StatusMuted         lipgloss.Style
	HorizontalSeparator lipgloss.Style
}

func NewTheme(isDark bool) Theme {
	choose := lipgloss.LightDark(isDark)
	brand := choose(lipgloss.Color("#9A5B13"), lipgloss.Color("#E3A44A"))
	accent := choose(lipgloss.Color("#087F8C"), lipgloss.Color("#4FC3C8"))
	text := choose(lipgloss.Color("#172033"), lipgloss.Color("#E8EDF4"))
	muted := choose(lipgloss.Color("#667085"), lipgloss.Color("#8D99AA"))
	surface := choose(lipgloss.Color("#E8EDF3"), lipgloss.Color("#172033"))
	selected := choose(lipgloss.Color("#DCE8EE"), lipgloss.Color("#26364A"))
	border := choose(lipgloss.Color("#AAB4C3"), lipgloss.Color("#415168"))
	success := choose(lipgloss.Color("#18794E"), lipgloss.Color("#5FCB9A"))
	warning := choose(lipgloss.Color("#9A6700"), lipgloss.Color("#F2C14E"))
	danger := choose(lipgloss.Color("#B42318"), lipgloss.Color("#FF7B72"))
	disabled := choose(lipgloss.Color("#98A2B3"), lipgloss.Color("#697586"))
	return Theme{
		BrandColor: brand, AccentColor: accent, TextColor: text, MutedColor: muted,
		SurfaceColor: surface, SelectedColor: selected, BorderColor: border,
		FocusedBorderColor: brand, SuccessColor: success, WarningColor: warning,
		DangerColor: danger, DisabledColor: disabled,
		TopBar:              lipgloss.NewStyle().Background(surface).Foreground(text).Padding(0, 1),
		Brand:               lipgloss.NewStyle().Foreground(brand).Bold(true),
		Version:             lipgloss.NewStyle().Foreground(muted),
		Update:              lipgloss.NewStyle().Foreground(warning).Bold(true),
		WorkspaceBar:        lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		PageTitle:           lipgloss.NewStyle().Foreground(text).Bold(true),
		Breadcrumb:          lipgloss.NewStyle().Foreground(muted),
		Text:                lipgloss.NewStyle().Foreground(text),
		TextMuted:           lipgloss.NewStyle().Foreground(muted),
		Panel:               lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1),
		PanelTitle:          lipgloss.NewStyle().Foreground(brand).Bold(true),
		Modal:               lipgloss.NewStyle().Background(surface).Foreground(text).Border(lipgloss.RoundedBorder()).BorderForeground(brand).Padding(1, 2),
		ModalTitle:          lipgloss.NewStyle().Foreground(brand).Bold(true),
		TableHeader:         lipgloss.NewStyle().Foreground(muted).Bold(true).Padding(0, 1).BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(border),
		TableCell:           lipgloss.NewStyle().Foreground(text).Padding(0, 1),
		TableSelected:       lipgloss.NewStyle().Foreground(text).Background(selected).Bold(true).Padding(0, 1).BorderLeft(true).BorderStyle(lipgloss.Border{Left: "›"}).BorderForeground(brand),
		Help:                lipgloss.NewStyle().Foreground(muted),
		StatusSuccess:       lipgloss.NewStyle().Foreground(success),
		StatusWarning:       lipgloss.NewStyle().Foreground(warning),
		StatusError:         lipgloss.NewStyle().Foreground(danger),
		StatusMuted:         lipgloss.NewStyle().Foreground(disabled),
		HorizontalSeparator: lipgloss.NewStyle().Foreground(border),
	}
}

func (t Theme) HelpStyles() help.Styles {
	styles := help.DefaultDarkStyles()
	styles.ShortKey = lipgloss.NewStyle().Foreground(t.BrandColor).Bold(true)
	styles.ShortDesc = lipgloss.NewStyle().Foreground(t.MutedColor)
	styles.ShortSeparator = lipgloss.NewStyle().Foreground(t.BorderColor)
	styles.FullKey = styles.ShortKey
	styles.FullDesc = styles.ShortDesc
	styles.FullSeparator = styles.ShortSeparator
	styles.Ellipsis = styles.ShortSeparator
	return styles
}

func (t Theme) HuhTheme() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		styles := huh.ThemeBase(isDark)
		styles.Focused.Base = styles.Focused.Base.BorderForeground(t.FocusedBorderColor)
		styles.Focused.Title = styles.Focused.Title.Foreground(t.BrandColor).Bold(true)
		styles.Focused.NoteTitle = styles.Focused.NoteTitle.Foreground(t.BrandColor).Bold(true)
		styles.Focused.Description = styles.Focused.Description.Foreground(t.MutedColor)
		styles.Focused.ErrorIndicator = styles.Focused.ErrorIndicator.Foreground(t.DangerColor)
		styles.Focused.ErrorMessage = styles.Focused.ErrorMessage.Foreground(t.DangerColor)
		styles.Focused.SelectSelector = styles.Focused.SelectSelector.Foreground(t.BrandColor).SetString("› ")
		styles.Focused.SelectedOption = styles.Focused.SelectedOption.Foreground(t.AccentColor).Bold(true)
		styles.Focused.Option = styles.Focused.Option.Foreground(t.TextColor)
		styles.Focused.FocusedButton = styles.Focused.FocusedButton.Background(t.BrandColor).Foreground(lipgloss.Color("#101828")).Bold(true)
		styles.Focused.BlurredButton = styles.Focused.BlurredButton.Background(t.SelectedColor).Foreground(t.TextColor)
		styles.Focused.TextInput.Cursor = styles.Focused.TextInput.Cursor.Foreground(t.BrandColor)
		styles.Focused.TextInput.Prompt = styles.Focused.TextInput.Prompt.Foreground(t.BrandColor)
		styles.Focused.TextInput.Text = styles.Focused.TextInput.Text.Foreground(t.TextColor)
		styles.Focused.TextInput.Placeholder = styles.Focused.TextInput.Placeholder.Foreground(t.DisabledColor)
		styles.Blurred = styles.Focused
		styles.Blurred.Base = styles.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
		styles.Blurred.Title = styles.Blurred.Title.Foreground(t.MutedColor)
		styles.Group.Title = lipgloss.NewStyle().Foreground(t.BrandColor).Bold(true)
		styles.Group.Description = lipgloss.NewStyle().Foreground(t.MutedColor)
		styles.Help = t.HelpStyles()
		return styles
	})
}

package styles

import "github.com/charmbracelet/lipgloss"

var (
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SecondaryColor = lipgloss.Color("#04B575")
	WarningColor   = lipgloss.Color("#FF5F56")
	MutedColor     = lipgloss.Color("#626262")
	BgSelected     = lipgloss.Color("#353535")

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(PrimaryColor).
			Padding(0, 1).
			MarginBottom(1)

	SubHeaderStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	CurrentBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	MarkedBadge = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	NormalRow = lipgloss.NewStyle().
			Padding(0, 1)

	SelectedRow = lipgloss.NewStyle().
			Padding(0, 1).
			Background(BgSelected)

	ButtonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(MutedColor).
			Padding(0, 2).
			MarginRight(1)

	ActiveButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(PrimaryColor).
				Bold(true).
				Padding(0, 2).
				MarginRight(1)

	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(WarningColor).
			Padding(1, 2)
)
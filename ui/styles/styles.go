package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Midnight Commander / HTOP classic palette
	CyanBg      = lipgloss.Color("#008080")
	BlueBg      = lipgloss.Color("#0000AA")
	HeaderBg    = lipgloss.Color("#005F87")
	FooterBg    = lipgloss.Color("#005F87")
	PanelBorder = lipgloss.Color("#00AAAA")

	FgWhite  = lipgloss.Color("#FFFFFF")
	FgYellow = lipgloss.Color("#FFFF55")
	FgGreen  = lipgloss.Color("#55FF55")
	FgRed    = lipgloss.Color("#FF5555")
	FgBlack  = lipgloss.Color("#000000")

	// Header Styles
	HeaderTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(FgYellow).
			Background(HeaderBg)

	HeaderInfo = lipgloss.NewStyle().
			Foreground(FgWhite).
			Background(HeaderBg)

	// Double-border panel frame
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(PanelBorder).
			Background(BlueBg)

	TableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(FgYellow).
			Background(CyanBg)

	// Table Rows
	NormalRow = lipgloss.NewStyle().
			Foreground(FgWhite).
			Background(BlueBg)

	SelectedRow = lipgloss.NewStyle().
			Bold(true).
			Foreground(FgBlack).
			Background(CyanBg)

	CurrentBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(FgGreen)

	MarkedBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(FgRed)

	// Bottom Action Bar
	ButtonNormal = lipgloss.NewStyle().
			Foreground(FgBlack).
			Background(CyanBg).
			Padding(0, 1)

	ButtonActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(FgBlack).
			Background(FgYellow).
			Padding(0, 1)

	FooterBarStyle = lipgloss.NewStyle().
			Background(FooterBg).
			Foreground(FgWhite)

	// Modal Box
	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(FgRed).
			Background(BlueBg).
			Padding(1, 2)
)
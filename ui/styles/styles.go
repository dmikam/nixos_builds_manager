package styles

import "github.com/charmbracelet/lipgloss"

var (
	HeaderTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981")).
			Background(lipgloss.Color("#000000"))

	HeaderInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	TableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F3F4F6")).
			Background(lipgloss.Color("#111827"))

	NormalRow = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB"))

	SelectedRow = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F9FAFB")).
			Background(lipgloss.Color("#1F2937"))

	CurrentBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981"))

	MarkedBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#EF4444"))

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Background(lipgloss.Color("#000000"))

	FooterBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#000000"))

	ButtonNormal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1)

	ButtonActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#10B981")).
			Padding(0, 1)

	ButtonDisabled = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4B5563")).
			Background(lipgloss.Color("#111827")).
			Padding(0, 1)

	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#10B981")).
			Background(lipgloss.Color("#000000")).
			Padding(1, 2)
)
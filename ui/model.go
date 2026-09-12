package ui

import (
	"nixos_builds_manager/nix"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type FocusArea int

const (
	FocusList FocusArea = iota
	FocusFooter
)

type CommandType int

const (
	CmdNone CommandType = iota
	CmdPurge
	CmdOptimize
)

type Model struct {
	Generations []nix.Generation
	Cursor      int
	Focus       FocusArea
	ActiveButton int

	// Dialog & Modal state
	ConfirmModal bool
	IsLoading    bool
	LoadingMsg   string
	ActiveCmd    CommandType

	// Log Output
	Viewport viewport.Model
	ShowLog  bool
	LogData  string

	Spinner spinner.Model
	Err     error
}

type GenerationsLoadedMsg []nix.Generation
type OperationFinishedMsg struct {
	Output string
	Err    error
}

func InitialModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	vp := viewport.New(80, 10)

	return Model{
		Generations:  []nix.Generation{},
		Cursor:       0,
		Focus:        FocusList,
		ActiveButton: 0,
		ConfirmModal: false,
		IsLoading:    true,
		LoadingMsg:   "Loading generations...",
		Spinner:      s,
		Viewport:     vp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
		fetchGenerationsCmd(),
	)
}

func fetchGenerationsCmd() tea.Cmd {
	return func() tea.Msg {
		gens, err := nix.ListGenerations()
		if err != nil {
			return OperationFinishedMsg{Output: "", Err: err}
		}
		return GenerationsLoadedMsg(gens)
	}
}
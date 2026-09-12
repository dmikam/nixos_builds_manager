package ui

import (
	"nixos_builds_manager/nix"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
	CmdBuild
)

type Model struct {
	Width        int
	Height       int
	Generations  []nix.Generation
	Cursor       int
	Focus        FocusArea
	ActiveButton int

	// Dialog & Modal state
	ConfirmModal bool
	BuildModal   bool
	LabelInput   textinput.Model

	IsLoading  bool
	LoadingMsg string

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
	vp := viewport.New(80, 20)

	ti := textinput.New()
	ti.Placeholder = "Optional build label/profile name..."
	ti.CharLimit = 64
	ti.Width = 40

	return Model{
		Generations:  []nix.Generation{},
		Cursor:       0,
		Focus:        FocusList,
		ActiveButton: 0,
		ConfirmModal: false,
		BuildModal:   false,
		LabelInput:   ti,
		IsLoading:    true,
		LoadingMsg:   "Loading NixOS generations...",
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
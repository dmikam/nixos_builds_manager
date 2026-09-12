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

type Model struct {
	Width        int
	Height       int
	Generations  []nix.Generation
	Cursor       int
	Focus        FocusArea
	ActiveButton int
	FreeSpace    string

	// Modals
	AboutModal        bool
	BuildModal        bool
	LabelInput        textinput.Model
	IsProfile         bool
	SwitchBuild       bool
	BuildModalOption  int

	ConfirmModal     bool
	PurgeModalOption int

	// Confirmation Dialog Modals
	ConfirmOptimizeModal bool
	OptimizeModalOption  int

	ConfirmGCModal bool
	GCModalOption  int

	ConfirmQuitModal bool
	QuitModalOption  int

	SwitchModal       bool
	SwitchTargetGen   *nix.Generation
	SwitchModalOption int

	RenameModal bool
	RenameGen   *nix.Generation
	RenameInput textinput.Model

	AnalyzeModal  bool
	AnalyzeGen    *nix.Generation
	AnalyzeResult string

	IsLoading  bool
	LoadingMsg string

	Viewport viewport.Model
	ShowLog  bool
	LogData  string

	Spinner spinner.Model
	Err     error
}

type GenerationsLoadedMsg []nix.Generation
type StreamOutputMsg string
type OperationCompletedMsg struct {
	Err error
}

func InitialModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	vp := viewport.New(80, 20)

	ti := textinput.New()
	ti.Placeholder = "Enter build label..."
	ti.CharLimit = 64
	ti.Width = 40

	ri := textinput.New()
	ri.Placeholder = "Enter new label..."
	ri.CharLimit = 64
	ri.Width = 40

	return Model{
		Generations:          []nix.Generation{},
		Cursor:               0,
		Focus:                FocusList,
		ActiveButton:         0,
		FreeSpace:            nix.GetNixStoreFreeSpace(),
		AboutModal:           false,
		ConfirmModal:         false,
		ConfirmOptimizeModal: false,
		ConfirmGCModal:       false,
		ConfirmQuitModal:     false,
		BuildModal:           false,
		SwitchModal:          false,
		RenameModal:          false,
		AnalyzeModal:         false,
		SwitchModalOption:    0,
		LabelInput:           ti,
		RenameInput:          ri,
		IsProfile:            false,
		SwitchBuild:          true,
		BuildModalOption:     0,
		PurgeModalOption:     0,
		OptimizeModalOption:  0,
		GCModalOption:        0,
		QuitModalOption:      0,
		IsLoading:            true,
		LoadingMsg:           "Loading NixOS generations...",
		Spinner:              s,
		Viewport:             vp,
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
			return OperationCompletedMsg{Err: err}
		}
		return GenerationsLoadedMsg(gens)
	}
}
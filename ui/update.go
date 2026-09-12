package ui

import (
	"fmt"

	"nixos_builds_manager/nix"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Viewport.Width = msg.Width - 6
		m.Viewport.Height = msg.Height - 8

	case tea.KeyMsg:
		if m.AboutModal {
			if msg.String() == "esc" || msg.String() == "enter" || msg.String() == "a" || msg.String() == "q" {
				m.AboutModal = false
			}
			return m, nil
		}
		if m.AnalyzeModal {
			if msg.String() == "esc" || msg.String() == "enter" || msg.String() == "s" {
				m.AnalyzeModal = false
			}
			return m, nil
		}
		if m.RenameModal {
			return m.handleRenameModalKeys(msg)
		}
		if m.SwitchModal {
			return m.handleSwitchModalKeys(msg)
		}
		if m.ConfirmModal {
			return m.handleModalKeys(msg)
		}
		if m.BuildModal {
			return m.handleBuildModalKeys(msg)
		}
		if m.ShowLog && !m.IsLoading {
			if msg.String() == "esc" || msg.String() == "q" || msg.String() == "enter" {
				m.ShowLog = false
				return m, fetchGenerationsCmd()
			}
			m.Viewport, cmd = m.Viewport.Update(msg)
			return m, cmd
		}
		return m.handleMainKeys(msg)

	case tea.MouseMsg:
		if msg.Type == tea.MouseWheelUp && m.Focus == FocusList {
			if m.Cursor > 0 {
				m.Cursor--
			}
		} else if msg.Type == tea.MouseWheelDown && m.Focus == FocusList {
			if m.Cursor < len(m.Generations)-1 {
				m.Cursor++
			}
		}

	case GenerationsLoadedMsg:
		m.Generations = msg
		m.FreeSpace = nix.GetNixStoreFreeSpace()
		m.IsLoading = false
		if m.Cursor >= len(m.Generations) && len(m.Generations) > 0 {
			m.Cursor = len(m.Generations) - 1
		}

	case StreamOutputMsg:
		m.LogData += string(msg) + "\n"
		m.Viewport.SetContent(m.LogData)
		m.Viewport.GotoBottom()
		return m, waitForStreamCmd()

	case OperationCompletedMsg:
		m.IsLoading = false
		m.FreeSpace = nix.GetNixStoreFreeSpace()
		if msg.Err != nil {
			m.LogData += fmt.Sprintf("\n[ERROR]: %v\n", msg.Err)
		} else {
			m.LogData += "\n[SUCCESS]: Operation completed successfully!\n"
		}
		m.Viewport.SetContent(m.LogData)
		m.Viewport.GotoBottom()

	case spinner.TickMsg:
		m.Spinner, cmd = m.Spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleRenameModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "enter":
		if m.RenameGen != nil {
			err := nix.RenameCustomProfile(*m.RenameGen, m.RenameInput.Value())
			if err != nil {
				m.LogData = fmt.Sprintf("Rename failed: %v", err)
			}
		}
		m.RenameModal = false
		return m, fetchGenerationsCmd()
	case "esc":
		m.RenameModal = false
		return m, nil
	}
	m.RenameInput, cmd = m.RenameInput.Update(msg)
	return m, cmd
}

func (m Model) handleSwitchModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h", "right", "l", "tab":
		if m.SwitchModalOption == 0 {
			m.SwitchModalOption = 1
		} else {
			m.SwitchModalOption = 0
		}
	case "y", "Y":
		return m.executeSwitch()
	case "n", "N", "esc":
		m.SwitchModal = false
	case "enter":
		if m.SwitchModalOption == 0 {
			return m.executeSwitch()
		}
		m.SwitchModal = false
	}
	return m, nil
}

func (m Model) executeSwitch() (tea.Model, tea.Cmd) {
	if m.SwitchTargetGen == nil {
		m.SwitchModal = false
		return m, nil
	}
	gen := *m.SwitchTargetGen
	m.SwitchModal = false
	m.IsLoading = true
	m.ShowLog = true
	m.LogData = fmt.Sprintf("Switching to Generation %d (%s)...\n\n", gen.ID, gen.Label)
	m.Viewport.SetContent(m.LogData)
	return m, runSwitchStreamCmd(gen)
}

func (m Model) handleModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h", "right", "l", "tab":
		if m.PurgeModalOption == 0 {
			m.PurgeModalOption = 1
		} else {
			m.PurgeModalOption = 0
		}
	case "y", "Y":
		return m.executePurge()
	case "n", "N", "esc":
		m.ConfirmModal = false
	case "enter":
		if m.PurgeModalOption == 0 {
			return m.executePurge()
		}
		m.ConfirmModal = false
	}
	return m, nil
}

func (m Model) executePurge() (tea.Model, tea.Cmd) {
	m.ConfirmModal = false
	m.IsLoading = true
	m.ShowLog = true
	m.LogData = "Starting purge operation...\n\n"
	m.Viewport.SetContent(m.LogData)
	return m, runPurgeStreamCmd(m.getSelectedGenerations())
}

func (m Model) handleBuildModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "tab":
		m.BuildModalOption = (m.BuildModalOption + 1) % 5
		if m.BuildModalOption == 0 {
			m.LabelInput.Focus()
		} else {
			m.LabelInput.Blur()
		}
		return m, nil

	case "shift+tab":
		m.BuildModalOption = (m.BuildModalOption + 4) % 5
		if m.BuildModalOption == 0 {
			m.LabelInput.Focus()
		} else {
			m.LabelInput.Blur()
		}
		return m, nil

	case "up":
		if m.BuildModalOption > 0 {
			m.BuildModalOption--
			if m.BuildModalOption == 0 {
				m.LabelInput.Focus()
			}
		}

	case "down":
		if m.BuildModalOption < 4 {
			m.BuildModalOption++
			if m.BuildModalOption != 0 {
				m.LabelInput.Blur()
			}
		}

	case "left", "right":
		if m.BuildModalOption == 3 || m.BuildModalOption == 4 {
			if m.BuildModalOption == 3 {
				m.BuildModalOption = 4
			} else {
				m.BuildModalOption = 3
			}
		}

	case " ":
		if m.BuildModalOption == 1 {
			m.IsProfile = !m.IsProfile
		} else if m.BuildModalOption == 2 {
			m.SwitchBuild = !m.SwitchBuild
		}
		return m, nil

	case "enter":
		if m.BuildModalOption == 4 {
			m.BuildModal = false
			return m, nil
		}
		m.BuildModal = false
		m.IsLoading = true
		m.ShowLog = true
		rawLabel := m.LabelInput.Value()
		cleanLabel := nix.SanitizeLabel(rawLabel)
		m.LogData = fmt.Sprintf("Starting build (Label: %s)...\n\n", cleanLabel)
		m.Viewport.SetContent(m.LogData)
		return m, runBuildStreamCmd(cleanLabel, m.IsProfile, m.SwitchBuild)

	case "esc":
		m.BuildModal = false
		return m, nil
	}

	if m.BuildModalOption == 0 {
		m.LabelInput, cmd = m.LabelInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleMainKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "f1", "a":
		m.AboutModal = true
		return m, nil

	case "f2", "n":
		m.ActiveButton = 1
		return m.executeButtonAction()

	case "f5", "r":
		m.ActiveButton = 2
		return m.executeButtonAction()

	case "f6", "o":
		m.ActiveButton = 3
		return m.executeButtonAction()

	case "f7", "c":
		m.ActiveButton = 4
		return m.executeButtonAction()

	case "f8", "p":
		if len(m.getSelectedGenerations()) > 0 {
			m.ActiveButton = 5
			return m.executeButtonAction()
		}

	case "f10", "q", "ctrl+c":
		return m, tea.Quit

	case "e":
		if m.Focus == FocusList && len(m.Generations) > m.Cursor {
			selected := &m.Generations[m.Cursor]
			m.RenameGen = selected
			m.RenameInput.SetValue(selected.Label)
			m.RenameInput.Focus()
			m.RenameModal = true
			return m, textinput.Blink
		}

	case "s":
		if m.Focus == FocusList && len(m.Generations) > m.Cursor {
			selected := &m.Generations[m.Cursor]
			size, err := nix.AnalyzeStorePathSize(selected.Target)
			if err != nil {
				size = "Failed to evaluate"
			}
			m.AnalyzeGen = selected
			m.AnalyzeResult = size
			m.AnalyzeModal = true
			return m, nil
		}

	case "tab":
		if m.Focus == FocusList {
			m.Focus = FocusFooter
		} else {
			m.Focus = FocusList
		}

	case "up", "k":
		if m.Focus == FocusList && m.Cursor > 0 {
			m.Cursor--
		}

	case "down", "j":
		if m.Focus == FocusList && m.Cursor < len(m.Generations)-1 {
			m.Cursor++
		}

	case "left":
		if m.Focus == FocusFooter && m.ActiveButton > 0 {
			m.ActiveButton--
		}

	case "right":
		if m.Focus == FocusFooter && m.ActiveButton < 6 {
			m.ActiveButton++
		}

	case " ":
		if m.Focus == FocusList && len(m.Generations) > 0 {
			gen := &m.Generations[m.Cursor]
			if !gen.IsCurrent {
				gen.Marked = !gen.Marked
			}
		}

	case "enter":
		if m.Focus == FocusList && len(m.Generations) > m.Cursor {
			selected := &m.Generations[m.Cursor]
			if !selected.IsCurrent {
				m.SwitchTargetGen = selected
				m.SwitchModal = true
				m.SwitchModalOption = 0
				return m, nil
			}
		} else if m.Focus == FocusFooter {
			return m.executeButtonAction()
		}
	}
	return m, nil
}

func (m Model) executeButtonAction() (tea.Model, tea.Cmd) {
	switch m.ActiveButton {
	case 0: // About
		m.AboutModal = true
	case 1: // New Build
		m.BuildModal = true
		m.BuildModalOption = 0
		m.IsProfile = false
		m.SwitchBuild = true
		m.LabelInput.Reset()
		m.LabelInput.Focus()
		return m, textinput.Blink
	case 2: // Refresh
		m.IsLoading = true
		m.LoadingMsg = "Refreshing generations..."
		return m, fetchGenerationsCmd()
	case 3: // Optimize Store
		m.IsLoading = true
		m.ShowLog = true
		m.LogData = "Optimizing Nix store...\n\n"
		m.Viewport.SetContent(m.LogData)
		return m, runOptimizeStreamCmd()
	case 4: // Clean GC
		m.IsLoading = true
		m.ShowLog = true
		m.LogData = "Cleaning unreferenced store paths...\n\n"
		m.Viewport.SetContent(m.LogData)
		return m, runGarbageCollectStreamCmd()
	case 5: // Purge Selected
		if len(m.getSelectedGenerations()) > 0 {
			m.ConfirmModal = true
			m.PurgeModalOption = 0
		}
	case 6: // Quit
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) getSelectedGenerations() []nix.Generation {
	var selected []nix.Generation
	for _, g := range m.Generations {
		if g.Marked && !g.IsCurrent {
			selected = append(selected, g)
		}
	}
	return selected
}

var globalStreamChan chan string

func waitForStreamCmd() tea.Cmd {
	return func() tea.Msg {
		line, ok := <-globalStreamChan
		if !ok {
			return nil
		}
		return StreamOutputMsg(line)
	}
}

func runBuildStreamCmd(label string, isProfile bool, switchBuild bool) tea.Cmd {
	globalStreamChan = make(chan string, 100)
	return tea.Batch(
		func() tea.Msg {
			err := nix.RebuildSystemStream(label, isProfile, switchBuild, globalStreamChan)
			close(globalStreamChan)
			return OperationCompletedMsg{Err: err}
		},
		waitForStreamCmd(),
	)
}

func runSwitchStreamCmd(gen nix.Generation) tea.Cmd {
	globalStreamChan = make(chan string, 100)
	return tea.Batch(
		func() tea.Msg {
			err := nix.SwitchToGenerationStream(gen, globalStreamChan)
			close(globalStreamChan)
			return OperationCompletedMsg{Err: err}
		},
		waitForStreamCmd(),
	)
}

func runPurgeStreamCmd(gens []nix.Generation) tea.Cmd {
	globalStreamChan = make(chan string, 100)
	return tea.Batch(
		func() tea.Msg {
			err := nix.PurgeGenerationsStream(gens, globalStreamChan)
			close(globalStreamChan)
			return OperationCompletedMsg{Err: err}
		},
		waitForStreamCmd(),
	)
}

func runGarbageCollectStreamCmd() tea.Cmd {
	globalStreamChan = make(chan string, 100)
	return tea.Batch(
		func() tea.Msg {
			err := nix.CollectGarbageStream(globalStreamChan)
			close(globalStreamChan)
			return OperationCompletedMsg{Err: err}
		},
		waitForStreamCmd(),
	)
}

func runOptimizeStreamCmd() tea.Cmd {
	globalStreamChan = make(chan string, 100)
	return tea.Batch(
		func() tea.Msg {
			err := nix.OptimizeStoreStream(globalStreamChan)
			close(globalStreamChan)
			return OperationCompletedMsg{Err: err}
		},
		waitForStreamCmd(),
	)
}
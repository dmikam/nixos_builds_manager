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
		if m.ConfirmModal {
			return m.handleModalKeys(msg)
		}
		if m.BuildModal {
			return m.handleBuildModalKeys(msg)
		}
		if m.ShowLog {
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
		m.IsLoading = false
		if m.Cursor >= len(m.Generations) && len(m.Generations) > 0 {
			m.Cursor = len(m.Generations) - 1
		}

	case OperationFinishedMsg:
		m.IsLoading = false
		m.ConfirmModal = false
		m.BuildModal = false
		if msg.Err != nil {
			m.LogData = fmt.Sprintf("Error: %v\n\nOutput:\n%s", msg.Err, msg.Output)
		} else {
			m.LogData = fmt.Sprintf("Operation completed successfully!\n\nOutput:\n%s", msg.Output)
		}
		m.Viewport.SetContent(m.LogData)
		m.ShowLog = true

	case spinner.TickMsg:
		m.Spinner, cmd = m.Spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.ConfirmModal = false
		m.IsLoading = true
		m.LoadingMsg = "Purging selected generations..."
		return m, runPurgeCmd(m.getSelectedGenerations())
	case "n", "N", "esc":
		m.ConfirmModal = false
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

func (m Model) handleBuildModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "enter":
		m.BuildModal = false
		m.IsLoading = true
		label := m.LabelInput.Value()
		m.LoadingMsg = fmt.Sprintf("Building new system generation (%s)...", label)
		return m, runBuildCmd(label)
	case "esc":
		m.BuildModal = false
		return m, nil
	}
	m.LabelInput, cmd = m.LabelInput.Update(msg)
	return m, cmd
}

func (m Model) handleMainKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

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

	case "left", "h":
		if m.Focus == FocusFooter && m.ActiveButton > 0 {
			m.ActiveButton--
		}

	case "right", "l":
		if m.Focus == FocusFooter && m.ActiveButton < 4 {
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
		if m.Focus == FocusFooter {
			return m.executeButtonAction()
		}
	}
	return m, nil
}

func (m Model) executeButtonAction() (tea.Model, tea.Cmd) {
	switch m.ActiveButton {
	case 0: // New Build
		m.BuildModal = true
		m.LabelInput.Reset()
		m.LabelInput.Focus()
		return m, textinput.Blink
	case 1: // Purge Selected
		marked := m.getSelectedIDs()
		if len(marked) > 0 {
			m.ConfirmModal = true
		}
	case 2: // Optimize Store
		m.IsLoading = true
		m.LoadingMsg = "Optimizing Nix Store..."
		return m, runOptimizeCmd()
	case 3: // Refresh
		m.IsLoading = true
		m.LoadingMsg = "Refreshing generations..."
		return m, fetchGenerationsCmd()
	case 4: // Quit
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) getSelectedIDs() []int {
	var ids []int
	for _, g := range m.Generations {
		if g.Marked && !g.IsCurrent {
			ids = append(ids, g.ID)
		}
	}
	return ids
}

func runBuildCmd(label string) tea.Cmd {
	return func() tea.Msg {
		out, err := nix.RebuildSystem(label)
		return OperationFinishedMsg{Output: out, Err: err}
	}
}

func runPurgeCmd(gens []nix.Generation) tea.Cmd {
	return func() tea.Msg {
		out, err := nix.PurgeGenerations(gens)
		return OperationFinishedMsg{Output: out, Err: err}
	}
}

func runOptimizeCmd() tea.Cmd {
	return func() tea.Msg {
		out, err := nix.OptimizeStore()
		return OperationFinishedMsg{Output: out, Err: err}
	}
}
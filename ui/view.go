package ui

import (
	"fmt"
	"strings"

	"nixos_builds_manager/ui/styles"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return "Initializing terminal size..."
	}

	if m.ShowLog {
		hint := "Running operation... please wait."
		if !m.IsLoading {
			hint = "Press [ESC / Enter / q] to return"
		}
		logView := fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			styles.HeaderTitle.Render(" --- OPERATION LOG / OUTPUT --- "),
			m.Viewport.View(),
			styles.HeaderInfo.Render(hint),
		)
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, logView)
	}

	if m.IsLoading {
		msg := fmt.Sprintf("%s %s", m.Spinner.View(), m.LoadingMsg)
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, msg)
	}

	if m.AboutModal {
		return m.renderAboutModal()
	}

	if m.AnalyzeModal {
		return m.renderAnalyzeModal()
	}

	if m.RenameModal {
		return m.renderRenameModal()
	}

	if m.SwitchModal {
		return m.renderSwitchModal()
	}

	if m.BuildModal {
		return m.renderBuildModal()
	}

	if m.ConfirmModal {
		return m.renderConfirmModal()
	}

	if m.ConfirmOptimizeModal {
		return m.renderOptimizeModal()
	}

	if m.ConfirmGCModal {
		return m.renderGCModal()
	}

	if m.ConfirmQuitModal {
		return m.renderQuitModal()
	}

	mainWidth := m.Width - 2
	if mainWidth < 40 {
		mainWidth = 40
	}

	headerBarHeight := 1
	footerBarHeight := 1
	panelHeight := m.Height - headerBarHeight - footerBarHeight - 1
	if panelHeight < 10 {
		panelHeight = 10
	}

	currentGen := "Unknown"
	for _, g := range m.Generations {
		if g.IsCurrent {
			currentGen = fmt.Sprintf("Gen %d (%s)", g.ID, g.Label)
			break
		}
	}
	headerText := fmt.Sprintf(" NixOS Builds Manager | Active: %s ", currentGen)
	headerTextPadded := fmt.Sprintf("%-*s", mainWidth, headerText)
	headerView := styles.HeaderTitle.Render(headerTextPadded)

	var content strings.Builder

	colMark := "Mark"
	colID := "ID"
	colLabel := "Build Label"
	colKernel := "Kernel"
	colDate := "Date & Time"
	colStatus := "Status"
	colTarget := "Nix Store Path Target"

	tblHeader := fmt.Sprintf(" %-4s %-6s %-22s %-16s %-18s %-10s %s", colMark, colID, colLabel, colKernel, colDate, colStatus, colTarget)
	tblHeaderPadded := fmt.Sprintf("%-*s", mainWidth-4, tblHeader)
	content.WriteString(styles.TableHeader.Render(tblHeaderPadded) + "\n")

	visibleRows := panelHeight - 4
	if visibleRows < 1 {
		visibleRows = 1
	}

	startIdx := 0
	if m.Cursor >= visibleRows {
		startIdx = m.Cursor - visibleRows + 1
	}
	endIdx := startIdx + visibleRows
	if endIdx > len(m.Generations) {
		endIdx = len(m.Generations)
	}

	for i := startIdx; i < endIdx; i++ {
		g := m.Generations[i]

		mark := "[ ]"
		if g.Marked {
			mark = "[X]"
		}

		status := " "
		if g.IsCurrent {
			status = styles.CurrentBadge.Render("CURRENT")
		} else if g.Marked {
			status = styles.MarkedBadge.Render("PURGE")
		}

		labelTrunc := g.Label
		if len(labelTrunc) > 21 {
			labelTrunc = labelTrunc[:18] + "..."
		}

		kernelTrunc := g.Kernel
		if len(kernelTrunc) > 15 {
			kernelTrunc = kernelTrunc[:12] + "..."
		}

		dateStr := g.Timestamp.Format("2006-01-02 15:04")
		targetTrunc := g.Target
		maxTargetWidth := mainWidth - 86
		if maxTargetWidth > 10 && len(targetTrunc) > maxTargetWidth {
			targetTrunc = "..." + targetTrunc[len(targetTrunc)-maxTargetWidth+3:]
		}

		rowStr := fmt.Sprintf(" %-4s %-6d %-22s %-16s %-18s %-10s %s", mark, g.ID, labelTrunc, kernelTrunc, dateStr, status, targetTrunc)
		rowPadded := fmt.Sprintf("%-*s", mainWidth-4, rowStr)

		if i == m.Cursor && m.Focus == FocusList {
			content.WriteString(styles.SelectedRow.Render(rowPadded) + "\n")
		} else {
			content.WriteString(styles.NormalRow.Render(rowPadded) + "\n")
		}
	}

	renderedRows := endIdx - startIdx
	for i := renderedRows; i < visibleRows; i++ {
		emptyPadded := fmt.Sprintf("%-*s", mainWidth-4, "")
		content.WriteString(styles.NormalRow.Render(emptyPadded) + "\n")
	}

	panelView := styles.PanelStyle.
		Width(mainWidth).
		Height(panelHeight).
		Render(content.String())

	buttons := []struct {
		Text     string
		Disabled bool
	}{
		{"F1 [A]bout", false},
		{"F2 R[e]name", false},
		{"F3 [N]ew Build", false},
		{"F4 [S]torage", false},
		{"F5 [R]efresh", false},
		{"F6 [O]ptimize", false},
		{"F7 [C]lean GC", false},
		{"F8 [P]urge", len(m.getSelectedGenerations()) == 0},
		{"F10 [Q]uit", false},
	}

	var btnViews []string
	for i, btn := range buttons {
		if btn.Disabled {
			btnViews = append(btnViews, styles.ButtonDisabled.Render(btn.Text))
		} else if m.Focus == FocusFooter && m.ActiveButton == i {
			btnViews = append(btnViews, styles.ButtonActive.Render(btn.Text))
		} else {
			btnViews = append(btnViews, styles.ButtonNormal.Render(btn.Text))
		}
	}

	leftFooter := strings.Join(btnViews, " ")
	rightFooter := fmt.Sprintf(" %s | v0.1 ", m.FreeSpace)
	gapWidth := mainWidth - lipgloss.Width(leftFooter) - lipgloss.Width(rightFooter)
	if gapWidth < 0 {
		gapWidth = 0
	}

	footerContent := leftFooter + strings.Repeat(" ", gapWidth) + rightFooter
	footerView := styles.FooterBarStyle.Render(footerContent)

	fullApp := lipgloss.JoinVertical(
		lipgloss.Left,
		headerView,
		panelView,
		footerView,
	)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, fullApp)
}

func (m Model) renderAboutModal() string {
	msg := fmt.Sprintf(
		"NIXOS BUILDS MANAGER v0.1\n\nTerminal UI tool to manage, switch, purge, and analyze NixOS system generations.\n\nGitHub:\nhttps://github.com/dmikam/nixos-builds-manager\n\nPress ESC / Enter / A / Q to close",
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}

func (m Model) renderAnalyzeModal() string {
	if m.AnalyzeGen == nil {
		return ""
	}
	msg := fmt.Sprintf(
		"STORE PATH ANALYZER\n\nGeneration: %d (%s)\nStore Path:\n%s\n\nClosure Disk Usage:\n%s\n\nPress ESC / Enter / S / F4 to close",
		m.AnalyzeGen.ID,
		m.AnalyzeGen.Label,
		m.AnalyzeGen.Target,
		m.AnalyzeResult,
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}

func (m Model) renderRenameModal() string {
	if m.RenameGen == nil {
		return ""
	}
	msg := fmt.Sprintf(
		"EDIT CUSTOM PROFILE LABEL\n\nGeneration: %d\nPath: %s\n\nNew Label:\n%s\n\nPress Enter to save, ESC to cancel",
		m.RenameGen.ID,
		m.RenameGen.Path,
		m.RenameInput.View(),
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}

func (m Model) renderSwitchModal() string {
	if m.SwitchTargetGen == nil {
		return ""
	}

	btnYes := styles.ButtonNormal.Render(" Yes, Switch ")
	btnNo := styles.ButtonNormal.Render(" Cancel ")

	if m.SwitchModalOption == 0 {
		btnYes = styles.ButtonActive.Render(" Yes, Switch ")
	} else {
		btnNo = styles.ButtonActive.Render(" Cancel ")
	}

	msg := fmt.Sprintf(
		"SWITCH SYSTEM GENERATION\n\nAre you sure you want to switch to:\nGeneration %d (%s)?\n\n%s   %s",
		m.SwitchTargetGen.ID,
		m.SwitchTargetGen.Label,
		btnYes,
		btnNo,
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}

func (m Model) renderBuildModal() string {
	chkProfile := "[ ] As a new Profile"
	if m.IsProfile {
		chkProfile = "[X] As a new Profile"
	}
	if m.BuildModalOption == 1 {
		chkProfile = styles.ButtonActive.Render(chkProfile)
	}

	chkSwitch := "[ ] Switch to this build"
	if m.SwitchBuild {
		chkSwitch = "[X] Switch to this build"
	}
	if m.BuildModalOption == 2 {
		chkSwitch = styles.ButtonActive.Render(chkSwitch)
	}

	btnStart := styles.ButtonNormal.Render(" Start Build ")
	btnCancel := styles.ButtonNormal.Render(" Cancel ")

	if m.BuildModalOption == 3 {
		btnStart = styles.ButtonActive.Render(" Start Build ")
	} else if m.BuildModalOption == 4 {
		btnCancel = styles.ButtonActive.Render(" Cancel ")
	}

	msg := fmt.Sprintf(
		"CREATE NEW NIXOS BUILD\n\nLabel:\n%s\n\n%s\n%s\n\n%s   %s",
		m.LabelInput.View(),
		chkProfile,
		chkSwitch,
		btnStart,
		btnCancel,
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}

func (m Model) renderConfirmModal() string {
	ids := m.getSelectedGenerations()

	btnYes := styles.ButtonNormal.Render(" Yes, Purge ")
	btnNo := styles.ButtonNormal.Render(" Cancel ")

	if m.PurgeModalOption == 0 {
		btnYes = styles.ButtonActive.Render(" Yes, Purge ")
	} else {
		btnNo = styles.ButtonActive.Render(" Cancel ")
	}

	msg := fmt.Sprintf(
		"CONFIRM PURGE GENERATIONS\n\nAre you sure you want to PURGE %d generation(s)?\n\n%s   %s",
		len(ids),
		btnYes,
		btnNo,
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}

func (m Model) renderOptimizeModal() string {
	btnYes := styles.ButtonNormal.Render(" Yes, Optimize ")
	btnNo := styles.ButtonNormal.Render(" Cancel ")

	if m.OptimizeModalOption == 0 {
		btnYes = styles.ButtonActive.Render(" Yes, Optimize ")
	} else {
		btnNo = styles.ButtonActive.Render(" Cancel ")
	}

	msg := fmt.Sprintf(
		"CONFIRM STORE OPTIMIZATION\n\nOptimize Nix store by hardlinking identical files?\n\n%s   %s",
		btnYes,
		btnNo,
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}

func (m Model) renderGCModal() string {
	btnYes := styles.ButtonNormal.Render(" Yes, Clean GC ")
	btnNo := styles.ButtonNormal.Render(" Cancel ")

	if m.GCModalOption == 0 {
		btnYes = styles.ButtonActive.Render(" Yes, Clean GC ")
	} else {
		btnNo = styles.ButtonActive.Render(" Cancel ")
	}

	msg := fmt.Sprintf(
		"CONFIRM GARBAGE COLLECTION\n\nRun Nix Garbage Collector to remove unreferenced store paths?\n\n%s   %s",
		btnYes,
		btnNo,
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}

func (m Model) renderQuitModal() string {
	btnYes := styles.ButtonNormal.Render(" Yes, Quit ")
	btnNo := styles.ButtonNormal.Render(" Cancel ")

	if m.QuitModalOption == 0 {
		btnYes = styles.ButtonActive.Render(" Yes, Quit ")
	} else {
		btnNo = styles.ButtonActive.Render(" Cancel ")
	}

	msg := fmt.Sprintf(
		"CONFIRM EXIT\n\nAre you sure you want to quit NixOS Builds Manager?\n\n%s   %s",
		btnYes,
		btnNo,
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}
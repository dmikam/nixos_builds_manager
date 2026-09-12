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

	if m.IsLoading {
		msg := fmt.Sprintf("%s %s", m.Spinner.View(), m.LoadingMsg)
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, msg)
	}

	if m.ShowLog {
		logView := fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			styles.HeaderTitle.Render(" --- OPERATION LOG / OUTPUT --- "),
			m.Viewport.View(),
			styles.HeaderInfo.Render("Press [ESC / Enter / q] to return"),
		)
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, logView)
	}

	if m.BuildModal {
		return m.renderBuildModal()
	}

	if m.ConfirmModal {
		return m.renderConfirmModal()
	}

	// Calculate panel sizes
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

	// 1. Top Header Bar
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

	// 2. Main Content (MC Panel Style)
	var content strings.Builder

	// Table Header inside Panel
	colMark := "Mark"
	colID := "ID"
	colLabel := "Build Label"
	colDate := "Date & Time"
	colStatus := "Status"
	colTarget := "Nix Store Path Target"

	tblHeader := fmt.Sprintf(" %-4s %-6s %-25s %-18s %-10s %s", colMark, colID, colLabel, colDate, colStatus, colTarget)
	tblHeaderPadded := fmt.Sprintf("%-*s", mainWidth-4, tblHeader)
	content.WriteString(styles.TableHeader.Render(tblHeaderPadded) + "\n")

	// Max visible rows based on panel height
	visibleRows := panelHeight - 4
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Calculate pagination scroll window
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
		if len(labelTrunc) > 24 {
			labelTrunc = labelTrunc[:21] + "..."
		}

		dateStr := g.Timestamp.Format("2006-01-02 15:04")
		targetTrunc := g.Target
		maxTargetWidth := mainWidth - 72
		if maxTargetWidth > 10 && len(targetTrunc) > maxTargetWidth {
			targetTrunc = "..." + targetTrunc[len(targetTrunc)-maxTargetWidth+3:]
		}

		rowStr := fmt.Sprintf(" %-4s %-6d %-25s %-18s %-10s %s", mark, g.ID, labelTrunc, dateStr, status, targetTrunc)
		rowPadded := fmt.Sprintf("%-*s", mainWidth-4, rowStr)

		if i == m.Cursor && m.Focus == FocusList {
			content.WriteString(styles.SelectedRow.Render(rowPadded) + "\n")
		} else {
			content.WriteString(styles.NormalRow.Render(rowPadded) + "\n")
		}
	}

	// Fill remaining empty space in panel to maintain height
	renderedRows := endIdx - startIdx
	for i := renderedRows; i < visibleRows; i++ {
		emptyPadded := fmt.Sprintf("%-*s", mainWidth-4, "")
		content.WriteString(styles.NormalRow.Render(emptyPadded) + "\n")
	}

	panelView := styles.PanelStyle.
		Width(mainWidth).
		Height(panelHeight).
		Render(content.String())

	// 3. Bottom Footer Bar / Function Keys
	buttons := []struct {
		Key   string
		Label string
	}{
		{"F1/1", "New Build"},
		{"F2/2", "Purge Selected"},
		{"F3/3", "Optimize Store"},
		{"F4/4", "Refresh"},
		{"F5/5", "Quit"},
	}

	var btnViews []string
	for i, btn := range buttons {
		btnStr := fmt.Sprintf("%s:%s", btn.Key, btn.Label)
		if m.Focus == FocusFooter && m.ActiveButton == i {
			btnViews = append(btnViews, styles.ButtonActive.Render(btnStr))
		} else {
			btnViews = append(btnViews, styles.ButtonNormal.Render(btnStr))
		}
	}

	footerContent := strings.Join(btnViews, " ")
	footerPadded := fmt.Sprintf("%-*s", mainWidth, footerContent)
	footerView := styles.FooterBarStyle.Render(footerPadded)

	// Combine components & center whole layout
	fullApp := lipgloss.JoinVertical(
		lipgloss.Left,
		headerView,
		panelView,
		footerView,
	)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, fullApp)
}

func (m Model) renderBuildModal() string {
	msg := fmt.Sprintf(
		"CREATE NEW NIXOS BUILD\n\nEnter optional profile label for the build:\n\n%s\n\n[Enter] Start Build  /  [ESC] Cancel",
		m.LabelInput.View(),
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}

func (m Model) renderConfirmModal() string {
	ids := m.getSelectedIDs()
	msg := fmt.Sprintf(
		"CONFIRM PURGE GENERATIONS\n\nAre you sure you want to PURGE %d generations?\nIDs: %v\n\n[Y] Yes, proceed  /  [N] Cancel",
		len(ids), ids,
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}
package ui

import (
	"fmt"
	"strings"

	"nixos_builds_manager/ui/styles"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return "Initializing..."
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

	if m.ConfirmModal {
		return m.renderModal()
	}

	mainWidth := m.Width - 2
	if mainWidth < 40 {
		mainWidth = 40
	}

	panelHeight := m.Height - 3
	if panelHeight < 8 {
		panelHeight = 8
	}

	// 1. Header Bar
	currentGen := "Unknown"
	for _, g := range m.Generations {
		if g.IsCurrent {
			currentGen = fmt.Sprintf("Gen %d (%s)", g.ID, g.Label)
			break
		}
	}
	headerText := fmt.Sprintf(" NixOS Builds Manager | Booted: %s ", currentGen)
	headerTextPadded := fmt.Sprintf("%-*s", mainWidth, headerText)
	headerView := styles.HeaderTitle.Render(headerTextPadded)

	// 2. MC Table Panel
	var content strings.Builder
	tblHeader := fmt.Sprintf(" %-4s %-6s %-18s %-10s %s", "Mark", "ID", "Date & Time", "Status", "Nix Store Path Target")
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

		dateStr := g.Timestamp.Format("2006-01-02 15:04")
		targetTrunc := g.Target
		maxTargetWidth := mainWidth - 45
		if maxTargetWidth > 10 && len(targetTrunc) > maxTargetWidth {
			targetTrunc = "..." + targetTrunc[len(targetTrunc)-maxTargetWidth+3:]
		}

		rowStr := fmt.Sprintf(" %-4s %-6d %-18s %-10s %s", mark, g.ID, dateStr, status, targetTrunc)
		rowPadded := fmt.Sprintf("%-*s", mainWidth-4, rowStr)

		if i == m.Cursor && m.Focus == FocusList {
			content.WriteString(styles.SelectedRow.Render(rowPadded) + "\n")
		} else {
			content.WriteString(styles.NormalRow.Render(rowPadded) + "\n")
		}
	}

	// Pad remaining space
	for i := endIdx - startIdx; i < visibleRows; i++ {
		emptyPadded := fmt.Sprintf("%-*s", mainWidth-4, "")
		content.WriteString(styles.NormalRow.Render(emptyPadded) + "\n")
	}

	panelView := styles.PanelStyle.
		Width(mainWidth).
		Height(panelHeight).
		Render(content.String())

	// 3. Footer Bar
	buttons := []struct {
		Key   string
		Label string
	}{
		{"F1/1", "Purge Selected"},
		{"F2/2", "Optimize Store"},
		{"F3/3", "Refresh"},
		{"F4/4", "Quit"},
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

	fullApp := lipgloss.JoinVertical(
		lipgloss.Left,
		headerView,
		panelView,
		footerView,
	)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, fullApp)
}

func (m Model) renderModal() string {
	ids := m.getSelectedIDs()
	msg := fmt.Sprintf(
		"CONFIRM PURGE GENERATIONS\n\nAre you sure you want to PURGE %d generations?\nIDs: %v\n\n[Y] Yes, proceed  /  [N] Cancel",
		len(ids), ids,
	)
	modalView := styles.ModalStyle.Render(msg)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalView)
}
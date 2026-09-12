package ui

import (
	"fmt"
	"strings"

	"nixos_builds_manager/ui/styles"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.IsLoading {
		return fmt.Sprintf("\n  %s %s\n", m.Spinner.View(), m.LoadingMsg)
	}

	if m.ShowLog {
		return fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			styles.HeaderStyle.Render("Operation Log / Output"),
			m.Viewport.View(),
			styles.SubHeaderStyle.Render("Press [ESC/Enter/q] to return"),
		)
	}

	if m.ConfirmModal {
		return m.renderModal()
	}

	var doc strings.Builder

	// Header
	doc.WriteString(styles.HeaderStyle.Render("NixOS Generation Manager & Store Optimizer"))
	doc.WriteString("\n")

	// Current info
	currentGen := "Unknown"
	for _, g := range m.Generations {
		if g.IsCurrent {
			currentGen = fmt.Sprintf("Gen %d (%s)", g.ID, g.Label)
			break
		}
	}
	doc.WriteString(fmt.Sprintf(" Booted System: %s\n\n", styles.SubHeaderStyle.Render(currentGen)))

	// Main Table Header
	doc.WriteString(fmt.Sprintf(" %-4s %-6s %-20s %-12s %s\n", "Mark", "ID", "Date", "Status", "Store Target"))
	doc.WriteString(" " + strings.Repeat("-", 75) + "\n")

	// Table Rows
	for i, g := range m.Generations {
		mark := "[ ]"
		if g.Marked {
			mark = "[X]"
		}

		status := ""
		if g.IsCurrent {
			status = styles.CurrentBadge.Render("CURRENT")
		} else if g.Marked {
			status = styles.MarkedBadge.Render("PURGE")
		}

		dateStr := g.Timestamp.Format("2006-01-02 15:04")
		targetTrunc := g.Target
		if len(targetTrunc) > 30 {
			targetTrunc = "..." + targetTrunc[len(targetTrunc)-27:]
		}

		rowStr := fmt.Sprintf("%-4s %-6d %-20s %-12s %s", mark, g.ID, dateStr, status, targetTrunc)

		if i == m.Cursor && m.Focus == FocusList {
			doc.WriteString(styles.SelectedRow.Render(rowStr) + "\n")
		} else {
			doc.WriteString(styles.NormalRow.Render(rowStr) + "\n")
		}
	}

	doc.WriteString("\n")

	// Footer Action Buttons
	buttons := []string{"Purge Selected", "Optimize Store", "Refresh", "Quit"}
	var btnViews []string

	for i, btn := range buttons {
		if m.Focus == FocusFooter && m.ActiveButton == i {
			btnViews = append(btnViews, styles.ActiveButtonStyle.Render(btn))
		} else {
			btnViews = append(btnViews, styles.ButtonStyle.Render(btn))
		}
	}

	doc.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, btnViews...))
	doc.WriteString("\n\n")

	// Keybind hints
	doc.WriteString(styles.SubHeaderStyle.Render("Controls: ") + "[Tab] Focus toggle | [Space] Mark/Unmark | [Arrows] Navigate | [Enter] Select Action")

	return doc.String()
}

func (m Model) renderModal() string {
	ids := m.getSelectedIDs()
	msg := fmt.Sprintf("Are you sure you want to PURGE %d generations?\nIDs: %v\n\n[Y] Yes, proceed  /  [N] Cancel", len(ids), ids)
	return lipgloss.Place(
		80, 15,
		lipgloss.Center, lipgloss.Center,
		styles.ModalStyle.Render(msg),
	)
}
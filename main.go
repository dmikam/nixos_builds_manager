package main

import (
	"fmt"
	"os"

	"nixos_builds_manager/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if os.Geteuid() != 0 {
		fmt.Println("Error: This application requires root permissions.")
		fmt.Println("Please run it using sudo or as the root user:")
		fmt.Println("  sudo ./nixos_builds_manager")
		os.Exit(1)
	}

	p := tea.NewProgram(ui.InitialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
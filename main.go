package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// hyprsunset-controller — Bubble Tea TUI scaffold
//
// Extend here:
//   - keys:       add cases in Update's tea.KeyMsg switch, document them in View
//   - IPC:        add hyprsunset commands in hyprsunset.go, call them from a tea.Cmd
//   - persistence: load before tea.NewProgram, save after Run returns

func main() {
	// Check if Dependencies are installed
	if err := CheckDependencies(); err != nil {
		if notifyErr := Notify(err.Error()); notifyErr != nil {
			fmt.Fprintln(os.Stderr, "Notification Error:", notifyErr) // Print this just if Notify() errors
		}

		fmt.Fprintln(os.Stderr, "Error:", err) // Always show the dependencies error
		os.Exit(1)
	}

	// Create the TUI
	if _, err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

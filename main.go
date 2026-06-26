package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

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

// CheckDependencies verifies the external commands hyprsunset-tui needs are available.
func CheckDependencies() error {
	for _, bin := range []string{"hyprsunset", "uwsm", "notify-send"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found in PATH", bin)
		}
	}

	return nil
}

// Notify sends a critical desktop notification for hyprsunset-tui.
func Notify(body string) error {
	return exec.Command(
		"notify-send",
		"-a", "hyprsunset-tui",
		"-u", "critical",
		"hyprsunset-tui",
		body,
	).Run()
}

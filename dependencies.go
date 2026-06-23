package main

import (
	"fmt"
	"os/exec"
)

func CheckDependencies() error {
	// Check if hyprsunset is in PATH
	if _, err := exec.LookPath("hyprsunset"); err != nil {
		return fmt.Errorf("hyprsunset is not found in PATH (Not installed?)")
	}

	// Check if hyprctl is in PATH
	// I think it is possible to install hyprsunset without hyprland
	// Could get rid of this section if you cannot
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return fmt.Errorf("hyprctl is not found in PATH (Not installed?)")
	}

	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl is not found in PATH (Not installed?)")
	}

	return nil
}

func Notify(title, body string) error {
	// Check if notify-send is in the PATH
	if _, err := exec.LookPath("notify-send"); err != nil {
		return fmt.Errorf("notify-send is not found in PATH (Not installed?)")
	}

	// This runs the notify-send command
	return exec.Command(
		"notify-send",
		"-a", "hyprsunset-controller",
		"-u", "critical",
		title,
		body,
	).Run()
}

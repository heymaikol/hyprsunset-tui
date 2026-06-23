package main

import (
	"errors"
	"fmt"
	"os/exec"
)

func CheckDependencies() error {
	// hyprctl is checked because hyprsunset may be installed without Hyprland
	for _, bin := range []string{"hyprsunset", "hyprctl", "uwsm", "pgrep", "pkill"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found in PATH", bin)
		}
	}

	return nil
}

func Notify(title, body string) error {
	// Check if notify-send is in the PATH
	if _, err := exec.LookPath("notify-send"); err != nil {
		return errors.New("notify-send is not found in PATH (Not installed?)")
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

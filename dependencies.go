package main

import (
	"fmt"
	"os/exec"
)

func CheckDependencies() error {
	// hyprctl is checked because hyprsunset may be installed without Hyprland
	for _, bin := range []string{"hyprsunset", "hyprctl", "uwsm"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found in PATH", bin)
		}
	}

	return nil
}

func Notify(body string) error {
	return exec.Command(
		"notify-send",
		"-a", "hstui",
		"-u", "critical",
		"hstui",
		body,
	).Run()
}

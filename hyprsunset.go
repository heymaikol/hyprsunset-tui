package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// hyprsunset wraps the hyprsunset runtime IPC. It currently shells out to
// `hyprctl hyprsunset ...`; swap the body of hyprctl() for a direct socket
// client if you outgrow that. Add new commands here as typed funcs.

const hyprsunsetProcess = "hyprsunset"

func hyprctl(args ...string) error {
	return exec.Command("hyprctl", append([]string{"hyprsunset"}, args...)...).Run()
}

func SetTemperature(kelvin int) error { return hyprctl("temperature", strconv.Itoa(kelvin)) }
func SetGamma(percent int) error      { return hyprctl("gamma", strconv.Itoa(percent)) }

func startHyprsunset() ([]byte, error) {
	return exec.Command(
		"uwsm",
		"app",
		"-s", "b",
		"-t", "service",
		"-u", "hyprsunset.service",
		"--",
		"hyprsunset",
	).CombinedOutput()
}

func IsHyprsunsetRunning() (bool, error) {
	output, err := exec.Command("pgrep", "-x", hyprsunsetProcess).CombinedOutput()
	state := strings.TrimSpace(string(output))
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	if state != "" {
		return false, fmt.Errorf("%s: %w", state, err)
	}
	return false, err
}

func SetHyprsunsetRunning(enabled bool) error {
	if enabled {
		output, err := startHyprsunset()
		if err != nil {
			state := strings.TrimSpace(string(output))
			if state == "" {
				return err
			}
			return fmt.Errorf("%s: %w", state, err)
		}
		return nil
	}

	output, err := exec.Command("pkill", "-x", hyprsunsetProcess).CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		state := strings.TrimSpace(string(output))
		if state == "" {
			return err
		}
		return fmt.Errorf("%s: %w", state, err)
	}
	return nil
}

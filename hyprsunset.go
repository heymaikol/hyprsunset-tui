package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// hyprsunset wraps the hyprsunset runtime IPC; it currently shells out to
// `hyprctl hyprsunset ...`, swap the body of hyprctl() for a direct socket
// client if you outgrow that — add new commands here as typed funcs

// process name, as matched by pgrep/pkill -x
const hyprsunsetProcess = "hyprsunset"

// hyprctl runs `hyprctl hyprsunset <args>` and reports only success/failure
func hyprctl(args ...string) error {
	return exec.Command("hyprctl", append([]string{"hyprsunset"}, args...)...).Run()
}

// SetTemperature sets the colour temperature in Kelvin
func SetTemperature(kelvin int) error { return hyprctl("temperature", strconv.Itoa(kelvin)) }

// SetGamma sets the gamma as an integer percent (100 == 1.0)
func SetGamma(percent int) error { return hyprctl("gamma", strconv.Itoa(percent)) }

// startHyprsunset launches hyprsunset as a uwsm-managed systemd service so it
// outlives this process; returns combined output for error context
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

// IsHyprsunsetRunning reports whether the process is alive
func IsHyprsunsetRunning() (bool, error) {
	err := exec.Command("pgrep", "-x", hyprsunsetProcess).Run()
	// pgrep exits 0 if found, 1 if not, anything else is a real error
	switch exitErr, ok := err.(*exec.ExitError); {
	case err == nil:
		return true, nil
	case ok && exitErr.ExitCode() == 1:
		return false, nil
	default:
		return false, err
	}
}

// SetHyprsunsetRunning starts or stops hyprsunset; on failure it surfaces the
// command's output as the error message when there is any
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
		// pkill exit 1 means no process matched, already stopped — treat as success
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

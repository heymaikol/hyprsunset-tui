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

const hyprsunsetService = "hyprsunset.service"

func hyprctl(args ...string) error {
	return exec.Command("hyprctl", append([]string{"hyprsunset"}, args...)...).Run()
}

func SetTemperature(kelvin int) error { return hyprctl("temperature", strconv.Itoa(kelvin)) }
func SetGamma(percent int) error      { return hyprctl("gamma", strconv.Itoa(percent)) }

func systemctl(args ...string) ([]byte, error) {
	return exec.Command("systemctl", append([]string{"--user"}, args...)...).CombinedOutput()
}

func IsHyprsunsetRunning() (bool, error) {
	output, err := systemctl("is-active", hyprsunsetService)
	state := strings.TrimSpace(string(output))
	if state == "active" {
		return true, nil
	}
	if state == "inactive" || state == "failed" || state == "unknown" {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%s: %w", state, err)
	}
	return false, nil
}

func SetHyprsunsetRunning(enabled bool) error {
	action := "stop"
	if enabled {
		action = "start"
	}
	output, err := systemctl(action, hyprsunsetService)
	if err != nil {
		state := strings.TrimSpace(string(output))
		if state == "" {
			return err
		}
		return fmt.Errorf("%s: %w", state, err)
	}
	return nil
}

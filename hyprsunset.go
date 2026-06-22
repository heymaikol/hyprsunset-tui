package main

import (
	"os/exec"
	"strconv"
)

// hyprsunset wraps the hyprsunset runtime IPC. It currently shells out to
// `hyprctl hyprsunset ...`; swap the body of hyprctl() for a direct socket
// client if you outgrow that. Add new commands here as typed funcs.

func hyprctl(args ...string) error {
	return exec.Command("hyprctl", append([]string{"hyprsunset"}, args...)...).Run()
}

func SetTemperature(kelvin int) error { return hyprctl("temperature", strconv.Itoa(kelvin)) }
func SetGamma(percent int) error      { return hyprctl("gamma", strconv.Itoa(percent)) }
func Identity() error                 { return hyprctl("identity") }
func Reset() error                    { return hyprctl("reset") }

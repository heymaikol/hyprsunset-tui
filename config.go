package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const hyprsunsetConfigPath = "hypr/hyprsunset.conf"

type hyprsunsetProfile struct {
	time        string
	temperature int
	gamma       float32
	identity    bool
}

func defaultHyprsunsetProfile() hyprsunsetProfile {
	return hyprsunsetProfile{
		time:        "00:00",
		temperature: neutralTemp,
		gamma:       neutralGamma,
	}
}

func hyprsunsetConfigFile() (string, error) {
	configPath, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configPath, hyprsunsetConfigPath), nil
}

func loadHyprsunsetProfile() (hyprsunsetProfile, error) {
	path, err := hyprsunsetConfigFile()
	if err != nil {
		return hyprsunsetProfile{}, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return hyprsunsetProfile{}, err
	}

	return parseContent(content)
}

func saveHyprsunsetProfile(profile hyprsunsetProfile) error {
	path, err := hyprsunsetConfigFile()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	content, err := os.ReadFile(path)
	if err == nil {
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
		content = replaceLastProfileContent(content, formatHyprsunsetProfile(profile))
	} else if os.IsNotExist(err) {
		content = formatHyprsunsetProfile(profile)
	} else {
		return err
	}

	return os.WriteFile(path, content, mode)
}

func formatHyprsunsetProfile(profile hyprsunsetProfile) []byte {
	var b bytes.Buffer
	fmt.Fprintln(&b, "profile {")
	fmt.Fprintf(&b, "    time = %s\n", profile.time)
	fmt.Fprintf(&b, "    identity = %t\n", profile.identity)
	fmt.Fprintf(&b, "    temperature = %d\n", profile.temperature)
	fmt.Fprintf(&b, "    gamma = %.1f\n", profile.gamma)
	fmt.Fprintln(&b, "}")
	return b.Bytes()
}

func replaceLastProfileContent(content, profile []byte) []byte {
	lines := strings.SplitAfter(string(content), "\n")
	lastStart, lastEnd := -1, -1
	currentStart := -1

	for i, rawLine := range lines {
		line := strings.TrimSpace(strings.Split(rawLine, "#")[0])
		switch {
		case line == "profile {":
			currentStart = i
		case line == "}" && currentStart >= 0:
			lastStart, lastEnd = currentStart, i+1
			currentStart = -1
		}
	}

	if lastStart < 0 {
		if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
			content = append(content, '\n')
		}
		return append(content, profile...)
	}

	replaced := append([]string{}, lines[:lastStart]...)
	replaced = append(replaced, string(profile))
	replaced = append(replaced, lines[lastEnd:]...)
	return []byte(strings.Join(replaced, ""))
}

func parseContent(content []byte) (hyprsunsetProfile, error) {
	profile := defaultHyprsunsetProfile()
	inProfile := false

	for _, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(strings.Split(rawLine, "#")[0])

		switch {
		case line == "profile {":
			profile = defaultHyprsunsetProfile()
			inProfile = true
		case line == "}":
			inProfile = false
		case inProfile:
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)

			switch key {
			case "time":
				profile.time = value
			case "temperature":
				temperature, err := strconv.Atoi(value)
				if err != nil {
					return profile, fmt.Errorf("invalid temperature %q", value)
				}

				profile.temperature = temperature
			case "gamma":
				gamma, err := strconv.ParseFloat(value, 32)
				if err != nil {
					return profile, fmt.Errorf("invalid gamma %q", value)
				}

				profile.gamma = float32(gamma)
			case "identity":
				identity, err := strconv.ParseBool(value)
				if err != nil {
					return profile, fmt.Errorf("invalid identity %q", value)
				}

				profile.identity = identity
			}
		}
	}

	return profile, nil
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

func loadHyprsunsetProfile(now time.Time) (hyprsunsetProfile, error) {
	// Gets the configuration path
	configPath, err := os.UserConfigDir()
	if err != nil {
		return hyprsunsetProfile{}, err
	}

	// Makes the full path
	path := filepath.Join(configPath, hyprsunsetConfigPath)

	// Reads the file
	content, err := os.ReadFile(path)
	if err != nil {
		return hyprsunsetProfile{}, err
	}

	// Parse the content into a hyprsunsetProfile
	return parseContent(content)
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

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const hyprsunsetConfigPath = "hypr/hyprsunset.conf"

type hyprsunsetProfile struct {
	startMinutes int
	temperature  int
	gamma        int
	identity     bool
}

func defaultHyprsunsetProfile() hyprsunsetProfile {
	return hyprsunsetProfile{
		startMinutes: 0,
		temperature:  neutralTemp,
		gamma:        neutralGamma,
	}
}

func (p hyprsunsetProfile) uiTemperature() int {
	if p.identity {
		return neutralTemp
	}
	return p.temperature
}

func (p hyprsunsetProfile) statusText() string {
	if p.identity {
		return fmt.Sprintf("identity / %d%%", p.gamma)
	}
	return fmt.Sprintf("%dK / %d%%", p.temperature, p.gamma)
}

func currentHyprsunsetProfile(now time.Time) (hyprsunsetProfile, error) {
	configPath, err := os.UserConfigDir()
	if err != nil {
		return hyprsunsetProfile{}, err
	}

	path := filepath.Join(configPath, hyprsunsetConfigPath)
	return currentHyprsunsetProfileFromFile(path, now)
}

func currentHyprsunsetProfileFromFile(path string, now time.Time) (hyprsunsetProfile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultHyprsunsetProfile(), nil
		}
		return defaultHyprsunsetProfile(), err
	}
	return currentHyprsunsetProfileFromConfig(string(content), now)
}

func currentHyprsunsetProfileFromConfig(content string, now time.Time) (hyprsunsetProfile, error) {
	profiles, err := parseHyprsunsetProfiles(content)
	if err != nil {
		return defaultHyprsunsetProfile(), err
	}
	if len(profiles) == 0 {
		return defaultHyprsunsetProfile(), nil
	}

	nowMinutes := now.Hour()*60 + now.Minute()
	active := profiles[0]
	found := false
	for _, profile := range profiles {
		if profile.startMinutes <= nowMinutes && (!found || profile.startMinutes > active.startMinutes) {
			active = profile
			found = true
		}
	}
	if !found {
		active = profiles[0]
		for _, profile := range profiles[1:] {
			if profile.startMinutes > active.startMinutes {
				active = profile
			}
		}
	}

	return active, nil
}

func parseHyprsunsetProfiles(content string) ([]hyprsunsetProfile, error) {
	var profiles []hyprsunsetProfile
	var profile hyprsunsetProfile
	inProfile := false

	for lineNumber, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(stripComment(rawLine))
		if line == "" {
			continue
		}

		switch {
		case !inProfile && line == "profile {":
			profile = defaultHyprsunsetProfile()
			inProfile = true
		case inProfile && line == "}":
			profiles = append(profiles, profile)
			inProfile = false
		case inProfile:
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				return nil, fmt.Errorf("line %d: expected key = value", lineNumber+1)
			}
			if err := setProfileValue(&profile, strings.TrimSpace(key), strings.TrimSpace(value)); err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNumber+1, err)
			}
		}
	}

	if inProfile {
		return nil, fmt.Errorf("profile block is missing closing brace")
	}

	return profiles, nil
}

func setProfileValue(profile *hyprsunsetProfile, key, value string) error {
	switch key {
	case "time":
		minutes, err := parseProfileTime(value)
		if err != nil {
			return err
		}
		profile.startMinutes = minutes
	case "temperature":
		temperature, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid temperature %q", value)
		}
		profile.temperature = temperature
	case "gamma":
		gamma, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid gamma %q", value)
		}
		profile.gamma = int(math.Round(gamma * 100))
	case "identity":
		identity, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid identity %q", value)
		}
		profile.identity = identity
	}
	return nil
}

func parseProfileTime(value string) (int, error) {
	hourText, minuteText, ok := strings.Cut(value, ":")
	if !ok {
		return 0, fmt.Errorf("invalid time %q", value)
	}

	hour, err := strconv.Atoi(hourText)
	if err != nil || hour < 0 || hour > 23 {
		return 0, fmt.Errorf("invalid time %q", value)
	}

	minute, err := strconv.Atoi(minuteText)
	if err != nil || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("invalid time %q", value)
	}

	return hour*60 + minute, nil
}

func stripComment(line string) string {
	commentStart := strings.Index(line, "#")
	if commentStart == -1 {
		return line
	}
	return line[:commentStart]
}

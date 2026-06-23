package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// path to the config file, relative to the user config dir (~/.config)
const hyprsunsetConfigPath = "hypr/hyprsunset.conf"

// hyprsunsetProfile is one `profile { ... }` block from the config
type hyprsunsetProfile struct {
	time        string  // schedule time, "HH:MM"
	temperature int     // colour temperature in Kelvin
	gamma       float32 // gamma multiplier
	identity    bool    // hyprsunset identity flag
}

// defaultHyprsunsetProfile is the neutral fallback when nothing is on disk
func defaultHyprsunsetProfile() hyprsunsetProfile {
	return hyprsunsetProfile{
		time:        "00:00",
		temperature: neutralTemp,
		gamma:       neutralGamma,
	}
}

// hyprsunsetConfigFile returns the absolute path to the config file
func hyprsunsetConfigFile() (string, error) {
	configPath, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configPath, hyprsunsetConfigPath), nil
}

// loadHyprsunsetProfile reads and parses the on-disk profile
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

// saveHyprsunsetProfile writes the profile back, rewriting the last existing
// profile block in place and preserving the rest of the file
func saveHyprsunsetProfile(profile hyprsunsetProfile) error {
	path, err := hyprsunsetConfigFile()
	if err != nil {
		return err
	}

	// Ensure the parent dir exists (~/.config/hypr)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	mode := os.FileMode(0o644) // default perms for a freshly created file
	content, err := os.ReadFile(path)
	if err == nil {
		// File exists: keep its perms and swap the last profile block
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
		content = replaceLastProfileContent(content, formatHyprsunsetProfile(profile))
	} else if os.IsNotExist(err) {
		// No file yet: write a fresh profile
		content = formatHyprsunsetProfile(profile)
	} else {
		return err
	}

	return os.WriteFile(path, content, mode)
}

// formatHyprsunsetProfile renders a profile as a hyprsunset config block
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

// replaceLastProfileContent swaps the last `profile { ... }` block in content
// for profile, leaving everything else untouched; appends if none is found
func replaceLastProfileContent(content, profile []byte) []byte {
	// Keep newlines on each line so the file round-trips byte-for-byte
	lines := strings.SplitAfter(string(content), "\n")
	lastStart, lastEnd := -1, -1 // bounds of the last complete block found
	currentStart := -1           // start of the block currently being scanned

	// Find the last balanced profile block; comments (after #) are ignored
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

	// No block to replace: append, making sure there's a separating newline
	if lastStart < 0 {
		if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
			content = append(content, '\n')
		}
		return append(content, profile...)
	}

	// Splice the new block in place of the old one
	replaced := append([]string{}, lines[:lastStart]...)
	replaced = append(replaced, string(profile))
	replaced = append(replaced, lines[lastEnd:]...)
	return []byte(strings.Join(replaced, ""))
}

// parseContent extracts a profile from config bytes, returning the values of
// the last profile block; unknown keys are ignored, bad values are errors
func parseContent(content []byte) (hyprsunsetProfile, error) {
	profile := defaultHyprsunsetProfile()
	inProfile := false // are we inside a profile block right now

	for _, rawLine := range strings.Split(string(content), "\n") {
		// Strip trailing comments and surrounding whitespace
		line := strings.TrimSpace(strings.Split(rawLine, "#")[0])

		switch {
		case line == "profile {":
			// New block: reset so only the last block's values win
			profile = defaultHyprsunsetProfile()
			inProfile = true
		case line == "}":
			inProfile = false
		case inProfile:
			// Split "key = value"; skip lines without an =
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

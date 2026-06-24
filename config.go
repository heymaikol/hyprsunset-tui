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

// loadHyprsunsetProfiles reads and parses every on-disk profile; an empty or
// missing block list falls back to a single default profile
func loadHyprsunsetProfiles() ([]hyprsunsetProfile, error) {
	path, err := hyprsunsetConfigFile()
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	profiles, err := parseProfiles(content)
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return []hyprsunsetProfile{defaultHyprsunsetProfile()}, nil
	}
	return profiles, nil
}

// saveHyprsunsetProfiles writes the profiles back, replacing all existing
// profile blocks and preserving the file's surrounding content and perms
func saveHyprsunsetProfiles(profiles []hyprsunsetProfile) error {
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
		// File exists: keep its perms and swap the profile blocks
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
		content = replaceProfilesContent(content, profiles)
	} else if os.IsNotExist(err) {
		// No file yet: write fresh profile blocks
		content = formatHyprsunsetProfiles(profiles)
	} else {
		return err
	}

	return os.WriteFile(path, content, mode)
}

// formatHyprsunsetProfiles renders profiles as config blocks, blank-line separated
func formatHyprsunsetProfiles(profiles []hyprsunsetProfile) []byte {
	var b bytes.Buffer
	for i, profile := range profiles {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintln(&b, "profile {")
		fmt.Fprintf(&b, "    time = %s\n", profile.time)
		fmt.Fprintf(&b, "    identity = %t\n", profile.identity)
		fmt.Fprintf(&b, "    temperature = %d\n", profile.temperature)
		fmt.Fprintf(&b, "    gamma = %.1f\n", profile.gamma)
		fmt.Fprintln(&b, "}")
	}
	return b.Bytes()
}

func configLine(raw string) string {
	body, _, _ := strings.Cut(raw, "#")
	return strings.TrimSpace(body)
}

// replaceProfilesContent replaces the whole span of `profile { ... }` blocks in
// content with profiles, preserving the lines before the first block and after
// the last (a leading comment header, trailing global config); appends if none
// is found. Comments interleaved between blocks are dropped.
func replaceProfilesContent(content []byte, profiles []hyprsunsetProfile) []byte {
	// Keep newlines on each line so untouched regions round-trip byte-for-byte
	lines := strings.SplitAfter(string(content), "\n")
	firstStart, lastEnd := -1, -1 // span of all profile blocks
	currentStart := -1            // start of the block currently being scanned

	// Find the first block's start and the last block's end; comments are ignored
	for i, rawLine := range lines {
		line := configLine(rawLine)
		switch {
		case line == "profile {":
			currentStart = i
		case line == "}" && currentStart >= 0:
			if firstStart < 0 {
				firstStart = currentStart
			}
			lastEnd = i + 1
			currentStart = -1
		}
	}

	rendered := formatHyprsunsetProfiles(profiles)

	// No block to replace: append, making sure there's a separating newline
	if firstStart < 0 {
		if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
			content = append(content, '\n')
		}
		return append(content, rendered...)
	}

	// Splice the new blocks in place of the old span
	replaced := append([]string{}, lines[:firstStart]...)
	replaced = append(replaced, string(rendered))
	replaced = append(replaced, lines[lastEnd:]...)
	return []byte(strings.Join(replaced, ""))
}

// parseProfiles extracts every profile block from config bytes, in order;
// unknown keys are ignored, bad values are errors
func parseProfiles(content []byte) ([]hyprsunsetProfile, error) {
	var profiles []hyprsunsetProfile
	profile := defaultHyprsunsetProfile()
	inProfile := false // are we inside a profile block right now

	for _, rawLine := range strings.Split(string(content), "\n") {
		// Strip trailing comments and surrounding whitespace
		line := configLine(rawLine)

		switch {
		case line == "profile {":
			// New block: start from defaults so omitted keys keep neutral values
			profile = defaultHyprsunsetProfile()
			inProfile = true
		case line == "}" && inProfile:
			profiles = append(profiles, profile)
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
					return profiles, fmt.Errorf("invalid temperature %q", value)
				}

				profile.temperature = temperature
			case "gamma":
				gamma, err := strconv.ParseFloat(value, 32)
				if err != nil {
					return profiles, fmt.Errorf("invalid gamma %q", value)
				}

				profile.gamma = float32(gamma)
			case "identity":
				identity, err := strconv.ParseBool(value)
				if err != nil {
					return profiles, fmt.Errorf("invalid identity %q", value)
				}

				profile.identity = identity
			}
		}
	}

	return profiles, nil
}

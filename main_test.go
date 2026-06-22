package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClamp(t *testing.T) {
	if clamp(500, tempMin, tempMax) != tempMin {
		t.Fatal("below min not clamped")
	}
	if clamp(99999, tempMin, tempMax) != tempMax {
		t.Fatal("above max not clamped")
	}
	if clamp(4000, tempMin, tempMax) != 4000 {
		t.Fatal("in-range changed")
	}
}

func TestPresetKeysMapToIndex(t *testing.T) {
	for i, key := range []byte{'1', '2', '3'} {
		if int(key-'1') != i {
			t.Fatalf("preset key %c maps to wrong index", key)
		}
	}
	if presets[0].name != "Day" || presets[2].temp != 3000 {
		t.Fatal("preset table wrong")
	}
}

func TestCurrentHyprsunsetProfileFromConfig(t *testing.T) {
	config := `
max-gamma = 150

profile {
    time = 7:30
    identity = true
}

profile {
    time = 21:00
    temperature = 5500
    gamma = 0.8
}
`

	t.Run("uses latest profile before current time", func(t *testing.T) {
		profile, err := currentHyprsunsetProfileFromConfig(config, timeAt(20, 0))
		if err != nil {
			t.Fatalf("currentHyprsunsetProfileFromConfig() error = %v", err)
		}
		if !profile.identity {
			t.Fatal("profile.identity = false, want true")
		}
		if profile.uiTemperature() != neutralTemp || profile.gamma != neutralGamma {
			t.Fatalf("profile values = %dK / %d%%, want %dK / %d%%", profile.uiTemperature(), profile.gamma, neutralTemp, neutralGamma)
		}
	})

	t.Run("uses night profile", func(t *testing.T) {
		profile, err := currentHyprsunsetProfileFromConfig(config, timeAt(21, 30))
		if err != nil {
			t.Fatalf("currentHyprsunsetProfileFromConfig() error = %v", err)
		}
		if profile.identity {
			t.Fatal("profile.identity = true, want false")
		}
		if profile.uiTemperature() != 5500 || profile.gamma != 80 {
			t.Fatalf("profile values = %dK / %d%%, want 5500K / 80%%", profile.uiTemperature(), profile.gamma)
		}
	})

	t.Run("wraps to previous day's profile", func(t *testing.T) {
		profile, err := currentHyprsunsetProfileFromConfig(config, timeAt(6, 0))
		if err != nil {
			t.Fatalf("currentHyprsunsetProfileFromConfig() error = %v", err)
		}
		if profile.uiTemperature() != 5500 || profile.gamma != 80 {
			t.Fatalf("profile values = %dK / %d%%, want 5500K / 80%%", profile.uiTemperature(), profile.gamma)
		}
	})
}

func TestCurrentHyprsunsetProfileFromFile(t *testing.T) {
	t.Run("missing file uses defaults", func(t *testing.T) {
		profile, err := currentHyprsunsetProfileFromFile(filepath.Join(t.TempDir(), "hyprsunset.conf"), timeAt(12, 0))
		if err != nil {
			t.Fatalf("currentHyprsunsetProfileFromFile() error = %v", err)
		}
		if profile.uiTemperature() != neutralTemp || profile.gamma != neutralGamma {
			t.Fatalf("profile values = %dK / %d%%, want %dK / %d%%", profile.uiTemperature(), profile.gamma, neutralTemp, neutralGamma)
		}
	})

	t.Run("malformed file returns error", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "hyprsunset.conf")
		if err := os.WriteFile(configPath, []byte("profile {\n  time = nope\n}\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		if _, err := currentHyprsunsetProfileFromFile(configPath, timeAt(12, 0)); err == nil {
			t.Fatal("currentHyprsunsetProfileFromFile() error = nil, want malformed config error")
		}
	})
}

func TestResetCmd(t *testing.T) {
	t.Run("runs reset command", func(t *testing.T) {
		binDir := t.TempDir()
		argsFile := filepath.Join(t.TempDir(), "hyprctl-args")
		t.Setenv("PATH", binDir)
		t.Setenv("HYPRCTL_ARGS_FILE", argsFile)

		writeExecutable(t, binDir, "hyprctl", "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$HYPRCTL_ARGS_FILE\"\n")

		profile := hyprsunsetProfile{temperature: 5500, gamma: 80}
		msg := resetCmd(profile)()
		applied, ok := msg.(appliedMsg)
		if !ok {
			t.Fatalf("resetCmd() message = %T, want appliedMsg", msg)
		}
		if applied.isErr {
			t.Fatalf("resetCmd() isErr = true, want false: %s", applied.text)
		}
		if applied.text != "reset to config profile: 5500K / 80%" {
			t.Fatalf("resetCmd() text = %q, want config profile status", applied.text)
		}

		gotBytes, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("read hyprctl args: %v", err)
		}

		want := "hyprsunset\nreset\n"
		if got := string(gotBytes); got != want {
			t.Fatalf("hyprctl args = %q, want %q", got, want)
		}
	})

	t.Run("returns command error", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprctl", "#!/bin/sh\nexit 42\n")
		t.Setenv("PATH", binDir)

		msg := resetCmd(defaultHyprsunsetProfile())()
		applied, ok := msg.(appliedMsg)
		if !ok {
			t.Fatalf("resetCmd() message = %T, want appliedMsg", msg)
		}
		if !applied.isErr {
			t.Fatalf("resetCmd() isErr = false, want true")
		}
		if !strings.Contains(applied.text, "reset:") {
			t.Fatalf("resetCmd() text = %q, want reset error", applied.text)
		}
	})
}

func TestCheckDependencies(t *testing.T) {
	t.Run("all dependencies present", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprsunset", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, binDir, "hyprctl", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		if err := CheckDependencies(); err != nil {
			t.Fatalf("CheckDependencies() error = %v", err)
		}
	})

	t.Run("missing hyprsunset", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprctl", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		err := CheckDependencies()
		if err == nil {
			t.Fatal("CheckDependencies() error = nil, want missing hyprsunset error")
		}
		if !strings.Contains(err.Error(), "hyprsunset") {
			t.Fatalf("CheckDependencies() error = %q, want hyprsunset", err)
		}
	})

	t.Run("missing hyprctl", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprsunset", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		err := CheckDependencies()
		if err == nil {
			t.Fatal("CheckDependencies() error = nil, want missing hyprctl error")
		}
		if !strings.Contains(err.Error(), "hyprctl") {
			t.Fatalf("CheckDependencies() error = %q, want hyprctl", err)
		}
	})
}

func TestNotify(t *testing.T) {
	t.Run("missing notify-send", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		err := Notify("title", "body")
		if err == nil {
			t.Fatal("Notify() error = nil, want missing notify-send error")
		}
		if !strings.Contains(err.Error(), "notify-send") {
			t.Fatalf("Notify() error = %q, want notify-send", err)
		}
	})

	t.Run("sends expected arguments", func(t *testing.T) {
		binDir := t.TempDir()
		argsFile := filepath.Join(t.TempDir(), "notify-args")
		t.Setenv("PATH", binDir)
		t.Setenv("NOTIFY_ARGS_FILE", argsFile)

		writeExecutable(t, binDir, "notify-send", "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$NOTIFY_ARGS_FILE\"\n")

		if err := Notify("title", "body"); err != nil {
			t.Fatalf("Notify() error = %v", err)
		}

		gotBytes, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("read notify args: %v", err)
		}

		want := "-a\nhyprsunset-controller\n-u\ncritical\ntitle\nbody\n"
		if got := string(gotBytes); got != want {
			t.Fatalf("notify-send args = %q, want %q", got, want)
		}
	})

	t.Run("returns command error", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "notify-send", "#!/bin/sh\nexit 42\n")
		t.Setenv("PATH", binDir)

		if err := Notify("title", "body"); err == nil {
			t.Fatal("Notify() error = nil, want command error")
		}
	})
}

func writeExecutable(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", name, err)
	}
}

func timeAt(hour, minute int) time.Time {
	return time.Date(2026, 6, 21, hour, minute, 0, 0, time.Local)
}

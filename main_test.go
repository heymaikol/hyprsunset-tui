package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCursorAdjustsSelectedField(t *testing.T) {
	step := func(m model, k tea.KeyType) model {
		next, _ := m.Update(tea.KeyMsg{Type: k})
		return next.(model)
	}

	m := model{time: "12:00", identity: false, temp: 6000, gamma: 1.0}

	// cursor 0 = time
	m = step(m, tea.KeyLeft)
	if m.time != "11:45" {
		t.Fatalf("time = %q, want 11:45", m.time)
	}

	// down to identity, toggle
	m = step(step(m, tea.KeyDown), tea.KeyRight)
	if !m.identity {
		t.Fatal("identity = false, want toggled true")
	}

	// down to temperature, lower
	m = step(step(m, tea.KeyDown), tea.KeyLeft)
	if m.temp != 6000-tempStep {
		t.Fatalf("temp = %d, want %d", m.temp, 6000-tempStep)
	}

	if adjustTime("00:00", -timeStep) != "23:45" {
		t.Fatalf("wrap: got %q, want 23:45", adjustTime("00:00", -timeStep))
	}
}

func TestTabSwitchesBetweenPanels(t *testing.T) {
	step := func(m model, k tea.KeyType) model {
		next, _ := m.Update(tea.KeyMsg{Type: k})
		return next.(model)
	}

	m := model{time: "12:00", identity: false, temp: 6000, gamma: 1.0}

	m = step(m, tea.KeyTab)
	if m.focusedPanel != commonPanel {
		t.Fatalf("focusedPanel = %v, want commonPanel", m.focusedPanel)
	}

	m = step(m, tea.KeyLeft)
	if m.time != "12:00" {
		t.Fatalf("time changed while Common focused: got %q, want 12:00", m.time)
	}

	m = step(m, tea.KeyShiftTab)
	if m.focusedPanel != advancedPanel {
		t.Fatalf("focusedPanel = %v, want advancedPanel", m.focusedPanel)
	}

	m = step(m, tea.KeyLeft)
	if m.time != "11:45" {
		t.Fatalf("time = %q, want 11:45", m.time)
	}
}

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

func TestViewShowsConfigurationFields(t *testing.T) {
	view := model{
		temp:     6000,
		gamma:    1.0,
		time:     "07:00",
		identity: true,
		enabled:  true,
	}.View()

	if !strings.Contains(view, "Configuration") {
		t.Fatalf("View() = %q, want configuration title", view)
	}
	if !strings.Contains(view, "Time:") || !strings.Contains(view, "07:00") {
		t.Fatalf("View() = %q, want configuration time", view)
	}
	if !strings.Contains(view, "Identity:") || !strings.Contains(view, "true") {
		t.Fatalf("View() = %q, want identity", view)
	}
	if !strings.Contains(view, "[x] Enabled") {
		t.Fatalf("View() = %q, want enabled checkbox", view)
	}
}

func TestEnabledCheckboxTogglesHyprsunsetService(t *testing.T) {
	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "systemctl-args")
	t.Setenv("PATH", binDir)
	t.Setenv("SYSTEMCTL_ARGS_FILE", argsFile)
	writeExecutable(t, binDir, "systemctl", "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SYSTEMCTL_ARGS_FILE\"\nexit 0\n")

	m := model{focusedPanel: commonPanel}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("Update(space) cmd = nil, want systemctl command")
	}

	msg := cmd()
	next, _ = next.(model).Update(msg)
	got := next.(model)
	if !got.enabled {
		t.Fatal("enabled = false, want true")
	}

	gotBytes, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read systemctl args: %v", err)
	}
	want := "--user\nstart\nhyprsunset.service\n"
	if string(gotBytes) != want {
		t.Fatalf("systemctl args = %q, want %q", gotBytes, want)
	}
}

func TestEnterAppliesAndADoesNot(t *testing.T) {
	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "hyprctl-args")
	t.Setenv("PATH", binDir)
	t.Setenv("HYPRCTL_ARGS_FILE", argsFile)
	writeExecutable(t, binDir, "hyprctl", "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$HYPRCTL_ARGS_FILE\"\nexit 0\n")

	m := model{temp: 4500, gamma: 0.8}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil {
		t.Fatal("Update(a) cmd != nil, want nil")
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Update(enter) cmd = nil, want hyprctl command")
	}
	if msg := cmd(); msg != (statusMsg{text: "applied 4500K / 0.8", isErr: false}) {
		t.Fatalf("enter command msg = %#v, want successful statusMsg", msg)
	}

	gotBytes, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read hyprctl args: %v", err)
	}
	want := "hyprsunset\ntemperature\n4500\nhyprsunset\ngamma\n80\n"
	if string(gotBytes) != want {
		t.Fatalf("hyprctl args = %q, want %q", gotBytes, want)
	}
}

func TestSaveHyprsunsetProfileWritesConfig(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	configPath := filepath.Join(configDir, hyprsunsetConfigPath)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	existing := []byte(`# keep this comment
profile {
    time = 07:00
    temperature = 6500
    gamma = 1.0
}

profile {
    time = 19:00
    temperature = 4500
    gamma = 0.8
}
`)
	if err := os.WriteFile(configPath, existing, 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	want := hyprsunsetProfile{
		time:        "20:30",
		identity:    true,
		temperature: 4250,
		gamma:       0.7,
	}
	if err := saveHyprsunsetProfile(want); err != nil {
		t.Fatalf("saveHyprsunsetProfile() error = %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(content), "time = 07:00") {
		t.Fatalf("saved config = %q, want first profile preserved", content)
	}
	if count := strings.Count(string(content), "profile {"); count != 2 {
		t.Fatalf("profile count = %d, want 2", count)
	}
	got, err := parseContent(content)
	if err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if got != want {
		t.Fatalf("saved profile = %+v, want %+v", got, want)
	}
	if info, err := os.Stat(configPath); err != nil {
		t.Fatalf("stat saved config: %v", err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("saved config mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestSKeySavesCurrentConfiguration(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	m := model{
		time:     "06:15",
		identity: false,
		temp:     5000,
		gamma:    0.9,
		saved: hyprsunsetProfile{
			time:        "00:00",
			temperature: neutralTemp,
			gamma:       neutralGamma,
		},
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("Update(s) cmd = nil, want save command")
	}

	msg := cmd()
	next, _ = next.(model).Update(msg)
	got := next.(model)
	want := hyprsunsetProfile{time: "06:15", temperature: 5000, gamma: 0.9}
	if got.status != "saved configuration" || got.statusErr {
		t.Fatalf("status = %q / %t, want saved configuration / false", got.status, got.statusErr)
	}
	if got.saved != want {
		t.Fatalf("saved baseline = %+v, want %+v", got.saved, want)
	}

	content, err := os.ReadFile(filepath.Join(configDir, hyprsunsetConfigPath))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	profile, err := parseContent(content)
	if err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if profile != want {
		t.Fatalf("file profile = %+v, want %+v", profile, want)
	}
}

func TestParseContent(t *testing.T) {
	t.Run("identity profile keeps default visible values", func(t *testing.T) {
		config := `
profile {
    time = 07:00
    identity = true
}
`

		profile, err := parseContent([]byte(config))
		if err != nil {
			t.Fatalf("parseContent() error = %v", err)
		}
		if profile.time != "07:00" {
			t.Fatalf("profile.time = %q, want 07:00", profile.time)
		}
		if !profile.identity {
			t.Fatal("profile.identity = false, want true")
		}
		if profile.temperature != neutralTemp || profile.gamma != neutralGamma {
			t.Fatalf("profile values = %dK / %.1f, want %dK / %.1f", profile.temperature, profile.gamma, neutralTemp, neutralGamma)
		}
	})

	t.Run("parses temperature and gamma", func(t *testing.T) {
		config := `
profile {
    time = 21:00
    temperature = 5500
    gamma = 0.8
}
`

		profile, err := parseContent([]byte(config))
		if err != nil {
			t.Fatalf("parseContent() error = %v", err)
		}
		if profile.time != "21:00" || profile.temperature != 5500 || profile.gamma != 0.8 || profile.identity {
			t.Fatalf("profile = %+v, want 21:00 / 5500K / 0.8 / identity false", profile)
		}
	})

	t.Run("starts each profile from defaults", func(t *testing.T) {
		config := `
profile {
    time = 07:00
    gamma = 0.5
}

profile {
    time = 21:00
    temperature = 4000
}
`

		profile, err := parseContent([]byte(config))
		if err != nil {
			t.Fatalf("parseContent() error = %v", err)
		}
		if profile.time != "21:00" || profile.temperature != 4000 || profile.gamma != neutralGamma {
			t.Fatalf("profile = %+v, want 21:00 / 4000K / %.1f", profile, neutralGamma)
		}
	})

	t.Run("returns invalid gamma error", func(t *testing.T) {
		config := `
profile {
    gamma = nope
}
`

		if _, err := parseContent([]byte(config)); err == nil {
			t.Fatal("parseContent() error = nil, want invalid gamma error")
		}
	})
}

func TestCheckDependencies(t *testing.T) {
	t.Run("all dependencies present", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprsunset", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, binDir, "hyprctl", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, binDir, "systemctl", "#!/bin/sh\nexit 0\n")
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

	t.Run("missing systemctl", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprsunset", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, binDir, "hyprctl", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		err := CheckDependencies()
		if err == nil {
			t.Fatal("CheckDependencies() error = nil, want missing systemctl error")
		}
		if !strings.Contains(err.Error(), "systemctl") {
			t.Fatalf("CheckDependencies() error = %q, want systemctl", err)
		}
	})
}

func TestIsHyprsunsetRunning(t *testing.T) {
	t.Run("active", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "systemctl", "#!/bin/sh\necho active\nexit 0\n")
		t.Setenv("PATH", binDir)

		running, err := IsHyprsunsetRunning()
		if err != nil {
			t.Fatalf("IsHyprsunsetRunning() error = %v", err)
		}
		if !running {
			t.Fatal("IsHyprsunsetRunning() = false, want true")
		}
	})

	t.Run("inactive", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "systemctl", "#!/bin/sh\necho inactive\nexit 3\n")
		t.Setenv("PATH", binDir)

		running, err := IsHyprsunsetRunning()
		if err != nil {
			t.Fatalf("IsHyprsunsetRunning() error = %v", err)
		}
		if running {
			t.Fatal("IsHyprsunsetRunning() = true, want false")
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

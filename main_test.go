package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCursorAdjustsSelectedField(t *testing.T) {
	step := func(m model, k tea.KeyType) model {
		next, _ := m.Update(tea.KeyMsg{Type: k})
		return next.(model)
	}

	m := model{focusAdvanced: true, profiles: []hyprsunsetProfile{{time: "12:00", identity: false, temperature: 6000, gamma: 1.0}}}

	// cursor 0 = Profile selector; down to Time
	m = step(step(m, tea.KeyDown), tea.KeyLeft)
	if m.current().time != "11:45" {
		t.Fatalf("time = %q, want 11:45", m.current().time)
	}

	// down to identity, toggle
	m = step(step(m, tea.KeyDown), tea.KeyRight)
	if !m.current().identity {
		t.Fatal("identity = false, want toggled true")
	}

	// down to temperature, lower
	m = step(step(m, tea.KeyDown), tea.KeyLeft)
	if m.current().temperature != 6000-tempStep {
		t.Fatalf("temp = %d, want %d", m.current().temperature, 6000-tempStep)
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

	m := model{profiles: []hyprsunsetProfile{{time: "12:00", identity: false, temperature: 6000, gamma: 1.0}}}

	if m.focusAdvanced {
		t.Fatal("focusAdvanced = true, want false")
	}

	m = step(m, tea.KeyLeft)
	if m.current().time != "12:00" {
		t.Fatalf("time changed while Simple focused: got %q, want 12:00", m.current().time)
	}

	m = step(m, tea.KeyTab)
	if !m.focusAdvanced {
		t.Fatal("focusAdvanced = false, want true")
	}

	// cursor 0 = Profile selector; down to Time, then adjust
	m = step(step(m, tea.KeyDown), tea.KeyLeft)
	if m.current().time != "11:45" {
		t.Fatalf("time = %q, want 11:45", m.current().time)
	}

	m = step(m, tea.KeyShiftTab)
	if m.focusAdvanced {
		t.Fatal("focusAdvanced = true, want false")
	}
}

func TestInitialModelStartsOnSimplePanel(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeExecutable(t, binDir, "pgrep", "#!/bin/sh\nexit 1\n")

	m := initialModel()
	if m.focusAdvanced {
		t.Fatal("focusAdvanced = true, want false")
	}
}

func TestProfileAddDeleteKeys(t *testing.T) {
	step := func(m model, r rune) model {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		return next.(model)
	}

	m := model{
		focusAdvanced: true,
		profiles:      []hyprsunsetProfile{{time: "07:00", temperature: 6000, gamma: 1.0}},
	}

	// 'n' appends a default profile and selects it
	m = step(m, 'n')
	if len(m.profiles) != 2 || m.selected != 1 {
		t.Fatalf("after n: len=%d selected=%d, want 2/1", len(m.profiles), m.selected)
	}

	// 'd' removes the selected profile and clamps the selection
	m = step(m, 'd')
	if len(m.profiles) != 1 || m.selected != 0 {
		t.Fatalf("after d: len=%d selected=%d, want 1/0", len(m.profiles), m.selected)
	}

	// 'd' refuses to delete the last remaining profile
	m = step(m, 'd')
	if len(m.profiles) != 1 {
		t.Fatalf("after d on last: len=%d, want 1", len(m.profiles))
	}
	if !m.statusErr {
		t.Fatal("deleting last profile should set an error status")
	}

	// 'n' is ignored when the Simple panel is focused
	common := model{profiles: []hyprsunsetProfile{{}}}
	if got := step(common, 'n'); len(got.profiles) != 1 {
		t.Fatalf("n on Simple panel: len=%d, want 1", len(got.profiles))
	}
}

func TestSimpleDayNightControl(t *testing.T) {
	key := func(m model, k tea.KeyType) model {
		next, _ := m.Update(tea.KeyMsg{Type: k})
		return next.(model)
	}

	// A single profile shows only the Day row (Enabled + 3 Day cells)
	one := model{profiles: []hyprsunsetProfile{{time: "07:00", temperature: 6000, gamma: 1.0}}}
	if got := one.simpleCellCount(); got != 1+len(simpleProfileFields) {
		t.Fatalf("one-profile cells = %d, want %d", got, 1+len(simpleProfileFields))
	}

	// With Day + Night, navigate to the Night temperature and lower it
	m := model{profiles: []hyprsunsetProfile{
		{time: "07:00", temperature: 6000, gamma: 1.0},
		{time: "20:00", temperature: 4000, gamma: 0.9},
	}}
	// Cells: 0=Enabled, 1=DayTime, 2=DayTemp, 3=DayGamma, 4=NightTime, 5=NightTemp...
	for i := 0; i < 5; i++ {
		m = key(m, tea.KeyDown)
	}
	m = key(m, tea.KeyLeft) // lower Night temperature
	if m.profiles[1].temperature != 4000-tempStep {
		t.Fatalf("night temp = %d, want %d", m.profiles[1].temperature, 4000-tempStep)
	}
	if m.profiles[0].temperature != 6000 {
		t.Fatalf("day temp = %d, want 6000 (untouched)", m.profiles[0].temperature)
	}

	// Down stops at the last cell
	for i := 0; i < 10; i++ {
		m = key(m, tea.KeyDown)
	}
	if m.simpleCursor != m.simpleCellCount()-1 {
		t.Fatalf("cursor = %d, want clamped to %d", m.simpleCursor, m.simpleCellCount()-1)
	}
}

func TestViewShowsDayNightRows(t *testing.T) {
	// One profile: Simple shows only the Day row, no Night
	one := model{profiles: []hyprsunsetProfile{{time: "07:00", temperature: 6000, gamma: 1.0}}}.View()
	if !strings.Contains(one, "Day") {
		t.Fatalf("View() missing Day row: %q", one)
	}
	if strings.Contains(one, "Night") {
		t.Fatalf("View() shows Night with a single profile: %q", one)
	}

	// Two profiles: both rows show
	two := model{profiles: []hyprsunsetProfile{
		{time: "07:00", temperature: 6000, gamma: 1.0},
		{time: "20:00", temperature: 4000, gamma: 0.9},
	}}.View()
	if !strings.Contains(two, "Day") || !strings.Contains(two, "Night") {
		t.Fatalf("View() missing Day/Night rows: %q", two)
	}
}

func TestConfigurationUsesDayNightLabels(t *testing.T) {
	three := []hyprsunsetProfile{
		{time: "07:00", temperature: 6000, gamma: 1.0},
		{time: "20:00", temperature: 4000, gamma: 0.9},
		{time: "23:00", temperature: 3000, gamma: 0.8},
	}
	v := model{profiles: three}.View()
	// Day/Night replace the numbered labels; extras stay numbered
	if strings.Contains(v, "Profile 1") || strings.Contains(v, "Profile 2") {
		t.Fatalf("Configuration still uses numbered Day/Night labels: %q", v)
	}
	if !strings.Contains(v, "Profile 3") {
		t.Fatalf("Configuration missing numbered label for extra profile: %q", v)
	}
}

func TestBackspaceClearsAndReaddsField(t *testing.T) {
	step := func(m model, k tea.KeyType) model {
		next, _ := m.Update(tea.KeyMsg{Type: k})
		return next.(model)
	}

	m := model{
		focusAdvanced: true,
		profiles:      []hyprsunsetProfile{{time: "12:00", temperature: 6000, gamma: 1.0}},
	}

	// Profile=0, Time=1, Identity=2, Temperature=3
	m = step(step(step(m, tea.KeyDown), tea.KeyDown), tea.KeyDown)

	// Backspace clears the attribute; it renders as the unset marker
	m = step(m, tea.KeyBackspace)
	if m.current().isSet(temperatureBit) {
		t.Fatal("temperature still set after backspace")
	}
	if got := fields[m.cursor].render(m); got != unsetMark {
		t.Fatalf("render after clear = %q, want %q", got, unsetMark)
	}

	// ←/→ re-adds it (the mistake is recoverable)
	m = step(m, tea.KeyRight)
	if !m.current().isSet(temperatureBit) {
		t.Fatal("temperature not re-added after adjust")
	}

	// Profile selector (bit 0) is not clearable
	top := model{focusAdvanced: true, profiles: []hyprsunsetProfile{{}, {}}}
	if got := step(top, tea.KeyBackspace); got.current().unset != 0 {
		t.Fatalf("backspace on Profile selector set unset = %b, want 0", got.current().unset)
	}
}

func TestUnsetFieldOmittedFromConfig(t *testing.T) {
	out := string(formatHyprsunsetProfiles([]hyprsunsetProfile{
		{time: "07:00", temperature: 6000, gamma: 1.0, unset: temperatureBit},
	}))
	if strings.Contains(out, "temperature") {
		t.Fatalf("cleared temperature still in config: %q", out)
	}
	if !strings.Contains(out, "time = 07:00") || !strings.Contains(out, "gamma = 1.0") {
		t.Fatalf("present fields missing from config: %q", out)
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
		profiles: []hyprsunsetProfile{{time: "07:00", identity: true, temperature: 6000, gamma: 1.0}},
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

func TestViewMaxGammaOnlyWhenRaised(t *testing.T) {
	base := model{profiles: []hyprsunsetProfile{{time: "07:00", temperature: 6000, gamma: 1.0}}}

	// At default only the Advanced editor row shows it, not the diff box
	def := base
	def.maxGamma, def.savedMaxGamma = defaultMaxGamma, defaultMaxGamma
	if n := strings.Count(def.View(), "Max Gamma"); n != 1 {
		t.Fatalf("Max Gamma count at default = %d, want 1 (Advanced row only)", n)
	}

	// Raised and changed from disk: the diff box adds the old → new line
	raised := base
	raised.maxGamma, raised.savedMaxGamma = 150, defaultMaxGamma
	view := raised.View()
	if n := strings.Count(view, "Max Gamma"); n != 2 {
		t.Fatalf("Max Gamma count when raised = %d, want 2 (Advanced + diff box)", n)
	}
	if !strings.Contains(view, "100%") || !strings.Contains(view, "→") {
		t.Fatalf("View() = %q, want Max Gamma 100%% → 150%% diff", view)
	}
}

func TestSpaceTogglesSimpleCheckbox(t *testing.T) {
	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "uwsm-args")
	pkillArgs := filepath.Join(t.TempDir(), "pkill-args")
	t.Setenv("PATH", binDir)
	t.Setenv("UWSM_ARGS_FILE", argsFile)
	t.Setenv("PKILL_ARGS_FILE", pkillArgs)
	writeExecutable(t, binDir, "uwsm", "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$UWSM_ARGS_FILE\"\nexit 0\n")
	writeExecutable(t, binDir, "pkill", "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$PKILL_ARGS_FILE\"\nexit 0\n")

	m := model{}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if cmd == nil {
		t.Fatal("Update(space) cmd = nil, want toggle command")
	}

	msg := cmd()
	next, _ = next.(model).Update(msg)
	got := next.(model)
	if !got.enabled {
		t.Fatal("enabled = false after space, want true")
	}

	next, cmd = got.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if cmd == nil {
		t.Fatal("Update(space) cmd = nil while enabled, want toggle command")
	}

	msg = cmd()
	next, _ = next.(model).Update(msg)
	got = next.(model)
	if got.enabled {
		t.Fatal("enabled = true after second space, want false")
	}

	gotBytes, err := os.ReadFile(pkillArgs)
	if err != nil {
		t.Fatalf("read pkill args: %v", err)
	}
	want := "-x\nhyprsunset\n"
	if string(gotBytes) != want {
		t.Fatalf("pkill args = %q, want %q", gotBytes, want)
	}
}

func TestSaveHyprsunsetProfilesWritesConfig(t *testing.T) {
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

# night notes stay with the profile list
profile {
    time = 22:00
    temperature = 3500
    gamma = 0.6
}
`)
	if err := os.WriteFile(configPath, existing, 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	want := []hyprsunsetProfile{
		{time: "07:00", temperature: 6500, gamma: 1.0},
		{time: "20:30", identity: true, temperature: 4250, gamma: 0.7},
	}
	if err := saveHyprsunsetProfiles(want, defaultMaxGamma); err != nil {
		t.Fatalf("saveHyprsunsetProfiles() error = %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(content), "# keep this comment") {
		t.Fatalf("saved config = %q, want leading comment preserved", content)
	}
	leading := strings.Index(string(content), "# keep this comment")
	interleaved := strings.Index(string(content), "# night notes stay with the profile list")
	if interleaved < 0 {
		t.Fatalf("saved config = %q, want interleaved comment preserved", content)
	}
	if count := strings.Count(string(content), "profile {"); count != 2 {
		t.Fatalf("profile count = %d, want 2", count)
	}
	secondProfile := strings.LastIndex(string(content), "profile {")
	if !(leading < interleaved && interleaved < secondProfile) {
		t.Fatalf("saved config = %q, want interleaved comment between profile blocks", content)
	}
	got, err := parseProfiles(content)
	if err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("saved profiles = %+v, want %+v", got, want)
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
		profiles: []hyprsunsetProfile{{time: "06:15", identity: false, temperature: 5000, gamma: 0.9}},
		saved:    []hyprsunsetProfile{{time: "00:00", temperature: neutralTemp, gamma: neutralGamma}},
		maxGamma: defaultMaxGamma,
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("Update(s) cmd = nil, want save command")
	}

	msg := cmd()
	next, _ = next.(model).Update(msg)
	got := next.(model)
	want := []hyprsunsetProfile{{time: "06:15", temperature: 5000, gamma: 0.9}}
	if got.status != "saved configuration" || got.statusErr {
		t.Fatalf("status = %q / %t, want saved configuration / false", got.status, got.statusErr)
	}
	if !reflect.DeepEqual(got.saved, want) {
		t.Fatalf("saved baseline = %+v, want %+v", got.saved, want)
	}

	content, err := os.ReadFile(filepath.Join(configDir, hyprsunsetConfigPath))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	profiles, err := parseProfiles(content)
	if err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if !reflect.DeepEqual(profiles, want) {
		t.Fatalf("file profiles = %+v, want %+v", profiles, want)
	}
}

func TestParseProfiles(t *testing.T) {
	only := func(t *testing.T, config string) hyprsunsetProfile {
		t.Helper()
		profiles, err := parseProfiles([]byte(config))
		if err != nil {
			t.Fatalf("parseProfiles() error = %v", err)
		}
		if len(profiles) != 1 {
			t.Fatalf("parseProfiles() len = %d, want 1", len(profiles))
		}
		return profiles[0]
	}

	t.Run("identity profile keeps default visible values", func(t *testing.T) {
		profile := only(t, `
profile {
    time = 07:00
    identity = true
}
`)
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
		profile := only(t, `
profile {
    time = 21:00
    temperature = 5500
    gamma = 0.8
}
`)
		if profile.time != "21:00" || profile.temperature != 5500 || profile.gamma != 0.8 || profile.identity {
			t.Fatalf("profile = %+v, want 21:00 / 5500K / 0.8 / identity false", profile)
		}
	})

	t.Run("returns every block in order", func(t *testing.T) {
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
		profiles, err := parseProfiles([]byte(config))
		if err != nil {
			t.Fatalf("parseProfiles() error = %v", err)
		}
		want := []hyprsunsetProfile{
			{time: "07:00", temperature: neutralTemp, gamma: 0.5, unset: temperatureBit | identityBit},
			{time: "21:00", temperature: 4000, gamma: neutralGamma, unset: gammaBit | identityBit},
		}
		if !reflect.DeepEqual(profiles, want) {
			t.Fatalf("profiles = %+v, want %+v", profiles, want)
		}
	})

	t.Run("returns invalid gamma error", func(t *testing.T) {
		config := `
profile {
    gamma = nope
}
`

		if _, err := parseProfiles([]byte(config)); err == nil {
			t.Fatal("parseProfiles() error = nil, want invalid gamma error")
		}
	})
}

func TestMaxGammaConfig(t *testing.T) {
	if got := parseMaxGamma([]byte("max-gamma = 150 # cap\nprofile {\n}\n")); got != 150 {
		t.Fatalf("parseMaxGamma = %d, want 150", got)
	}
	if got := parseMaxGamma([]byte("profile {\n}\n")); got != defaultMaxGamma {
		t.Fatalf("parseMaxGamma (absent) = %d, want %d", got, defaultMaxGamma)
	}

	// Insert at top when non-default and absent
	if out := string(setMaxGammaLine([]byte("profile {\n}\n"), 150)); !strings.HasPrefix(out, "max-gamma = 150\n") {
		t.Fatalf("insert: %q, want max-gamma prefix", out)
	}

	// Replace existing line in place, no duplicate
	out := string(setMaxGammaLine([]byte("max-gamma = 100\nprofile {\n}\n"), 200))
	if !strings.Contains(out, "max-gamma = 200\n") || strings.Count(out, "max-gamma") != 1 {
		t.Fatalf("replace: %q, want single max-gamma = 200", out)
	}

	// Default value, absent line: leave content untouched
	in := "profile {\n}\n"
	if out := string(setMaxGammaLine([]byte(in), defaultMaxGamma)); out != in {
		t.Fatalf("default insert changed content: %q", out)
	}

	// Default value, existing line: replace it in place without separator cleanup
	want := "max-gamma = 100\n\nprofile {\n}\n"
	if out := string(setMaxGammaLine([]byte("max-gamma = 150\n\nprofile {\n}\n"), defaultMaxGamma)); out != want {
		t.Fatalf("default replace: %q, want %q", out, want)
	}
}

func TestCheckDependencies(t *testing.T) {
	t.Run("all dependencies present", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprsunset", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, binDir, "uwsm", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, binDir, "notify-send", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		if err := CheckDependencies(); err != nil {
			t.Fatalf("CheckDependencies() error = %v", err)
		}
	})

	t.Run("missing hyprsunset", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "uwsm", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, binDir, "notify-send", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		err := CheckDependencies()
		if err == nil {
			t.Fatal("CheckDependencies() error = nil, want missing hyprsunset error")
		}
		if !strings.Contains(err.Error(), "hyprsunset") {
			t.Fatalf("CheckDependencies() error = %q, want hyprsunset", err)
		}
	})

	t.Run("missing uwsm", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprsunset", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, binDir, "notify-send", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		err := CheckDependencies()
		if err == nil {
			t.Fatal("CheckDependencies() error = nil, want missing uwsm error")
		}
		if !strings.Contains(err.Error(), "uwsm") {
			t.Fatalf("CheckDependencies() error = %q, want uwsm", err)
		}
	})

	t.Run("missing notify-send", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprsunset", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, binDir, "uwsm", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		err := CheckDependencies()
		if err == nil {
			t.Fatal("CheckDependencies() error = nil, want missing notify-send error")
		}
		if !strings.Contains(err.Error(), "notify-send") {
			t.Fatalf("CheckDependencies() error = %q, want notify-send", err)
		}
	})
}

func TestIsHyprsunsetRunning(t *testing.T) {
	t.Run("active", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "pgrep", "#!/bin/sh\necho 1234\nexit 0\n")
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
		writeExecutable(t, binDir, "pgrep", "#!/bin/sh\nexit 1\n")
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

		err := Notify("body")
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

		if err := Notify("body"); err != nil {
			t.Fatalf("Notify() error = %v", err)
		}

		gotBytes, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("read notify args: %v", err)
		}

		want := "-a\nhstui\n-u\ncritical\nhstui\nbody\n"
		if got := string(gotBytes); got != want {
			t.Fatalf("notify-send args = %q, want %q", got, want)
		}
	})

	t.Run("returns command error", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "notify-send", "#!/bin/sh\nexit 42\n")
		t.Setenv("PATH", binDir)

		if err := Notify("body"); err == nil {
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

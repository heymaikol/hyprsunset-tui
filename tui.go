package main

import (
	"cmp"
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	tempStep                 = 250         // Kelvin per left/right press on Temperature
	gammaStep        float32 = 0.1         // gamma units per press
	timeStep                 = 15          // minutes per press on Time
	tempMin, tempMax         = 1000, 10000 // clamp range for temperature (K)
	gammaMin         float32 = 0.1         // lowest allowed gamma
	gammaMax         float32 = 2.0         // highest allowed gamma
	neutralTemp              = 6000        // daylight-neutral temperature
	neutralGamma     float32 = 1.0         // unadjusted gamma
)

// Returns lo if v is below it
// Returns hi if v is above it
func clamp[T cmp.Ordered](v, lo, hi T) T {
	return max(lo, min(hi, v))
}

// model is the full TUI state
type model struct {
	profiles     []hyprsunsetProfile // all profiles, editable in the Advanced panel
	selected     int                 // index of the profile being edited
	cursor       int                 // selected row in the Advanced panel
	focusedPanel panel               // which panel has focus
	enabled      bool                // is hyprsunset currently running
	status       string              // status line text
	statusErr    bool                // render status as an error
	saved        []hyprsunsetProfile // on-disk profiles, for the diff box
}

// current returns a pointer to the profile being edited
func (m *model) current() *hyprsunsetProfile { return &m.profiles[m.selected] }

// cloneProfiles copies a profile slice; profiles are flat value structs, so a
// fresh backing array fully detaches it (used for the diff baseline)
func cloneProfiles(profiles []hyprsunsetProfile) []hyprsunsetProfile {
	return append([]hyprsunsetProfile(nil), profiles...)
}

// panel identifies a focusable region of the UI
type panel int

const (
	advancedPanel panel = iota // editable field list
	commonPanel                // simple enable/disable toggle
)

// initialModel builds the starting state from the on-disk profiles and the
// live hyprsunset status, falling back to defaults on error
func initialModel() model {
	// Load the saved profiles; use a single default if they can't be read
	profiles, err := loadHyprsunsetProfiles()
	if err != nil {
		profiles = []hyprsunsetProfile{defaultHyprsunsetProfile()}
	}
	// Seed the model from the profiles, start focused on the Simple panel
	m := model{
		profiles:     profiles,
		focusedPanel: commonPanel,
		saved:        cloneProfiles(profiles),
	}
	// Surface a load failure in the status line
	if err != nil {
		m.status = "config: " + err.Error()
		m.statusErr = true
	}
	// Reflect whether hyprsunset is actually running right now
	enabled, err := IsHyprsunsetRunning()
	if err != nil {
		m.status = "uwsm: " + err.Error()
		m.statusErr = true
	} else {
		m.enabled = enabled
	}
	return m
}

// Init has no startup command (Bubble Tea entry point)
func (m model) Init() tea.Cmd { return nil }

// statusMsg carries the result of an async operation back into Update
type statusMsg struct {
	text    string
	isErr   bool
	enabled *bool
	saved   []hyprsunsetProfile
}

// setEnabledCmd starts or stops the hyprsunset daemon and reports the new state.
// The daemon reads the saved config and applies the time-matching profile, so no
// values are pushed here.
func setEnabledCmd(enabled bool) tea.Cmd {
	return func() tea.Msg {
		if err := SetHyprsunsetRunning(enabled); err != nil {
			return statusMsg{text: "enabled: " + err.Error(), isErr: true}
		}
		// Human-readable label for the status line
		state := "disabled"
		if enabled {
			state = "enabled"
		}
		return statusMsg{text: "hyprsunset " + state, isErr: false, enabled: &enabled}
	}
}

// saveConfigCmd writes the profiles to disk and updates the diff baseline
func saveConfigCmd(profiles []hyprsunsetProfile) tea.Cmd {
	saved := cloneProfiles(profiles)
	return func() tea.Msg {
		if err := saveHyprsunsetProfiles(saved); err != nil {
			return statusMsg{text: "save: " + err.Error(), isErr: true}
		}
		return statusMsg{text: "saved configuration", isErr: false, saved: saved}
	}
}

// Update handles incoming messages: async results and key presses
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		// Async command finished; fold any reported state back in
		if msg.enabled != nil {
			m.enabled = *msg.enabled
		}
		if msg.saved != nil {
			m.saved = msg.saved
		}
		m.status, m.statusErr = msg.text, msg.isErr
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			// Toggle between the two panels (0<->1)
			m.focusedPanel = commonPanel - m.focusedPanel
		case "up":
			// Up/down only move the cursor in the Advanced panel
			if m.focusedPanel != advancedPanel {
				break
			}
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.focusedPanel != advancedPanel {
				break
			}
			if m.cursor < len(fields)-1 {
				m.cursor++
			}
		case "left":
			// Arrows only adjust fields in the Advanced panel
			if m.focusedPanel != advancedPanel {
				break
			}
			fields[m.cursor].adjust(&m, -1)
		case "right":
			if m.focusedPanel != advancedPanel {
				break
			}
			fields[m.cursor].adjust(&m, 1)
		case "n":
			// New profile (Advanced only): append a default and select it
			if m.focusedPanel != advancedPanel {
				break
			}
			m.profiles = append(m.profiles, defaultHyprsunsetProfile())
			m.selected = len(m.profiles) - 1
		case "d":
			// Delete the selected profile (Advanced only); keep at least one
			if m.focusedPanel != advancedPanel {
				break
			}
			if len(m.profiles) == 1 {
				m.status, m.statusErr = "keep at least one profile", true
				break
			}
			m.profiles = append(m.profiles[:m.selected], m.profiles[m.selected+1:]...)
			m.selected = clamp(m.selected, 0, len(m.profiles)-1)
		case " ":
			if m.focusedPanel == commonPanel {
				return m, setEnabledCmd(!m.enabled)
			}
		case "s":
			// Persist all profiles to the config file
			return m, saveConfigCmd(m.profiles)
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

// field is one editable row; render shows the value, adjust changes it by dir (-1/+1)
type field struct {
	label  string
	render func(m model) string
	adjust func(m *model, dir int)
}

// fields is the ordered list of rows in the Advanced panel. fields[0] selects
// which profile is being edited; the rest edit that profile's attributes.
var fields = []field{
	// Profile: cycle the selected profile; render shows position in the list
	{"Profile", func(m model) string { return fmt.Sprintf("[%d/%d]", m.selected+1, len(m.profiles)) }, func(m *model, d int) {
		n := len(m.profiles)
		m.selected = (m.selected + d + n) % n
	}},
	// Time: shift by timeStep minutes, wrapping at midnight
	{"Time", func(m model) string { return m.current().time }, func(m *model, d int) {
		m.current().time = adjustTime(m.current().time, d*timeStep)
	}},
	// Identity: boolean toggle; direction is ignored
	{"Identity", func(m model) string { return strconv.FormatBool(m.current().identity) }, func(m *model, d int) {
		m.current().identity = !m.current().identity
	}},
	// Temperature: step by tempStep K, clamped to [tempMin, tempMax]
	{"Temperature", func(m model) string { return strconv.Itoa(m.current().temperature) + " K" }, func(m *model, d int) {
		m.current().temperature = clamp(m.current().temperature+d*tempStep, tempMin, tempMax)
	}},
	// Gamma: step by gammaStep, clamped to [gammaMin, gammaMax]
	{"Gamma", func(m model) string { return fmt.Sprintf("%.1f", m.current().gamma) }, func(m *model, d int) {
		m.current().gamma = clamp(m.current().gamma+float32(d)*gammaStep, gammaMin, gammaMax)
	}},
}

// profileFields are the attribute rows (everything after the Profile selector),
// reused by the Configuration diff box
var profileFields = fields[1:]

// adjustTime shifts "H:MM" by deltaMin, wrapping within a day
func adjustTime(s string, deltaMin int) string {
	var h, min int
	fmt.Sscanf(s, "%d:%d", &h, &min) // parse hours and minutes
	// Convert to minutes, apply delta, wrap into [0,1440); double mod handles negatives
	t := ((h*60+min+deltaMin)%1440 + 1440) % 1440
	return fmt.Sprintf("%02d:%02d", t/60, t%60) // back to "HH:MM"
}

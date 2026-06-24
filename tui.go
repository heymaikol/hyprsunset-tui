package main

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	tempStep                         = 250         // Kelvin per left/right press on Temperature
	gammaStep                float32 = 0.1         // gamma units per press
	timeStep                         = 15          // minutes per press on Time
	tempMin, tempMax                 = 1000, 10000 // clamp range for temperature (K)
	gammaMin                 float32 = 0.1         // lowest allowed gamma
	neutralTemp                      = 6000        // daylight-neutral temperature
	neutralGamma             float32 = 1.0         // unadjusted gamma
	maxGammaStep                     = 10          // percent per press on Max Gamma
	maxGammaMin, maxGammaMax         = 100, 200    // clamp range for max-gamma (%)
	defaultMaxGamma                  = 100         // hyprsunset's default max-gamma (%)
)

// Returns lo if v is below it
// Returns hi if v is above it
func clamp[T cmp.Ordered](v, lo, hi T) T {
	return max(lo, min(hi, v))
}

// model is the full TUI state
type model struct {
	profiles      []hyprsunsetProfile // all profiles, editable in the Advanced panel
	selected      int                 // index of the profile being edited
	cursor        int                 // selected row in the Advanced panel
	focusAdvanced bool                // true when the Advanced panel has focus
	enabled       bool                // is hyprsunset currently running
	status        string              // status line text
	statusErr     bool                // render status as an error
	saved         []hyprsunsetProfile // on-disk profiles, for the diff box
	maxGamma      int                 // global max-gamma (%), the gamma ceiling
	savedMaxGamma int                 // on-disk max-gamma, for the diff box
}

// current returns a pointer to the profile being edited
func (m *model) current() *hyprsunsetProfile { return &m.profiles[m.selected] }

// initialModel builds the starting state from the on-disk profiles and the
// live hyprsunset status, falling back to defaults on error
func initialModel() model {
	// Load the saved profiles; use a single default if they can't be read
	profiles, maxGamma, err := loadHyprsunsetProfiles()
	if err != nil {
		profiles = []hyprsunsetProfile{defaultHyprsunsetProfile()}
		maxGamma = defaultMaxGamma
	}
	// Seed the model from the profiles, start focused on the Simple panel
	m := model{
		profiles:      profiles,
		saved:         slices.Clone(profiles),
		maxGamma:      maxGamma,
		savedMaxGamma: maxGamma,
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
	text          string
	isErr         bool
	enabled       *bool
	saved         []hyprsunsetProfile
	savedMaxGamma int
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
func saveConfigCmd(profiles []hyprsunsetProfile, maxGamma int) tea.Cmd {
	saved := slices.Clone(profiles)
	return func() tea.Msg {
		if err := saveHyprsunsetProfiles(saved, maxGamma); err != nil {
			return statusMsg{text: "save: " + err.Error(), isErr: true}
		}
		return statusMsg{text: "saved configuration", isErr: false, saved: saved, savedMaxGamma: maxGamma}
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
			m.savedMaxGamma = msg.savedMaxGamma
		}
		m.status, m.statusErr = msg.text, msg.isErr
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			m.focusAdvanced = !m.focusAdvanced
		case "up":
			// Up/down only move the cursor in the Advanced panel
			if !m.focusAdvanced {
				break
			}
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if !m.focusAdvanced {
				break
			}
			if m.cursor < len(fields)-1 {
				m.cursor++
			}
		case "left":
			// Arrows only adjust fields in the Advanced panel
			if !m.focusAdvanced {
				break
			}
			fields[m.cursor].adjust(&m, -1)
		case "right":
			if !m.focusAdvanced {
				break
			}
			fields[m.cursor].adjust(&m, 1)
		case "backspace":
			// Clear the selected attribute so it's omitted from the saved
			// config; |= 0 is a no-op on the Profile selector. Re-add with ←/→.
			if !m.focusAdvanced {
				break
			}
			m.current().unset |= fields[m.cursor].bit
		case "n":
			// New profile (Advanced only): append a default and select it
			if !m.focusAdvanced {
				break
			}
			m.profiles = append(m.profiles, defaultHyprsunsetProfile())
			m.selected = len(m.profiles) - 1
		case "d":
			// Delete the selected profile (Advanced only); keep at least one
			if !m.focusAdvanced {
				break
			}
			if len(m.profiles) == 1 {
				m.status, m.statusErr = "keep at least one profile", true
				break
			}
			m.profiles = append(m.profiles[:m.selected], m.profiles[m.selected+1:]...)
			m.selected = clamp(m.selected, 0, len(m.profiles)-1)
		case " ":
			if !m.focusAdvanced {
				return m, setEnabledCmd(!m.enabled)
			}
		case "s":
			// Persist all profiles to the config file
			return m, saveConfigCmd(m.profiles, m.maxGamma)
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

// unsetMark is shown for an attribute the user cleared with backspace
const unsetMark = "—"

// field is one editable row; render shows the value, adjust changes it by dir (-1/+1)
type field struct {
	label  string
	bit    fieldBit // attribute this row edits; 0 = not clearable (Profile selector)
	render func(m model) string
	adjust func(m *model, dir int)
}

// shown renders val, or the unset marker if attribute b was cleared
func shown(m model, b fieldBit, val func() string) string {
	if !m.current().isSet(b) {
		return unsetMark
	}
	return val()
}

// edit applies f and marks attribute b present again, re-adding a cleared value
func edit(m *model, b fieldBit, f func()) {
	m.current().unset &^= b
	f()
}

// fields is the ordered list of rows in the Advanced panel. fields[0] selects
// which profile is being edited; the rest edit that profile's attributes.
var fields = []field{
	// Profile: cycle the selected profile; render shows position in the list
	{"Profile", 0, func(m model) string { return fmt.Sprintf("[%d/%d]", m.selected+1, len(m.profiles)) }, func(m *model, d int) {
		n := len(m.profiles)
		m.selected = (m.selected + d + n) % n
	}},
	// Time: shift by timeStep minutes, wrapping at midnight
	{"Time", timeBit, func(m model) string { return shown(m, timeBit, func() string { return m.current().time }) }, func(m *model, d int) {
		edit(m, timeBit, func() { m.current().time = adjustTime(m.current().time, d*timeStep) })
	}},
	// Identity: boolean toggle; direction is ignored
	{"Identity", identityBit, func(m model) string {
		return shown(m, identityBit, func() string { return strconv.FormatBool(m.current().identity) })
	}, func(m *model, _ int) {
		edit(m, identityBit, func() { m.current().identity = !m.current().identity })
	}},
	// Temperature: step by tempStep K, clamped to [tempMin, tempMax]
	{"Temperature", temperatureBit, func(m model) string {
		return shown(m, temperatureBit, func() string { return strconv.Itoa(m.current().temperature) + " K" })
	}, func(m *model, d int) {
		edit(m, temperatureBit, func() { m.current().temperature = clamp(m.current().temperature+d*tempStep, tempMin, tempMax) })
	}},
	// Gamma: step by gammaStep, clamped to [gammaMin, maxGamma]; the ceiling is
	// the global max-gamma so the app can't set a gamma hyprsunset would reject
	{"Gamma", gammaBit, func(m model) string {
		return shown(m, gammaBit, func() string { return fmt.Sprintf("%.1f", m.current().gamma) })
	}, func(m *model, d int) {
		edit(m, gammaBit, func() {
			m.current().gamma = clamp(m.current().gamma+float32(d)*gammaStep, gammaMin, float32(m.maxGamma)/100)
		})
	}},
	// Max Gamma: global gamma ceiling (%), not per-profile; bit 0 = not clearable
	{"Max Gamma", 0, func(m model) string { return fmt.Sprintf("%d%%", m.maxGamma) }, func(m *model, d int) {
		m.maxGamma = clamp(m.maxGamma+d*maxGammaStep, maxGammaMin, maxGammaMax)
	}},
}

// profileFields are the per-profile attribute rows: everything except the
// Profile selector (first) and the global Max Gamma (last), reused by the
// Configuration diff box
var profileFields = fields[1 : len(fields)-1]

// adjustTime shifts "H:MM" by deltaMin, wrapping within a day
func adjustTime(s string, deltaMin int) string {
	var h, mins int
	// Sscanf failure leaves h/mins at 0 — fine fallback for a malformed time
	_, _ = fmt.Sscanf(s, "%d:%d", &h, &mins)
	// Convert to minutes, apply delta, wrap into [0,1440); double mod handles negatives
	t := ((h*60+mins+deltaMin)%1440 + 1440) % 1440
	return fmt.Sprintf("%02d:%02d", t/60, t%60) // back to "HH:MM"
}

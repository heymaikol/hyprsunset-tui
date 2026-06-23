package main

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// Lipgloss styles, shared across the view
var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")) // app title (orange)
	valStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))  // current values (cyan)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))            // help text / old values (grey)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))            // error status (red)
)

// model is the full TUI state
type model struct {
	temp         int               // temperature in Kelvin
	gamma        float32           // gamma multiplier
	cursor       int               // selected row in the Advanced panel
	focusedPanel panel             // which panel has focus
	time         string            // schedule time, "HH:MM"
	identity     bool              // hyprsunset identity flag
	enabled      bool              // is hyprsunset currently running
	status       string            // status line text
	statusErr    bool              // render status as an error
	saved        hyprsunsetProfile // on-disk profile, for the diff box
}

// panel identifies a focusable region of the UI
type panel int

const (
	advancedPanel panel = iota // editable field list
	commonPanel                // simple enable/disable toggle
)

// initialModel builds the starting state from the on-disk profile and the
// live hyprsunset status, falling back to defaults on error
func initialModel() model {
	// Load the saved profile; use defaults if it can't be read
	profile, err := loadHyprsunsetProfile()
	if err != nil {
		profile = defaultHyprsunsetProfile()
	}
	// Seed the model from the profile, start focused on the Simple panel
	m := model{
		temp:         profile.temperature,
		gamma:        profile.gamma,
		time:         profile.time,
		identity:     profile.identity,
		focusedPanel: commonPanel,
		saved:        profile,
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
	saved   *hyprsunsetProfile
}

// applyCmd pushes temp/gamma to the running hyprsunset (not persisted)
func applyCmd(temp int, gamma float32) tea.Cmd {
	return func() tea.Msg {
		// Apply temperature first; bail out on failure
		if err := SetTemperature(temp); err != nil {
			return statusMsg{text: "temperature: " + err.Error(), isErr: true}
		}
		// Gamma is sent as an integer percent (1.0 -> 100)
		if err := SetGamma(int(gamma * 100)); err != nil {
			return statusMsg{text: "gamma: " + err.Error(), isErr: true}
		}
		return statusMsg{text: fmt.Sprintf("applied %dK / %.1f", temp, gamma), isErr: false}
	}
}

// setEnabledCmd starts or stops hyprsunset and reports the new state
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

// saveConfigCmd writes the profile to disk and updates the diff baseline
func saveConfigCmd(profile hyprsunsetProfile) tea.Cmd {
	return func() tea.Msg {
		if err := saveHyprsunsetProfile(profile); err != nil {
			return statusMsg{text: "save: " + err.Error(), isErr: true}
		}
		return statusMsg{text: "saved configuration", isErr: false, saved: &profile}
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
			m.saved = *msg.saved
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
			// In Simple panel, left disables hyprsunset; in Advanced it decrements the field
			if m.focusedPanel != advancedPanel {
				if m.focusedPanel == commonPanel && m.enabled {
					return m, setEnabledCmd(false)
				}
				break
			}
			fields[m.cursor].adjust(&m, -1)
		case "right":
			// In Simple panel, right enables hyprsunset; in Advanced it increments the field
			if m.focusedPanel != advancedPanel {
				if m.focusedPanel == commonPanel && !m.enabled {
					return m, setEnabledCmd(true)
				}
				break
			}
			fields[m.cursor].adjust(&m, 1)
		case " ", "x":
			// Space/x flips the enabled toggle from the Simple panel
			if m.focusedPanel == commonPanel {
				return m, setEnabledCmd(!m.enabled)
			}
		case "enter":
			// Apply current temp/gamma to the live session
			return m, applyCmd(m.temp, m.gamma)
		case "s":
			// Persist the current values to the profile file
			return m, saveConfigCmd(hyprsunsetProfile{
				time:        m.time,
				temperature: m.temp,
				gamma:       m.gamma,
				identity:    m.identity,
			})
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

// fields is the ordered list of editable rows in the Advanced panel
var fields = []field{
	// Time: shift by timeStep minutes, wrapping at midnight
	{"Time", func(m model) string { return m.time }, func(m *model, d int) { m.time = adjustTime(m.time, d*timeStep) }},
	// Identity: boolean toggle; direction is ignored
	{"Identity", func(m model) string { return strconv.FormatBool(m.identity) }, func(m *model, d int) { m.identity = !m.identity }},
	// Temperature: step by tempStep K, clamped to [tempMin, tempMax]
	{"Temperature", func(m model) string { return strconv.Itoa(m.temp) + " K" }, func(m *model, d int) { m.temp = clamp(m.temp+d*tempStep, tempMin, tempMax) }},
	// Gamma: step by gammaStep, clamped to [gammaMin, gammaMax]
	{"Gamma", func(m model) string { return fmt.Sprintf("%.1f", m.gamma) }, func(m *model, d int) { m.gamma = clamp(m.gamma+float32(d)*gammaStep, gammaMin, gammaMax) }},
}

// adjustTime shifts "H:MM" by deltaMin, wrapping within a day
func adjustTime(s string, deltaMin int) string {
	var h, min int
	fmt.Sscanf(s, "%d:%d", &h, &min) // parse hours and minutes
	// Convert to minutes, apply delta, wrap into [0,1440); double mod handles negatives
	t := ((h*60+min+deltaMin)%1440 + 1440) % 1440
	return fmt.Sprintf("%02d:%02d", t/60, t%60) // back to "HH:MM"
}

// renderBox draws a bluetui-style bordered box: title sits in the top border,
// focused box gets a bright border, auto-sizes to title/body width
func renderBox(title, body string, focused bool) string {
	border := lipgloss.RoundedBorder()
	// Focused box gets a bright (orange) border, otherwise grey
	color := lipgloss.Color("244")
	if focused {
		color = lipgloss.Color("214")
	}
	bs := lipgloss.NewStyle().Foreground(color) // border-segment style

	// Render the body inside a padded border
	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(color).
		Padding(0, 1)
	lines := strings.Split(style.Render(body), "\n")
	// If the body is narrower than the title needs, re-render wider to fit it
	titleWidth := lipgloss.Width(title)
	if lipgloss.Width(lines[0]) < titleWidth+6 {
		lines = strings.Split(style.Width(titleWidth+4).Render(body), "\n")
	}
	// Rebuild the top border with the title embedded, padding the rest with border runes
	width := lipgloss.Width(lines[0])
	fill := width - titleWidth - 5 // remaining cells after corners, title, and spaces
	top := bs.Render(border.TopLeft+border.Top+" ") +
		bs.Bold(true).Render(title) +
		bs.Render(" "+strings.Repeat(border.Top, fill)+border.TopRight)
	lines[0] = top // replace the plain top border with the titled one
	return strings.Join(lines, "\n")
}

// View renders the whole UI: title, Simple/Advanced panels, Configuration diff, and help
func (m model) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render("hyprsunset-controller"))

	// Build the Advanced panel body, marking the selected row with "> "
	var adv strings.Builder
	for i, f := range fields {
		prefix := "  "
		if m.focusedPanel == advancedPanel && m.cursor == i {
			prefix = "> "
		}
		fmt.Fprintf(&adv, "%s%s: %s\n", prefix, f.label, valStyle.Render(f.render(m)))
	}

	// Simple panel: a single enabled checkbox
	checkbox := "[ ]"
	if m.enabled {
		checkbox = "[x]"
	}
	commonPrefix := "> " // cursor marker, only shown when the panel is focused
	if m.focusedPanel != commonPanel {
		commonPrefix = "  "
	}
	commonBody := fmt.Sprintf("%s%s Enabled", commonPrefix, checkbox)
	common := renderBox("Simple", commonBody, m.focusedPanel == commonPanel)
	advanced := renderBox("Advanced", strings.TrimRight(adv.String(), "\n"), m.focusedPanel == advancedPanel)
	left := lipgloss.JoinVertical(lipgloss.Left, common, advanced) // stack the two left-column boxes

	// Configuration box: reuse field renders against a model holding the on-disk values
	old := m
	old.time, old.identity = m.saved.time, m.saved.identity
	old.temp, old.gamma = m.saved.temperature, m.saved.gamma
	var prof strings.Builder
	for _, f := range fields {
		cur, was := f.render(m), f.render(old) // current vs saved value
		val := valStyle.Render(cur)
		// Show "old → new" when the live value differs from disk
		if cur != was {
			val = dimStyle.Render(was) + " → " + valStyle.Render(cur)
		}
		fmt.Fprintf(&prof, "%s: %s\n", f.label, val)
	}
	// Pad Configuration body so its box matches the stacked-left column height;
	// box adds 2 border rows, so body needs leftHeight-2 lines
	profBody := strings.TrimRight(prof.String(), "\n")
	if pad := lipgloss.Height(left) - 2 - lipgloss.Height(profBody); pad > 0 {
		profBody += strings.Repeat("\n", pad)
	}
	profile := renderBox("Configuration", profBody, false)

	// Place the Configuration box to the right of the stacked left column
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", profile))
	b.WriteByte('\n')

	// Two-line key hint footer
	fmt.Fprintf(&b, "\n%s\n", dimStyle.Render("[tab] panel   [↑/↓] select   [←/→] adjust"))
	fmt.Fprintf(&b, "%s\n", dimStyle.Render("[space] enable   [enter] apply   [s] save   [q] quit"))
	// Status line, red on error
	if m.status != "" {
		style := dimStyle
		if m.statusErr {
			style = errStyle
		}
		fmt.Fprintf(&b, "\n%s\n", style.Render("  > "+m.status))
	}
	return b.String()
}

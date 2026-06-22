package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tempStep                 = 250
	gammaStep        float32 = 0.1
	timeStep                 = 15 // minutes
	tempMin, tempMax         = 1000, 10000
	gammaMin         float32 = 0.1
	gammaMax         float32 = 2.0
	neutralTemp              = 6000
	neutralGamma     float32 = 1.0
)

type preset struct {
	name  string
	temp  int
	gamma float32
}

var presets = []preset{
	{"Day", 6000, 1.0},
	{"Evening", 4000, 0.9},
	{"Night", 3000, 0.8},
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	valStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

type model struct {
	temp      int
	gamma     float32
	cursor    int
	time      string
	identity  bool
	status    string
	statusErr bool
}

func initialModel() model {
	profile, err := loadHyprsunsetProfile(time.Now())
	m := model{
		temp:     profile.temperature,
		gamma:    profile.gamma,
		time:     profile.time,
		identity: profile.identity,
	}
	if err != nil {
		m.status = "config: " + err.Error()
		m.statusErr = true
	}
	return m
}

func (m model) Init() tea.Cmd { return nil }

// appliedMsg carries the result of an async hyprsunset call back into Update.
type appliedMsg struct {
	text  string
	isErr bool
}

func applyCmd(temp int, gamma float32) tea.Cmd {
	return func() tea.Msg {
		if err := SetTemperature(temp); err != nil {
			return appliedMsg{"temperature: " + err.Error(), true}
		}
		if err := SetGamma(int(gamma * 100)); err != nil {
			return appliedMsg{"gamma: " + err.Error(), true}
		}
		return appliedMsg{fmt.Sprintf("applied %dK / %.1f", temp, gamma), false}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case appliedMsg:
		m.status, m.statusErr = msg.text, msg.isErr
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(fields)-1 {
				m.cursor++
			}
		case "left":
			fields[m.cursor].adjust(&m, -1)
		case "right":
			fields[m.cursor].adjust(&m, 1)
		case "1", "2", "3":
			p := presets[msg.String()[0]-'1']
			m.temp, m.gamma = p.temp, p.gamma
			m.status, m.statusErr = "preset: "+p.name, false
		case "a", "enter":
			return m, applyCmd(m.temp, m.gamma)
		case "i":
			profile, err := loadHyprsunsetProfile(time.Now())
			if err != nil {
				m.status, m.statusErr = "config: "+err.Error(), true
				return m, nil
			}
			m.temp = profile.temperature
			m.gamma = profile.gamma
			m.time = profile.time
			m.identity = profile.identity
			// Missing thing?
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

// field is one editable row. render shows the value; adjust changes it by dir (-1/+1).
type field struct {
	label  string
	render func(m model) string
	adjust func(m *model, dir int)
}

var fields = []field{
	{"Time", func(m model) string { return m.time }, func(m *model, d int) { m.time = adjustTime(m.time, d*timeStep) }},
	{"Identity", func(m model) string { return strconv.FormatBool(m.identity) }, func(m *model, d int) { m.identity = !m.identity }},
	{"Temperature", func(m model) string { return strconv.Itoa(m.temp) + " K" }, func(m *model, d int) { m.temp = clamp(m.temp+d*tempStep, tempMin, tempMax) }},
	{"Gamma", func(m model) string { return fmt.Sprintf("%.1f", m.gamma) }, func(m *model, d int) { m.gamma = clampFloat(m.gamma+float32(d)*gammaStep, gammaMin, gammaMax) }},
}

// adjustTime shifts "H:MM" by deltaMin, wrapping within a day.
func adjustTime(s string, deltaMin int) string {
	var h, min int
	fmt.Sscanf(s, "%d:%d", &h, &min)
	t := ((h*60+min+deltaMin)%1440 + 1440) % 1440
	return fmt.Sprintf("%02d:%02d", t/60, t%60)
}

func (m model) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render("hyprsunset-controller"))
	for i, f := range fields {
		prefix := "  "
		if m.cursor == i {
			prefix = "> "
		}
		fmt.Fprintf(&b, "%s%s: %s\n", prefix, f.label, valStyle.Render(f.render(m)))
	}
	fmt.Fprintf(&b, "\n%s\n", dimStyle.Render("[↑/↓] select   [←/→] adjust"))
	fmt.Fprintf(&b, "%s\n", dimStyle.Render("[1] Day  [2] Evening  [3] Night"))
	fmt.Fprintf(&b, "%s\n", dimStyle.Render("[a/enter] apply   [i] reset to profile   [q] quit"))
	if m.status != "" {
		style := dimStyle
		if m.statusErr {
			style = errStyle
		}
		fmt.Fprintf(&b, "\n%s\n", style.Render("  > "+m.status))
	}
	return b.String()
}

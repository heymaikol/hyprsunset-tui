package main

import (
	"fmt"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tempStep                 = 250
	gammaStep        float32 = 0.1
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
	status    string
	statusErr bool
}

func initialModel() model {
	profile, err := loadHyprsunsetProfile(time.Now())
	m := model{temp: profile.temperature, gamma: profile.gamma}
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
		case "t", "left":
			m.temp = clamp(m.temp-tempStep, tempMin, tempMax)
		case "T", "right":
			m.temp = clamp(m.temp+tempStep, tempMin, tempMax)
		case "g", "down":
			m.gamma = clampFloat(m.gamma-gammaStep, gammaMin, gammaMax)
		case "G", "up":
			m.gamma = clampFloat(m.gamma+gammaStep, gammaMin, gammaMax)
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
			m.temp, m.gamma = profile.temperature, profile.gamma
			// Missing thing?
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	s := titleStyle.Render("hyprsunset-controller") + "\n\n"
	s += fmt.Sprintf("  Temperature: %s K\n", valStyle.Render(strconv.Itoa(m.temp)))
	s += fmt.Sprintf("  Gamma:       %s\n\n", valStyle.Render(fmt.Sprintf("%.1f", m.gamma)))
	s += dimStyle.Render("  [t/T or ←/→] temp   [g/G or ↓/↑] gamma") + "\n"
	s += dimStyle.Render("  [1] Day  [2] Evening  [3] Night") + "\n"
	s += dimStyle.Render("  [a/enter] apply   [i] reset to profile   [q] quit") + "\n"
	if m.status != "" {
		style := dimStyle
		if m.statusErr {
			style = errStyle
		}
		s += "\n" + style.Render("  > "+m.status) + "\n"
	}
	return s
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// hyprsunset-controller — Bubble Tea TUI scaffold.
//
// Extend here:
//   - presets:    add entries to the presets slice
//   - keys:       add cases in Update's tea.KeyMsg switch, document them in View
//   - IPC:        add hyprsunset commands in hyprsunset.go, call them from a tea.Cmd
//   - persistence: load before tea.NewProgram, save after Run returns

const (
	tempStep, gammaStep = 250, 5
	tempMin, tempMax    = 1000, 10000
	gammaMin, gammaMax  = 10, 200
)

type preset struct {
	name        string
	temp, gamma int
}

var presets = []preset{
	{"Day", 6000, 100},
	{"Evening", 4000, 90},
	{"Night", 3000, 80},
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

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	valStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

type model struct {
	temp, gamma int
	status      string
	statusErr   bool
}

func initialModel() model {
	return model{temp: 6000, gamma: 100}
}

func (m model) Init() tea.Cmd { return nil }

// appliedMsg carries the result of an async hyprsunset call back into Update.
type appliedMsg struct {
	text  string
	isErr bool
}

func applyCmd(temp, gamma int) tea.Cmd {
	return func() tea.Msg {
		if err := SetTemperature(temp); err != nil {
			return appliedMsg{"temperature: " + err.Error(), true}
		}
		if err := SetGamma(gamma); err != nil {
			return appliedMsg{"gamma: " + err.Error(), true}
		}
		return appliedMsg{fmt.Sprintf("applied %dK / %d%%", temp, gamma), false}
	}
}

func identityCmd() tea.Cmd {
	return func() tea.Msg {
		if err := Identity(); err != nil {
			return appliedMsg{"identity: " + err.Error(), true}
		}
		return appliedMsg{"reset to neutral", false}
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
			m.gamma = clamp(m.gamma-gammaStep, gammaMin, gammaMax)
		case "G", "up":
			m.gamma = clamp(m.gamma+gammaStep, gammaMin, gammaMax)
		case "1", "2", "3":
			p := presets[msg.String()[0]-'1']
			m.temp, m.gamma = p.temp, p.gamma
			m.status, m.statusErr = "preset: "+p.name, false
		case "a", "enter":
			return m, applyCmd(m.temp, m.gamma)
		case "i":
			m.temp, m.gamma = 6000, 100
			return m, identityCmd()
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	s := titleStyle.Render("hyprsunset-controller") + "\n\n"
	s += fmt.Sprintf("  Temperature: %s K\n", valStyle.Render(strconv.Itoa(m.temp)))
	s += fmt.Sprintf("  Gamma:       %s %%\n\n", valStyle.Render(strconv.Itoa(m.gamma)))
	s += dimStyle.Render("  [t/T or ←/→] temp   [g/G or ↓/↑] gamma") + "\n"
	s += dimStyle.Render("  [1] Day  [2] Evening  [3] Night") + "\n"
	s += dimStyle.Render("  [a/enter] apply   [i] identity/reset   [q] quit") + "\n"
	if m.status != "" {
		style := dimStyle
		if m.statusErr {
			style = errStyle
		}
		s += "\n" + style.Render("  > "+m.status) + "\n"
	}
	return s
}

func CheckDependencies() error {
	// Check if hyprsunset is in PATH
	if _, err := exec.LookPath("hyprsunset"); err != nil {
		return fmt.Errorf("hyprsunset is not found in PATH (Not installed?)")
	}

	// Check if hyprctl is in PATH
	// Is it possible to install hyprsunset without hyprland?
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return fmt.Errorf("hyprctl is not found in PATH (Not installed?)")
	}

	return nil
}

func main() {
	// Check if Dependencies are installed
	if err := CheckDependencies(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// Create the TUI
	if _, err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

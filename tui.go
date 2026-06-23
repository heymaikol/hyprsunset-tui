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
	tempStep                 = 250
	gammaStep        float32 = 0.1
	timeStep                 = 15 // minutes
	tempMin, tempMax         = 1000, 10000
	gammaMin         float32 = 0.1
	gammaMax         float32 = 2.0
	neutralTemp              = 6000
	neutralGamma     float32 = 1.0
)

// Returns lo if v is below it
// Returns hi if v is above it
func clamp[T cmp.Ordered](v, lo, hi T) T {
	return max(lo, min(hi, v))
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	valStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

type model struct {
	temp         int
	gamma        float32
	cursor       int
	focusedPanel panel
	time         string
	identity     bool
	status       string
	statusErr    bool
}

type panel int

const (
	advancedPanel panel = iota
	commonPanel
)

func initialModel() model {
	profile, err := loadHyprsunsetProfile()
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
		case "tab", "shift+tab":
			m.togglePanel()
		case "up":
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
			if m.focusedPanel != advancedPanel {
				break
			}
			fields[m.cursor].adjust(&m, -1)
		case "right":
			if m.focusedPanel != advancedPanel {
				break
			}
			fields[m.cursor].adjust(&m, 1)
		case "a", "enter":
			return m, applyCmd(m.temp, m.gamma)
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *model) togglePanel() {
	if m.focusedPanel == advancedPanel {
		m.focusedPanel = commonPanel
		return
	}
	m.focusedPanel = advancedPanel
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
	{"Gamma", func(m model) string { return fmt.Sprintf("%.1f", m.gamma) }, func(m *model, d int) { m.gamma = clamp(m.gamma+float32(d)*gammaStep, gammaMin, gammaMax) }},
}

// adjustTime shifts "H:MM" by deltaMin, wrapping within a day.
func adjustTime(s string, deltaMin int) string {
	var h, min int
	fmt.Sscanf(s, "%d:%d", &h, &min)
	t := ((h*60+min+deltaMin)%1440 + 1440) % 1440
	return fmt.Sprintf("%02d:%02d", t/60, t%60)
}

// renderBox draws a bluetui-style bordered box: title sits in the top border,
// focused box gets a bright border. Auto-sizes to title/body width.
func renderBox(title, body string, focused bool) string {
	border := lipgloss.RoundedBorder()
	color := lipgloss.Color("244")
	if focused {
		color = lipgloss.Color("214")
	}
	bs := lipgloss.NewStyle().Foreground(color)

	lines := strings.Split(body, "\n")
	inner := lipgloss.Width(title) + 4
	for _, l := range lines {
		inner = max(inner, lipgloss.Width(l)+3)
	}

	fill := inner - lipgloss.Width(title) - 3
	top := bs.Render(border.TopLeft+border.Top+" ") +
		bs.Bold(true).Render(title) +
		bs.Render(" "+strings.Repeat(border.Top, fill)+border.TopRight)

	var sb strings.Builder
	sb.WriteString(top)
	sb.WriteByte('\n')
	for _, l := range lines {
		content := " " + l + strings.Repeat(" ", inner-lipgloss.Width(l)-1)
		sb.WriteString(bs.Render(border.Left))
		sb.WriteString(content)
		sb.WriteString(bs.Render(border.Right))
		sb.WriteByte('\n')
	}
	sb.WriteString(bs.Render(border.BottomLeft + strings.Repeat(border.Top, inner) + border.BottomRight))
	return sb.String()
}

func (m model) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render("hyprsunset-controller"))

	var adv strings.Builder
	for i, f := range fields {
		prefix := "  "
		if m.focusedPanel == advancedPanel && m.cursor == i {
			prefix = "> "
		}
		fmt.Fprintf(&adv, "%s%s: %s\n", prefix, f.label, valStyle.Render(f.render(m)))
	}

	// Common is intentionally empty for now; blank lines keep panel heights aligned.
	common := renderBox("Common", strings.Repeat("\n", len(fields)-1), m.focusedPanel == commonPanel)
	advanced := renderBox("Advanced", strings.TrimRight(adv.String(), "\n"), m.focusedPanel == advancedPanel)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, common, "  ", advanced))
	b.WriteByte('\n')

	fmt.Fprintf(&b, "\n%s\n", dimStyle.Render("[tab] panel   [↑/↓] select   [←/→] adjust"))
	fmt.Fprintf(&b, "%s\n", dimStyle.Render("[a/enter] apply   [q] quit"))
	if m.status != "" {
		style := dimStyle
		if m.statusErr {
			style = errStyle
		}
		fmt.Fprintf(&b, "\n%s\n", style.Render("  > "+m.status))
	}
	return b.String()
}

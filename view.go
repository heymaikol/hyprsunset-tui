package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Lipgloss styles, shared across the view
var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")) // app title (orange)
	valStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))  // current values (cyan)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))            // help text / old values (grey)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))            // error status (red)
)

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

	// Configuration box: list every profile, diffing live values against the
	// on-disk baseline (saved[i]); newly added profiles have no baseline
	var prof strings.Builder
	for i := range m.profiles {
		if i > 0 {
			prof.WriteByte('\n')
		}
		fmt.Fprintf(&prof, "%s\n", dimStyle.Render(fmt.Sprintf("Profile %d", i+1)))
		live := model{profiles: m.profiles, selected: i}
		hasBaseline := i < len(m.saved)
		old := model{profiles: m.saved, selected: i}
		for _, f := range profileFields {
			cur := f.render(live)
			val := valStyle.Render(cur)
			// Show "old → new" when the live value differs from disk
			if hasBaseline {
				if was := f.render(old); cur != was {
					val = dimStyle.Render(was) + " → " + valStyle.Render(cur)
				}
			}
			fmt.Fprintf(&prof, "%s: %s\n", f.label, val)
		}
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

	// Two-line key hint footer; first line depends on the focused panel
	directions := "[tab] panel   [space] toggle"
	if m.focusedPanel == advancedPanel {
		directions = "[tab] panel   [↑/↓] select   [←/→] adjust   [n] new   [d] del"
	}
	fmt.Fprintf(&b, "\n%s\n", dimStyle.Render(directions))
	fmt.Fprintf(&b, "%s\n", dimStyle.Render("[s] save   [q] quit"))
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

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
	focusStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")) // focused Simple cell (orange)
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

// profileLabel names a profile by its day/night role; profile 0 is Day, 1 is
// Night, and any extras (managed in the Advanced panel) keep a numbered label.
func profileLabel(i int) string {
	switch i {
	case 0:
		return "Day"
	case 1:
		return "Night"
	default:
		return fmt.Sprintf("Profile %d", i+1)
	}
}

// simpleBody renders the Simple panel: the Enabled toggle plus, once a
// Day/Night cycle exists, an editable row per profile. The focused cell is
// highlighted while the Simple panel has focus.
func simpleBody(m model) string {
	focused := !m.focusAdvanced
	fp, ff, cellOk := simpleCell(m.simpleCursor)

	var b strings.Builder
	// Enabled toggle (cursor 0)
	checkbox := "[ ]"
	if m.enabled {
		checkbox = "[x]"
	}
	prefix := "  "
	if focused && m.simpleCursor == 0 {
		prefix = "> "
	}
	fmt.Fprintf(&b, "%s%s Enabled", prefix, checkbox)

	for p := 0; p < m.simpleRows(); p++ {
		if p == 0 {
			b.WriteByte('\n') // blank line between Enabled and the first row
		}
		rowPrefix := "  "
		if focused && cellOk && fp == p {
			rowPrefix = "> "
		}
		view := model{profiles: []hyprsunsetProfile{m.profiles[p]}}
		fmt.Fprintf(&b, "\n%s%-6s", rowPrefix, profileLabel(p))
		for fi, f := range simpleProfileFields {
			style := valStyle
			if focused && cellOk && fp == p && ff == fi {
				style = focusStyle
			}
			fmt.Fprintf(&b, "  %s", style.Render(f.render(view)))
		}
	}
	return b.String()
}

// View renders the whole UI: title, Simple/Advanced panels, Configuration diff, and help
func (m model) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render("hstui"))

	// Build the Advanced panel body, marking the selected row with "> "
	var adv strings.Builder
	for i, f := range fields {
		prefix := "  "
		if m.focusAdvanced && m.cursor == i {
			prefix = "> "
		}
		fmt.Fprintf(&adv, "%s%s: %s\n", prefix, f.label, valStyle.Render(f.render(m)))
	}

	common := renderBox("Simple", simpleBody(m), !m.focusAdvanced)
	advanced := renderBox("Advanced", strings.TrimRight(adv.String(), "\n"), m.focusAdvanced)
	left := lipgloss.JoinVertical(lipgloss.Left, common, advanced) // stack the two left-column boxes

	// Configuration box: list every profile, diffing live values against the
	// on-disk baseline (saved[i]); newly added profiles have no baseline
	var prof strings.Builder
	// Global max-gamma sits above the profiles, shown only when raised above the
	// default; diffs against the on-disk baseline like the per-profile rows
	if m.maxGamma > defaultMaxGamma {
		cur := fmt.Sprintf("%d%%", m.maxGamma)
		val := valStyle.Render(cur)
		if m.savedMaxGamma != m.maxGamma {
			val = dimStyle.Render(fmt.Sprintf("%d%%", m.savedMaxGamma)) + " → " + val
		}
		fmt.Fprintf(&prof, "Max Gamma: %s\n\n", val)
	}
	for i := range m.profiles {
		if i > 0 {
			prof.WriteByte('\n')
		}
		fmt.Fprintf(&prof, "%s\n", dimStyle.Render(profileLabel(i)))
		live := model{profiles: m.profiles, selected: i}
		hasBaseline := i < len(m.saved)
		old := model{profiles: m.saved, selected: i}
		for _, f := range profileFields {
			cur := f.render(live)
			val := valStyle.Render(cur)
			// Show "old → new" when the live value differs from disk
			if hasBaseline {
				if was := f.render(old); cur != was {
					val = dimStyle.Render(was) + " → " + val
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
	directions := "[tab] panel   [↑/↓] select   [←/→] adjust   [space] toggle"
	if m.focusAdvanced {
		directions = "[tab] panel   [↑/↓] select   [←/→] adjust   [bksp] clear   [n] new   [d] del"
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

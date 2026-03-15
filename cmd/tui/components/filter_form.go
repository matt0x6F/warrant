package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FilterForm renders labeled filter sections side-by-side (2 or 3 columns).
// Focus indicates which section is active (0, 1, or 2). Tab switches focus.
// Each section has its own items and selected index.
// Section3 is optional; if Section3Items is nil, only 2 columns are rendered.
type FilterForm struct {
	Section1Label    string
	Section1Items    []string
	Section1Selected int

	Section2Label    string
	Section2Items    []string
	Section2Selected int

	Section3Label    string
	Section3Items    []string
	Section3Selected int

	Focus int // 0 = section 1, 1 = section 2, 2 = section 3 (if present)
}

func (f FilterForm) renderSection(label string, items []string, selected int, focused bool) string {
	var b strings.Builder
	b.WriteString(Muted.Render(label) + "\n\n")
	for i, item := range items {
		if i == selected && focused {
			b.WriteString(Primary.Render("▸ ") + Secondary.Render(item) + "\n")
		} else if i == selected {
			b.WriteString("  " + Primary.Render(item) + "\n")
		} else {
			b.WriteString("  " + Secondary.Render(item) + "\n")
		}
	}
	return b.String()
}

func (f FilterForm) Render(width int) string {
	col1 := f.renderSection(f.Section1Label, f.Section1Items, f.Section1Selected, f.Focus == 0)
	col2 := f.renderSection(f.Section2Label, f.Section2Items, f.Section2Selected, f.Focus == 1)
	if len(f.Section3Items) > 0 {
		col3 := f.renderSection(f.Section3Label, f.Section3Items, f.Section3Selected, f.Focus == 2)
		return lipgloss.JoinHorizontal(lipgloss.Top, col1, "    ", col2, "    ", col3)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, col1, "    ", col2)
}

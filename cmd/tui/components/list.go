package components

import "strings"

type SelectList struct {
	Items        []string
	Selected     int
	EmptyMessage string
}

func (s SelectList) Render(width int) string {
	if len(s.Items) == 0 {
		return Muted.Render(s.EmptyMessage)
	}
	var lines []string
	for i, item := range s.Items {
		if i == s.Selected {
			lines = append(lines, Primary.Render("▸ ")+Secondary.Render(item))
		} else {
			lines = append(lines, "  "+Secondary.Render(item))
		}
	}
	return strings.Join(lines, "\n")
}

package components

import "strings"

func KeyHintBar(hints []string) string {
	return Muted.Render(strings.Join(hints, "  "))
}

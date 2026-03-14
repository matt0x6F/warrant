package components

type SelectList struct {
	Items        []string
	Selected     int
	EmptyMessage string
}

func (s SelectList) Render(width int) string {
	if len(s.Items) == 0 {
		return Muted.Render(s.EmptyMessage)
	}
	// TODO: implement in next task
	return ""
}

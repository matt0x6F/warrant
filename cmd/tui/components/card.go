package components

func CardRender(title, body string, width int) string {
	inner := body
	if title != "" {
		inner = Primary.Render(title) + "\n" + body
	}
	return Border.Width(width).Padding(1, 2).Render(inner)
}

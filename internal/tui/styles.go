package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	subtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("204")).
			Bold(true)

	focusedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	blurredStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Bold(true)

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// JSON token styles, used by the response screen's fx-style tree view.
	jsonKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	jsonStringStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	jsonNumberStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

	jsonBoolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

	jsonNullStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	jsonPunctStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	statusOKStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("42"))

	statusRedirectStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))

	statusClientErrStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("214"))

	statusServerErrStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("204"))
)

// statusClass classifies an HTTP status code as "2xx"/"3xx"/"4xx"/"5xx", or
// "" if it doesn't fall in any of those ranges. Kept separate from
// statusStyle (a pure, easily-testable mapping) since comparing rendered
// lipgloss.Style output isn't meaningful — lipgloss emits no ANSI codes at
// all outside a real terminal, so styled and unstyled render identically
// under `go test`.
func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return ""
	}
}

// statusStyle returns the style to render an HTTP status line in, based
// on its status class: 2xx green, 3xx cyan, 4xx orange, 5xx red — the
// same convention curl/httpie/browser devtools already use.
func statusStyle(code int) lipgloss.Style {
	switch statusClass(code) {
	case "2xx":
		return statusOKStyle
	case "3xx":
		return statusRedirectStyle
	case "4xx":
		return statusClientErrStyle
	case "5xx":
		return statusServerErrStyle
	default:
		return labelStyle
	}
}

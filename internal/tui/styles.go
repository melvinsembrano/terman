package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	subtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// helpKeyStyle, helpDescStyle and helpSepStyle style key hints in
	// footers throughout the app: bold accent-pink keys, legible gray
	// labels, and a dim separator between hints, so the footer reads as
	// distinct "key label" pairs instead of a flat wall of gray text.
	helpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	helpSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

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
			Foreground(lipgloss.Color("240"))

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

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

// keyHint pairs a key label (e.g. "esc") with what it does (e.g. "back"),
// for rendering hand-rolled screen footers with renderHints.
type keyHint struct {
	key  string
	desc string
}

// renderHints renders a footer line of key hints as "key desc • key desc",
// with keys in accent-pink bold, descriptions in legible gray, and dim
// separators — consistent with the styling applied to bubbles' built-in
// help component on the list screens.
func renderHints(hints ...keyHint) string {
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = helpKeyStyle.Render(h.key) + " " + helpDescStyle.Render(h.desc)
	}
	return strings.Join(parts, helpSepStyle.Render(" • "))
}

// newStyledDelegate builds a list.DefaultDelegate with row colors matching
// the app's accent-pink/gray theme instead of bubbles' default indigo. Only
// colors are touched — Height/Spacing (and thus row layout) are left at
// their defaults, since mouse.go's click hit-testing depends on them.
func newStyledDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.NormalTitle = d.Styles.NormalTitle.Foreground(lipgloss.Color("252"))
	d.Styles.NormalDesc = d.Styles.NormalDesc.Foreground(lipgloss.Color("245"))
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.
		Foreground(lipgloss.Color("212")).
		BorderForeground(lipgloss.Color("212")).
		Bold(true)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.
		Foreground(lipgloss.Color("212")).
		BorderForeground(lipgloss.Color("212"))
	return d
}

// styleListHelp applies the app's accent-pink/gray footer theme to a
// list.Model's built-in help component (rendered as the screen's footer).
func styleListHelp(lst *list.Model) {
	lst.Help.Styles.ShortKey = helpKeyStyle
	lst.Help.Styles.ShortDesc = helpDescStyle
	lst.Help.Styles.ShortSeparator = helpSepStyle
	lst.Help.Styles.FullKey = helpKeyStyle
	lst.Help.Styles.FullDesc = helpDescStyle
	lst.Help.Styles.FullSeparator = helpSepStyle
	lst.Help.Styles.Ellipsis = helpSepStyle
	lst.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
}

package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/httpx"
	"github.com/melvinsembrano/terman/internal/jsonview"
	"github.com/melvinsembrano/terman/internal/model"
)

// dividerWidth is how wide the section-separator rules are. The response
// screen doesn't always know the real viewport width yet when it first
// renders (e.g. before the first WindowSizeMsg), so this is a fixed,
// reasonable default rather than measuring s.vp.Width.
const dividerWidth = 40

// responseViewportTop is the absolute terminal row (from the top of the
// screen) where the response viewport's own content begins.
//
// The app header is "terman…\n" + "divider\n" — two newlines, which puts
// body content starting at terminal row 2, not row 3.  headerLines = 3
// (lipgloss.Height counts trailing-newline strings as one line taller than
// their visible row count), so it over-counts by one.  The response View()
// then adds a title line and one blank line ("\n\n") before the viewport,
// contributing 2 more rows.  Net: viewport starts at row 2 + 2 = 4, which
// equals headerLines - 1 + 2 = headerLines + 1.
//
// Concretely: header(2 visible rows) + title(1) + blank(1) = 4 rows before
// the viewport, so terminal row 4 is viewport content row 0.
const responseViewportTop = headerLines + 1

// runResultMsg carries the outcome of an asynchronously executed request
// back into the Bubble Tea update loop.
type runResultMsg struct {
	name string
	resp httpx.Response
	err  error
}

// runRequestCmd executes req in the background and reports the result as
// a runResultMsg.
func runRequestCmd(req model.Request, vars map[string]string) tea.Cmd {
	return func() tea.Msg {
		resp, err := httpx.Do(req, vars)
		return runResultMsg{name: req.Name, resp: resp, err: err}
	}
}

// responseScreen shows the outcome of running a request: a status line
// (colored by status class), a labeled headers block, and a labeled body
// block. JSON bodies render as an fx-style interactive tree — ↑/↓ moves a
// line cursor, enter/space folds or unfolds the object/array under it,
// collapsed containers show as "{…3}"/"[…5]". Anything else stays a plain
// scrollable block. Scrolling (pgup/pgdn, mouse wheel) is handled by the
// underlying viewport.Model exactly as before, regardless of body type.
type responseScreen struct {
	vp    viewport.Model
	title string

	headerBlock string // pre-rendered: status line + divider + Headers block
	headerLines int     // line count of headerBlock, to map cursor -> absolute line

	isJSON bool
	root   *jsonview.Node
	lines  []jsonview.Line
	cursor int

	rawBody   string // original response body, used for copy/editor commands
	plainBody string // set when the body isn't JSON (or is empty)

	spinner spinner.Model
	running bool // true while a request is in flight; drives the spinner view

	statusMsg string // transient one-line feedback (e.g. "copied!")
}

func newResponseScreen() responseScreen {
	return responseScreen{
		vp:      viewport.New(0, 0),
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle)),
	}
}

func (s *responseScreen) setSize(w, h int) {
	s.vp.Width = w
	// Reserve 4 rows for this screen's own chrome: title line + blank line
	// (2 rows above the viewport in View()) and the "\n" separator + hints
	// bar (2 rows below). Without this the viewport thinks it's taller than
	// its visible area, throwing off scroll offsets and click-to-line math.
	s.vp.Height = h - 4
}

// showRunning marks the screen as waiting on an in-flight request and
// starts the loading spinner. The returned cmd must be scheduled (e.g. via
// tea.Batch alongside the command that actually performs the request) or
// the spinner never animates.
func (s *responseScreen) showRunning(name string) tea.Cmd {
	s.title = "Running " + name
	s.running = true
	s.vp.SetContent("")
	s.vp.GotoTop()
	return s.spinner.Tick
}

func (s *responseScreen) showError(name string, err error) {
	s.title = name
	s.running = false
	s.vp.SetContent(errorStyle.Render("error: " + err.Error()))
	s.vp.GotoTop()
}

// showResult populates the screen from a completed response, parsing the
// body as JSON when possible.
func (s *responseScreen) showResult(name string, resp httpx.Response) {
	s.title = name
	s.running = false
	s.headerBlock = renderHeaderBlock(resp)
	s.headerLines = strings.Count(s.headerBlock, "\n")

	s.isJSON = false
	s.root = nil
	s.lines = nil
	s.rawBody = resp.Body
	s.plainBody = ""
	s.cursor = 0

	if strings.TrimSpace(resp.Body) != "" {
		if root, err := jsonview.Parse([]byte(resp.Body)); err == nil {
			s.isJSON = true
			s.root = root
			s.lines = jsonview.Flatten(root)
		} else {
			s.plainBody = resp.Body
		}
	}

	s.render()
	s.vp.GotoTop()
}

// renderHeaderBlock renders the status line (colored by status class) and
// a labeled, sorted headers block. Static per response — computed once in
// showResult, not on every keypress.
func renderHeaderBlock(resp httpx.Response) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\n",
		statusStyle(resp.StatusCode).Render(resp.Status),
		subtleStyle.Render("("+resp.Duration.Round(1_000_000).String()+")"))
	b.WriteString(divider() + "\n")

	b.WriteString(labelStyle.Render("Headers") + "\n")
	keys := make([]string, 0, len(resp.Headers))
	for k := range resp.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		b.WriteString(subtleStyle.Render("(none)") + "\n")
	}
	for _, k := range keys {
		fmt.Fprintf(&b, "%s%s %s\n",
			jsonKeyStyle.Render(k), jsonPunctStyle.Render(":"), strings.Join(resp.Headers[k], ", "))
	}
	b.WriteString(divider() + "\n")
	return b.String()
}

func divider() string {
	return dividerStyle.Render(strings.Repeat("─", dividerWidth))
}

// render composes headerBlock plus a "Body" section (the JSON tree, plain
// text, or an empty-body placeholder) into the viewport's content.
func (s *responseScreen) render() {
	var b strings.Builder
	b.WriteString(s.headerBlock)
	b.WriteString(labelStyle.Render("Body") + "\n")
	b.WriteString(divider() + "\n")

	switch {
	case s.isJSON:
		for i, line := range s.lines {
			prefix := "  "
			if i == s.cursor {
				prefix = focusedStyle.Render("▸ ")
			}
			b.WriteString(prefix + renderJSONLine(line) + "\n")
		}
	case s.plainBody != "":
		b.WriteString(s.plainBody)
	default:
		b.WriteString(subtleStyle.Render("(empty body)"))
	}

	s.vp.SetContent(b.String())
}

// renderJSONLine renders one tree line: indentation, an optional "key: "
// prefix, the value/bracket for this line's Kind, and a trailing comma
// when appropriate.
func renderJSONLine(line jsonview.Line) string {
	var b strings.Builder
	b.WriteString(strings.Repeat("  ", line.Depth))

	// The key prefix belongs on the opening line only — a LineClose is
	// just the bare closing bracket, never "key: }".
	if line.Kind != jsonview.LineClose && line.Node.Key != "" {
		keyLit, _ := json.Marshal(line.Node.Key)
		b.WriteString(jsonKeyStyle.Render(string(keyLit)))
		b.WriteString(jsonPunctStyle.Render(": "))
	}

	switch line.Kind {
	case jsonview.LineOpen:
		b.WriteString(jsonPunctStyle.Render(bracket(line.Node.Kind, true)))
	case jsonview.LineClose:
		b.WriteString(jsonPunctStyle.Render(bracket(line.Node.Kind, false)))
	case jsonview.LineValue:
		b.WriteString(renderJSONValue(line.Node))
	}

	// A trailing comma belongs after the value is fully written — for a
	// container that's the LineClose, never the LineOpen (which would
	// otherwise show "key: [," before any elements are rendered).
	if line.Kind != jsonview.LineOpen && !line.IsLast {
		b.WriteString(jsonPunctStyle.Render(","))
	}
	return b.String()
}

func bracket(kind jsonview.Kind, open bool) string {
	switch {
	case kind == jsonview.KindArray && open:
		return "["
	case kind == jsonview.KindArray && !open:
		return "]"
	case open:
		return "{"
	default:
		return "}"
	}
}

// renderJSONValue renders a scalar, an empty container ("{}"/"[]"), or a
// collapsed non-empty container ("{…3}"/"[…5]").
func renderJSONValue(n *jsonview.Node) string {
	switch n.Kind {
	case jsonview.KindString:
		lit, _ := json.Marshal(n.Scalar)
		return jsonStringStyle.Render(string(lit))
	case jsonview.KindNumber:
		return jsonNumberStyle.Render(n.Scalar)
	case jsonview.KindBool:
		return jsonBoolStyle.Render(n.Scalar)
	case jsonview.KindNull:
		return jsonNullStyle.Render("null")
	case jsonview.KindObject, jsonview.KindArray:
		open, close := bracket(n.Kind, true), bracket(n.Kind, false)
		if len(n.Children) == 0 {
			return jsonPunctStyle.Render(open + close)
		}
		return jsonPunctStyle.Render(fmt.Sprintf("%s…%d%s", open, len(n.Children), close))
	default:
		return ""
	}
}

// ensureCursorVisible scrolls the viewport just enough to keep the cursor
// line on screen.
func (s *responseScreen) ensureCursorVisible() {
	if !s.isJSON {
		return
	}
	abs := s.headerLines + 2 + s.cursor // +2 for the "Body" label and divider
	if abs < s.vp.YOffset {
		s.vp.SetYOffset(abs)
	} else if s.vp.Height > 0 && abs >= s.vp.YOffset+s.vp.Height {
		s.vp.SetYOffset(abs - s.vp.Height + 1)
	}
}

// lineAtY maps an absolute terminal row (as reported on a tea.MouseEvent)
// to an index into s.lines, honoring the current scroll offset. Reports
// false when the click falls outside the body's visible JSON lines (e.g.
// on the header/status block above, or below the last rendered line).
func (s *responseScreen) lineAtY(y int) (int, bool) {
	rel := y - responseViewportTop
	if rel < 0 {
		return 0, false
	}
	// s.headerLines + 2 accounts for the header block plus the "Body"
	// label and its divider, matching ensureCursorVisible's math.
	idx := s.vp.YOffset + rel - (s.headerLines + 2)
	if idx < 0 || idx >= len(s.lines) {
		return 0, false
	}
	return idx, true
}

// toggleFold collapses or expands the container at the cursor. A no-op on
// scalars, empty containers, or when the body isn't JSON.
func (s *responseScreen) toggleFold() {
	if !s.isJSON || s.cursor < 0 || s.cursor >= len(s.lines) {
		return
	}
	line := s.lines[s.cursor]
	n := line.Node
	switch line.Kind {
	case jsonview.LineOpen, jsonview.LineClose:
		// Only reachable when the node is currently expanded (Flatten
		// only emits Open/Close for expanded, non-empty containers).
		n.Collapsed = true
	case jsonview.LineValue:
		if (n.Kind == jsonview.KindObject || n.Kind == jsonview.KindArray) && len(n.Children) > 0 {
			// Only reachable when currently collapsed, for the same reason.
			n.Collapsed = false
		} else {
			return // scalar or empty container: nothing to fold
		}
	}

	s.lines = jsonview.Flatten(s.root)
	if s.cursor >= len(s.lines) {
		s.cursor = len(s.lines) - 1
	}
	s.render()
	s.ensureCursorVisible()
}

// editorFinishedMsg is sent back to the update loop after the external editor
// process exits (successfully or not).
type editorFinishedMsg struct{ err error }

// prettyBody returns the response body formatted for copying/editing. JSON
// bodies are pretty-printed; everything else is returned as-is.
func (s *responseScreen) prettyBody() string {
	if s.isJSON && strings.TrimSpace(s.rawBody) != "" {
		var buf bytes.Buffer
		if err := json.Indent(&buf, []byte(s.rawBody), "", "  "); err == nil {
			return buf.String()
		}
	}
	return s.rawBody
}

// copyBodyCmd copies the (pretty-printed) response body to the system
// clipboard and returns a Cmd that delivers a statusMsg back to the screen.
func (s *responseScreen) copyBodyCmd() tea.Cmd {
	body := s.prettyBody()
	return func() tea.Msg {
		if err := clipboard.WriteAll(body); err != nil {
			return responseStatusMsg("copy failed: " + err.Error())
		}
		return responseStatusMsg("copied!")
	}
}

// responseStatusMsg carries a one-line transient status update for the
// response screen (e.g. "copied!" after a clipboard write).
type responseStatusMsg string

// openEditorCmd writes the body to a temp file and opens it in the user's
// preferred editor ($VISUAL, then $EDITOR, then vi).
func (s *responseScreen) openEditorCmd() tea.Cmd {
	body := s.prettyBody()
	ext := ".txt"
	if s.isJSON {
		ext = ".json"
	}
	f, err := os.CreateTemp("", "terman-response-*"+ext)
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err} }
	}
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return func() tea.Msg { return editorFinishedMsg{err} }
	}
	f.Close()

	editorBin := os.Getenv("VISUAL")
	if editorBin == "" {
		editorBin = os.Getenv("EDITOR")
	}
	if editorBin == "" {
		editorBin = "vi"
	}
	c := exec.Command(editorBin, f.Name())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err}
	})
}

func (s responseScreen) Update(msg tea.Msg) (responseScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !s.running {
			return s, nil // stale tick from a request that already finished; drop it
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd
	case responseStatusMsg:
		s.statusMsg = string(msg)
		return s, nil
	case editorFinishedMsg:
		// Editor exited; nothing to update in the model (the file was
		// read-only from the user's perspective). Errors are silently
		// dropped — the user already saw them in their terminal.
		return s, nil
	case tea.MouseMsg:
		if s.isJSON && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if idx, ok := s.lineAtY(msg.Y); ok {
				s.cursor = idx
				s.render()
				s.ensureCursorVisible()
				return s, nil
			}
		}
	case tea.KeyMsg:
		if !s.running && s.rawBody != "" {
			switch msg.String() {
			case "c":
				s.statusMsg = "" // clear before async result arrives
				return s, s.copyBodyCmd()
			case "e":
				return s, s.openEditorCmd()
			}
		}
		if s.isJSON {
			switch msg.String() {
			case "up":
				if s.cursor > 0 {
					s.cursor--
					s.render()
					s.ensureCursorVisible()
				}
				return s, nil
			case "down":
				if s.cursor < len(s.lines)-1 {
					s.cursor++
					s.render()
					s.ensureCursorVisible()
				}
				return s, nil
			case "enter", " ":
				s.toggleFold()
				return s, nil
			}
		}
	}
	var cmd tea.Cmd
	s.vp, cmd = s.vp.Update(msg)
	return s, cmd
}

func (s responseScreen) View() string {
	if s.running {
		return titleStyle.Render(s.title) + "\n\n" +
			s.spinner.View() + " " + subtleStyle.Render("sending request…") + "\n\n" +
			renderHints(keyHint{"esc", "back"})
	}
	hints := []keyHint{{"↑/↓ pgup/pgdn", "scroll"}}
	if s.isJSON {
		hints = append(hints, keyHint{"click", "select"}, keyHint{"enter/space", "fold"})
	}
	if s.rawBody != "" {
		hints = append(hints, keyHint{"c", "copy"}, keyHint{"e", "editor"})
	}
	hints = append(hints, keyHint{"esc", "back"})
	hintsLine := renderHints(hints...)
	if s.statusMsg != "" {
		hintsLine = subtleStyle.Render(s.statusMsg) + "  " + hintsLine
	}
	return titleStyle.Render(s.title) + "\n\n" + s.vp.View() + "\n" + hintsLine
}

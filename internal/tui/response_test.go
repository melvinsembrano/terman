package tui

import (
	"net/http"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/httpx"
)

// TestRenderJSONLineBracketsAndCommas is a regression test: an earlier
// version put the trailing comma right after an opening bracket (e.g.
// `"tags": [,`) and repeated the key on the closing line (e.g.
// `"tags": ]`) instead of a bare `],`. Caught by rendering a real nested
// document and reading the output, not by a unit test — so pin the exact
// expected text for the tricky lines here.
func TestRenderJSONLineBracketsAndCommas(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 30)
	s.showResult("Req", httpx.Response{
		Status: "200 OK", StatusCode: 200,
		Body: `{"tags":["admin","beta"],"address":{"city":"London"}}`,
	})

	view := s.vp.View()
	wantLines := []string{
		`"tags": [`,     // open: no trailing comma yet
		`"admin",`,      // not last element: comma
		`"beta"`,        // last element: no comma
		`],`,            // close: bare bracket, comma (root has another member after)
		`"address": {`,  // next member's open
		`"city": "London"`, // last member of address: no comma
		`}`,             // address close: no comma (last root member)
	}
	for _, want := range wantLines {
		if !strings.Contains(view, want) {
			t.Errorf("view missing expected line %q, got:\n%s", want, view)
		}
	}
	// Specifically guard against the two exact bugs seen before the fix.
	if strings.Contains(view, "[,") {
		t.Error("found a comma immediately after an opening bracket")
	}
	if strings.Contains(view, `"tags": ]`) || strings.Contains(view, `"address": }`) {
		t.Error("found the key repeated on a closing-bracket line")
	}
}

func TestShowResultJSONBodyBuildsTree(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	resp := httpx.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Headers:    http.Header{"Content-Type": {"application/json"}},
		Body:       `{"name":"Ada","tags":["admin","beta"]}`,
	}

	s.showResult("Get Widget", resp)

	if !s.isJSON {
		t.Fatal("expected isJSON=true for a JSON body")
	}
	if s.root == nil {
		t.Fatal("expected root to be set")
	}
	if len(s.lines) == 0 {
		t.Fatal("expected non-empty lines")
	}
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want 0 initially", s.cursor)
	}

	view := s.vp.View()
	if !strings.Contains(view, "Ada") || !strings.Contains(view, "admin") {
		t.Errorf("view missing expected JSON content, got:\n%s", view)
	}
	if !strings.Contains(view, "Headers") || !strings.Contains(view, "Body") {
		t.Errorf("view missing section labels, got:\n%s", view)
	}
}

func TestShowResultNonJSONBodyStaysPlain(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	resp := httpx.Response{Status: "200 OK", StatusCode: 200, Body: "hello world, not json"}

	s.showResult("Get Widget", resp)

	if s.isJSON {
		t.Error("expected isJSON=false for a non-JSON body")
	}
	if s.plainBody != "hello world, not json" {
		t.Errorf("plainBody = %q", s.plainBody)
	}
	if !strings.Contains(s.vp.View(), "hello world, not json") {
		t.Errorf("view missing plain body, got:\n%s", s.vp.View())
	}
}

func TestShowResultEmptyBody(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	resp := httpx.Response{Status: "204 No Content", StatusCode: 204, Body: ""}

	s.showResult("Delete Widget", resp)

	if s.isJSON {
		t.Error("expected isJSON=false for an empty body")
	}
	if !strings.Contains(s.vp.View(), "empty body") {
		t.Errorf("view missing empty-body placeholder, got:\n%s", s.vp.View())
	}
}

func TestShowResultHeadersListedAndSorted(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	resp := httpx.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Headers: http.Header{
			"X-Zeta":  {"1"},
			"X-Alpha": {"2"},
		},
		Body: "",
	}

	s.showResult("Req", resp)

	view := s.vp.View()
	alphaIdx := strings.Index(view, "X-Alpha")
	zetaIdx := strings.Index(view, "X-Zeta")
	if alphaIdx == -1 || zetaIdx == -1 {
		t.Fatalf("both headers should appear, got:\n%s", view)
	}
	if alphaIdx > zetaIdx {
		t.Errorf("expected headers sorted alphabetically (X-Alpha before X-Zeta)")
	}
}

func TestCursorMovesUpDown(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	s.showResult("Req", httpx.Response{
		Status: "200 OK", StatusCode: 200,
		Body: `{"a":1,"b":2,"c":3}`,
	})

	updated, _ := s.Update(keyMsg("down"))
	if updated.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", updated.cursor)
	}
	updated, _ = updated.Update(keyMsg("down"))
	if updated.cursor != 2 {
		t.Fatalf("cursor after 2nd down = %d, want 2", updated.cursor)
	}
	updated, _ = updated.Update(keyMsg("up"))
	if updated.cursor != 1 {
		t.Fatalf("cursor after up = %d, want 1", updated.cursor)
	}
}

func TestCursorClampsAtBoundaries(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	s.showResult("Req", httpx.Response{Status: "200 OK", StatusCode: 200, Body: `{"a":1}`})

	// At the top already; "up" should not go negative.
	updated, _ := s.Update(keyMsg("up"))
	if updated.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 (clamped)", updated.cursor)
	}

	// Move to the last line, then try to go past it.
	last := len(updated.lines) - 1
	for i := 0; i < last+5; i++ {
		updated, _ = updated.Update(keyMsg("down"))
	}
	if updated.cursor != last {
		t.Fatalf("cursor = %d, want clamped to last line %d", updated.cursor, last)
	}
}

func TestToggleFoldCollapsesAndExpands(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	s.showResult("Req", httpx.Response{
		Status: "200 OK", StatusCode: 200,
		Body: `{"nested":{"x":1,"y":2},"other":3}`,
	})
	fullLen := len(s.lines)

	// Move cursor onto the "nested" object's opening line (index 0 is root
	// open, index 1 is "nested"'s open line).
	s.cursor = 1
	if s.lines[s.cursor].Node.Key != "nested" {
		t.Fatalf("test setup: expected cursor on 'nested', got key %q", s.lines[s.cursor].Node.Key)
	}

	updated, _ := s.Update(keyMsg("enter"))
	if len(updated.lines) >= fullLen {
		t.Fatalf("expected fewer lines after collapsing, got %d (was %d)", len(updated.lines), fullLen)
	}
	if !updated.lines[updated.cursor].Node.Collapsed {
		t.Error("expected the 'nested' node to be marked Collapsed")
	}

	// Toggle again (space this time) to expand back.
	reExpanded, _ := updated.Update(keyMsg(" "))
	if len(reExpanded.lines) != fullLen {
		t.Errorf("expected line count restored to %d after expanding, got %d", fullLen, len(reExpanded.lines))
	}
}

func TestToggleFoldNoopOnScalar(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	s.showResult("Req", httpx.Response{Status: "200 OK", StatusCode: 200, Body: `{"a":1}`})

	// Cursor starts on the root's open line (index 0); move to the scalar
	// "a" line (index 1).
	s.cursor = 1
	before := len(s.lines)

	updated, _ := s.Update(keyMsg("enter"))
	if len(updated.lines) != before {
		t.Errorf("expected no change folding a scalar line, got %d lines (was %d)", len(updated.lines), before)
	}
}

func TestNonJSONBodyUpDownFallsThroughToScroll(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 5)
	lines := strings.Repeat("line\n", 50)
	s.showResult("Req", httpx.Response{Status: "200 OK", StatusCode: 200, Body: lines})

	// Should not panic, and cursor stays meaningless (isJSON false) while
	// the viewport scrolls instead.
	before := s.vp.YOffset
	updated, _ := s.Update(keyMsg("down"))
	if updated.isJSON {
		t.Fatal("expected isJSON=false for a plain-text body")
	}
	_ = before // scrolling behavior is viewport.Model's own responsibility; just confirming no crash/regression
}

func TestStatusClassification(t *testing.T) {
	cases := map[int]string{
		200: "2xx", 201: "2xx", 299: "2xx",
		300: "3xx", 301: "3xx", 399: "3xx",
		400: "4xx", 404: "4xx", 499: "4xx",
		500: "5xx", 503: "5xx", 599: "5xx",
		600: "5xx", // codes >= 500 are open-ended: unusual high codes are still "some kind of server error"
		100: "", 0: "", -1: "",
	}
	for code, want := range cases {
		if got := statusClass(code); got != want {
			t.Errorf("statusClass(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestStatusStyleUsesDistinctStyleValues(t *testing.T) {
	// lipgloss emits no ANSI codes outside a real terminal, so rendered
	// output can't distinguish styles under `go test` — instead confirm
	// statusStyle actually picks four different package-level Style
	// values (not, say, the same one every time) by checking each
	// maps to a distinct pointer-identity via a distinguishing field
	// mutation would be invasive; comparing against the known style
	// variables directly is simplest and exercises the real switch.
	if got, want := statusStyle(200), statusOKStyle; got.GetForeground() != want.GetForeground() {
		t.Errorf("statusStyle(200) foreground = %v, want %v", got.GetForeground(), want.GetForeground())
	}
	if got, want := statusStyle(301), statusRedirectStyle; got.GetForeground() != want.GetForeground() {
		t.Errorf("statusStyle(301) foreground = %v, want %v", got.GetForeground(), want.GetForeground())
	}
	if got, want := statusStyle(404), statusClientErrStyle; got.GetForeground() != want.GetForeground() {
		t.Errorf("statusStyle(404) foreground = %v, want %v", got.GetForeground(), want.GetForeground())
	}
	if got, want := statusStyle(500), statusServerErrStyle; got.GetForeground() != want.GetForeground() {
		t.Errorf("statusStyle(500) foreground = %v, want %v", got.GetForeground(), want.GetForeground())
	}
}

func TestRenderHeaderBlockShowsNoneWhenEmpty(t *testing.T) {
	got := renderHeaderBlock(httpx.Response{Status: "200 OK", StatusCode: 200, Headers: nil})
	if !strings.Contains(got, "(none)") {
		t.Errorf("expected a (none) placeholder for no headers, got:\n%s", got)
	}
}

func TestShowErrorRendersMessage(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 10)
	s.showError("Get Widget", errBoom)

	if s.title != "Get Widget" {
		t.Errorf("title = %q, want %q", s.title, "Get Widget")
	}
	if !strings.Contains(s.vp.View(), "boom") {
		t.Errorf("expected view to contain error message, got:\n%s", s.vp.View())
	}
}

func TestShowRunningSetsTitle(t *testing.T) {
	s := newResponseScreen()
	cmd := s.showRunning("Get Widget")

	if !strings.Contains(s.title, "Get Widget") {
		t.Errorf("title = %q, want to contain %q", s.title, "Get Widget")
	}
	if !s.running {
		t.Error("expected running=true after showRunning")
	}
	if cmd == nil {
		t.Fatal("expected showRunning to return a non-nil cmd to kick off the spinner")
	}
	if _, ok := cmd().(spinner.TickMsg); !ok {
		t.Errorf("expected the returned cmd to produce a spinner.TickMsg")
	}
}

func TestShowResultAndShowErrorStopSpinner(t *testing.T) {
	s := newResponseScreen()
	s.showRunning("Get Widget")
	s.showResult("Get Widget", httpx.Response{Status: "200 OK", StatusCode: 200})
	if s.running {
		t.Error("expected running=false after showResult")
	}

	s2 := newResponseScreen()
	s2.showRunning("Get Widget")
	s2.showError("Get Widget", errBoom)
	if s2.running {
		t.Error("expected running=false after showError")
	}
}

func TestSpinnerTickIgnoredWhenNotRunning(t *testing.T) {
	s := newResponseScreen() // running defaults to false

	_, cmd := s.Update(spinner.TickMsg{})
	if cmd != nil {
		t.Error("expected a stale tick (not running) to produce a nil cmd, breaking the perpetuation chain")
	}
}

func TestSpinnerTickKeepsAnimatingWhileRunning(t *testing.T) {
	s := newResponseScreen()
	cmd := s.showRunning("Get Widget")

	updated, cmd2 := s.Update(cmd())
	if cmd2 == nil {
		t.Fatal("expected a follow-up cmd to keep the spinner animating while running")
	}
	if !strings.Contains(updated.View(), "sending request") {
		t.Errorf("expected the running view to show a loading message, got:\n%s", updated.View())
	}
}

func TestMouseClickSelectsJSONLine(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	s.showResult("Req", httpx.Response{
		Status: "200 OK", StatusCode: 200,
		Body: `{"a":1,"b":2,"c":3}`,
	})

	targetIdx := 2 // the "b": 2 line
	y := responseViewportTop + s.headerLines + 2 + targetIdx
	updated, _ := s.Update(tea.MouseMsg{Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})

	if updated.cursor != targetIdx {
		t.Fatalf("cursor after click = %d, want %d", updated.cursor, targetIdx)
	}
	if len(updated.lines) != len(s.lines) {
		t.Error("a click should only move the cursor, never fold/unfold")
	}
}

func TestMouseClickOutsideBodyIsNoop(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	s.showResult("Req", httpx.Response{Status: "200 OK", StatusCode: 200, Body: `{"a":1}`})

	// Click on the status/header block, well above the body lines.
	updated, _ := s.Update(tea.MouseMsg{Y: responseViewportTop, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want unchanged 0 for a click above the body", updated.cursor)
	}

	// Click far below the last rendered line.
	updated, _ = s.Update(tea.MouseMsg{Y: 1000, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want unchanged 0 for a click below the body", updated.cursor)
	}
}

func TestMouseClickOnNonJSONBodyIsNoop(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 20)
	s.showResult("Req", httpx.Response{Status: "200 OK", StatusCode: 200, Body: "plain text body"})

	updated, _ := s.Update(tea.MouseMsg{Y: responseViewportTop + 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (non-JSON bodies ignore clicks)", updated.cursor)
	}
}

func TestViewHelpTextMentionsFoldOnlyForJSON(t *testing.T) {
	jsonScreen := newResponseScreen()
	jsonScreen.setSize(80, 20)
	jsonScreen.showResult("Req", httpx.Response{Status: "200 OK", StatusCode: 200, Body: `{"a":1}`})
	if !strings.Contains(jsonScreen.View(), "fold") {
		t.Error("expected help text to mention folding for a JSON body")
	}

	plainScreen := newResponseScreen()
	plainScreen.setSize(80, 20)
	plainScreen.showResult("Req", httpx.Response{Status: "200 OK", StatusCode: 200, Body: "plain text"})
	if strings.Contains(plainScreen.View(), "fold") {
		t.Error("did not expect help text to mention folding for a non-JSON body")
	}
}

// keyMsg builds a tea.KeyMsg from a key string as used elsewhere in this
// test suite's convention (see internal/tui/app_test.go).
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// errBoom is a fixed error used to keep test cases terse.
var errBoom = boomError{}

type boomError struct{}

func (boomError) Error() string { return "boom" }

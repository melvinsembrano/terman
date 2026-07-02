package tui

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/melvinsembrano/terman/internal/httpx"
)

func TestFormatResponseWithHeadersAndBody(t *testing.T) {
	resp := httpx.Response{
		Status:   "200 OK",
		Duration: 123 * time.Millisecond,
		Headers:  http.Header{"Content-Type": {"application/json"}},
		Body:     `{"ok":true}`,
	}

	got := formatResponse(resp)

	if !strings.HasPrefix(got, "200 OK  (123ms)\n\n") {
		t.Errorf("formatResponse status line missing/wrong, got:\n%s", got)
	}
	if !strings.Contains(got, "Content-Type: application/json\n") {
		t.Errorf("formatResponse missing headers block, got:\n%s", got)
	}
	if !strings.HasSuffix(got, `{"ok":true}`) {
		t.Errorf("formatResponse missing body, got:\n%s", got)
	}
}

func TestFormatResponseWithoutHeaders(t *testing.T) {
	resp := httpx.Response{Status: "204 No Content", Body: ""}

	got := formatResponse(resp)
	want := "204 No Content  (0s)\n\n"
	if got != want {
		t.Errorf("formatResponse() = %q, want %q", got, want)
	}
}

func TestShowResultSetsTitleAndContent(t *testing.T) {
	s := newResponseScreen()
	s.setSize(80, 10)
	resp := httpx.Response{Status: "200 OK", Body: "hi"}

	s.showResult("Get Widget", resp)

	if s.title != "Get Widget" {
		t.Errorf("title = %q, want %q", s.title, "Get Widget")
	}
	if !strings.Contains(s.vp.View(), "hi") {
		t.Errorf("expected viewport view to include response body, got:\n%s", s.vp.View())
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
	s.showRunning("Get Widget")

	if !strings.Contains(s.title, "Get Widget") {
		t.Errorf("title = %q, want to contain %q", s.title, "Get Widget")
	}
}

// errBoom is a fixed error used to keep test cases terse.
var errBoom = boomError{}

type boomError struct{}

func (boomError) Error() string { return "boom" }

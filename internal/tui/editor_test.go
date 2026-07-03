package tui

import (
	"testing"

	"github.com/melvinsembrano/terman/internal/model"
)

func TestMethodIndex(t *testing.T) {
	cases := map[string]int{
		"GET":    0,
		"post":   1,
		"Put":    2,
		"PATCH":  3,
		"delete": 4,
		"BOGUS":  0,
		"":       0,
	}
	for in, want := range cases {
		if got := methodIndex(in); got != want {
			t.Errorf("methodIndex(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestEditorScreenToRequestParsesHeaders(t *testing.T) {
	s := newEditorScreen()
	s.name.SetValue("  My Request  ")
	s.url.SetValue("  https://example.com  ")
	s.methodIdx = methodIndex("POST")
	s.headers.SetValue("Content-Type: application/json\n\nbad-line-no-colon\n  X-Trace : abc123  \n")
	s.body.SetValue(`{"a":1}`)

	req := s.toRequest()

	if req.Name != "My Request" {
		t.Errorf("Name = %q, want %q", req.Name, "My Request")
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want %q", req.Method, "POST")
	}
	if req.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", req.URL, "https://example.com")
	}
	if len(req.Headers) != 2 {
		t.Fatalf("Headers = %v, want 2 entries", req.Headers)
	}
	if req.Headers["Content-Type"] != "application/json" {
		t.Errorf(`Headers["Content-Type"] = %q, want %q`, req.Headers["Content-Type"], "application/json")
	}
	if req.Headers["X-Trace"] != "abc123" {
		t.Errorf(`Headers["X-Trace"] = %q, want %q`, req.Headers["X-Trace"], "abc123")
	}
	if req.Body != `{"a":1}` {
		t.Errorf("Body = %q, want %q", req.Body, `{"a":1}`)
	}
}

func TestEditorScreenToRequestNoHeadersIsNil(t *testing.T) {
	s := newEditorScreen()
	s.name.SetValue("Bare")
	s.headers.SetValue("\n  \n")

	req := s.toRequest()
	if req.Headers != nil {
		t.Errorf("Headers = %v, want nil", req.Headers)
	}
}

func TestEditorScreenLoadRequestRoundTrip(t *testing.T) {
	original := model.Request{
		Name:    "Get Widget",
		Method:  "PUT",
		URL:     "https://api.example.com/widgets/1",
		Headers: map[string]string{"Authorization": "Bearer tok", "X-Env": "dev"},
		Body:    "payload",
	}

	s := newEditorScreen()
	s.loadRequest(original)

	if s.prevName != original.Name {
		t.Errorf("prevName = %q, want %q", s.prevName, original.Name)
	}
	if s.focus != focusName {
		t.Errorf("focus = %d, want focusName (%d)", s.focus, focusName)
	}

	got := s.toRequest()
	if got.Name != original.Name || got.Method != original.Method || got.URL != original.URL || got.Body != original.Body {
		t.Errorf("round-tripped request = %+v, want %+v", got, original)
	}
	if len(got.Headers) != len(original.Headers) {
		t.Fatalf("Headers = %v, want %v", got.Headers, original.Headers)
	}
	for k, v := range original.Headers {
		if got.Headers[k] != v {
			t.Errorf("Headers[%q] = %q, want %q", k, got.Headers[k], v)
		}
	}
}

func TestEditorScreenSetFocusWraps(t *testing.T) {
	s := newEditorScreen()

	s.setFocus(-1)
	if s.focus != focusBody {
		t.Errorf("setFocus(-1) = %d, want focusBody (%d)", s.focus, focusBody)
	}

	s.setFocus(focusCount)
	if s.focus != focusMethod {
		t.Errorf("setFocus(focusCount) = %d, want focusMethod (%d)", s.focus, focusMethod)
	}

	s.setFocus(focusHeaders)
	if s.focus != focusHeaders {
		t.Errorf("setFocus(focusHeaders) = %d, want focusHeaders (%d)", s.focus, focusHeaders)
	}
	if !s.headers.Focused() {
		t.Errorf("expected headers textarea to be focused")
	}
	if s.name.Focused() {
		t.Errorf("expected name textinput to be blurred")
	}
}

func TestEditorScreenLoadNewResetsForm(t *testing.T) {
	s := newEditorScreen()
	s.loadRequest(model.Request{Name: "Old", Group: "auth", Method: "POST", URL: "https://old.example.com"})

	s.loadNew("")

	if s.prevName != "" {
		t.Errorf("prevName after loadNew = %q, want empty", s.prevName)
	}
	if s.name.Value() != "" {
		t.Errorf("name value after loadNew = %q, want empty", s.name.Value())
	}
	if s.group.Value() != "" {
		t.Errorf("group value after loadNew(\"\") = %q, want empty", s.group.Value())
	}
	if s.methodIdx != methodIndex("GET") {
		t.Errorf("methodIdx after loadNew = %d, want %d", s.methodIdx, methodIndex("GET"))
	}
}

func TestEditorScreenLoadNewDefaultsGroup(t *testing.T) {
	s := newEditorScreen()
	s.loadNew("auth/oauth")

	if s.group.Value() != "auth/oauth" {
		t.Errorf("group value after loadNew(\"auth/oauth\") = %q, want %q", s.group.Value(), "auth/oauth")
	}
	req := s.toRequest()
	if req.Group != "auth/oauth" {
		t.Errorf("toRequest().Group = %q, want %q", req.Group, "auth/oauth")
	}
}

func TestEditorScreenLoadRequestRoundTripsGroup(t *testing.T) {
	original := model.Request{Name: "Login", Group: "auth", Method: "POST", URL: "https://example.com/login"}

	s := newEditorScreen()
	s.loadRequest(original)

	if s.prevGroup != "auth" {
		t.Errorf("prevGroup = %q, want %q", s.prevGroup, "auth")
	}
	if got := s.toRequest().Group; got != "auth" {
		t.Errorf("toRequest().Group = %q, want %q", got, "auth")
	}
}

func TestNormalizeGroupInput(t *testing.T) {
	cases := map[string]string{
		"":               "",
		"  ":             "",
		"auth":           "auth",
		"/auth/":         "auth",
		"auth//oauth":    "auth/oauth",
		" auth / oauth ": "auth/oauth",
		`auth\oauth`:     "auth/oauth",
	}
	for in, want := range cases {
		if got := normalizeGroupInput(in); got != want {
			t.Errorf("normalizeGroupInput(%q) = %q, want %q", in, got, want)
		}
	}
}

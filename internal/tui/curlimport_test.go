package tui

import "testing"

func TestCurlImportParseSuccess(t *testing.T) {
	s := newCurlImportScreen()
	s.name.SetValue("  Get Users  ")
	s.cmd.SetValue(`curl 'https://example.com/users' -H 'Accept: application/json'`)

	req, err := s.parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Name != "Get Users" {
		t.Errorf("Name = %q, want %q (trimmed)", req.Name, "Get Users")
	}
	if req.Method != "GET" {
		t.Errorf("Method = %q, want GET", req.Method)
	}
	if req.URL != "https://example.com/users" {
		t.Errorf("URL = %q", req.URL)
	}
	if req.Headers["Accept"] != "application/json" {
		t.Errorf("Headers[Accept] = %q", req.Headers["Accept"])
	}
}

func TestCurlImportParseNameRequired(t *testing.T) {
	s := newCurlImportScreen()
	s.cmd.SetValue(`curl 'https://example.com'`)

	if _, err := s.parse(); err == nil {
		t.Error("expected an error when name is empty")
	}
}

func TestCurlImportParsePropagatesCurlError(t *testing.T) {
	s := newCurlImportScreen()
	s.name.SetValue("Bad Command")
	s.cmd.SetValue(`curl -X GET`) // no URL

	if _, err := s.parse(); err == nil {
		t.Error("expected an error for a curl command with no URL")
	}
}

func TestCurlImportSetFocusToggles(t *testing.T) {
	s := newCurlImportScreen()
	s.setFocus(curlFocusName)
	if !s.name.Focused() || s.cmd.Focused() {
		t.Errorf("setFocus(curlFocusName): name.Focused=%v cmd.Focused=%v", s.name.Focused(), s.cmd.Focused())
	}

	s.setFocus(curlFocusCmd)
	if s.name.Focused() || !s.cmd.Focused() {
		t.Errorf("setFocus(curlFocusCmd): name.Focused=%v cmd.Focused=%v", s.name.Focused(), s.cmd.Focused())
	}
}

func TestCurlImportLoadNewResetsForm(t *testing.T) {
	s := newCurlImportScreen()
	s.name.SetValue("old")
	s.cmd.SetValue("curl https://old.example.com")
	s.err = "boom"

	s.loadNew()

	if s.name.Value() != "" {
		t.Errorf("name value after loadNew = %q, want empty", s.name.Value())
	}
	if s.cmd.Value() != "" {
		t.Errorf("cmd value after loadNew = %q, want empty", s.cmd.Value())
	}
	if s.err != "" {
		t.Errorf("err after loadNew = %q, want empty", s.err)
	}
	if s.focus != curlFocusName {
		t.Errorf("focus after loadNew = %d, want curlFocusName (%d)", s.focus, curlFocusName)
	}
}

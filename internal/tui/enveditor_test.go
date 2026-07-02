package tui

import (
	"testing"

	"github.com/melvinsembrano/terman/internal/model"
)

func TestEnvEditorToEnvironmentBuildsVars(t *testing.T) {
	s := newEnvEditorScreen()
	s.name.SetValue("  dev  ")
	s.pairs = []kvPair{
		{key: "base_url", value: "https://example.com"},
		{key: "  token  ", value: "abc"},
	}

	env := s.toEnvironment()
	if env.Name != "dev" {
		t.Errorf("Name = %q, want %q", env.Name, "dev")
	}
	if len(env.Vars) != 2 {
		t.Fatalf("Vars = %v, want 2 entries", env.Vars)
	}
	if env.Vars["base_url"] != "https://example.com" {
		t.Errorf(`Vars["base_url"] = %q`, env.Vars["base_url"])
	}
	if env.Vars["token"] != "abc" {
		t.Errorf(`Vars["token"] = %q, want %q (key should be trimmed)`, env.Vars["token"], "abc")
	}
}

func TestEnvEditorToEnvironmentSkipsBlankKeys(t *testing.T) {
	s := newEnvEditorScreen()
	s.name.SetValue("dev")
	s.pairs = []kvPair{
		{key: "", value: "ignored"},
		{key: "   ", value: "also ignored"},
		{key: "kept", value: "value"},
	}

	env := s.toEnvironment()
	if len(env.Vars) != 1 || env.Vars["kept"] != "value" {
		t.Errorf("Vars = %v, want only {kept: value}", env.Vars)
	}
}

func TestEnvEditorToEnvironmentNilWhenEmpty(t *testing.T) {
	s := newEnvEditorScreen()
	s.name.SetValue("dev")

	env := s.toEnvironment()
	if env.Vars != nil {
		t.Errorf("Vars = %v, want nil", env.Vars)
	}
}

func TestEnvEditorLoadEnvironmentRoundTrip(t *testing.T) {
	original := model.Environment{
		Name: "dev",
		Vars: map[string]string{"base_url": "https://example.com", "token": "abc"},
	}

	s := newEnvEditorScreen()
	s.loadEnvironment(original)

	if s.prevName != original.Name {
		t.Errorf("prevName = %q, want %q", s.prevName, original.Name)
	}
	if len(s.pairs) != 2 {
		t.Fatalf("pairs = %v, want 2 entries", s.pairs)
	}

	got := s.toEnvironment()
	if got.Name != original.Name {
		t.Errorf("Name = %q, want %q", got.Name, original.Name)
	}
	for k, v := range original.Vars {
		if got.Vars[k] != v {
			t.Errorf("Vars[%q] = %q, want %q", k, got.Vars[k], v)
		}
	}
}

func TestEnvEditorAddEditDeleteRow(t *testing.T) {
	s := newEnvEditorScreen()
	s.name.SetValue("dev")

	// Add a row via the modal flow.
	s.startAddRow()
	if !s.editing || s.editIdx != -1 {
		t.Fatalf("startAddRow: editing=%v editIdx=%d, want true/-1", s.editing, s.editIdx)
	}
	s.keyInput.SetValue("base_url")
	s.valInput.SetValue("https://example.com")
	s.commitRow()
	if s.editing {
		t.Errorf("expected modal closed after commitRow")
	}
	if len(s.pairs) != 1 || s.pairs[0].key != "base_url" || s.pairs[0].value != "https://example.com" {
		t.Fatalf("pairs after add = %v", s.pairs)
	}

	// Edit that row.
	s.selected = 0
	s.startEditRow()
	if !s.editing || s.editIdx != 0 {
		t.Fatalf("startEditRow: editing=%v editIdx=%d, want true/0", s.editing, s.editIdx)
	}
	s.valInput.SetValue("https://updated.example.com")
	s.commitRow()
	if s.pairs[0].value != "https://updated.example.com" {
		t.Errorf("pairs[0].value = %q, want updated value", s.pairs[0].value)
	}

	// Delete the row.
	s.deleteSelectedRow()
	if len(s.pairs) != 0 {
		t.Errorf("pairs after delete = %v, want empty", s.pairs)
	}
}

func TestEnvEditorStartEditRowOutOfRangeIsNoop(t *testing.T) {
	s := newEnvEditorScreen()
	s.selected = 5 // no rows exist
	s.startEditRow()
	if s.editing {
		t.Errorf("expected startEditRow to be a no-op with no rows, but editing=true")
	}
}

func TestEnvEditorDeleteSelectedRowOutOfRangeIsNoop(t *testing.T) {
	s := newEnvEditorScreen()
	s.selected = -1
	s.deleteSelectedRow() // should not panic
	if len(s.pairs) != 0 {
		t.Errorf("pairs = %v, want empty", s.pairs)
	}
}

func TestEnvEditorLoadNewResetsForm(t *testing.T) {
	s := newEnvEditorScreen()
	s.loadEnvironment(model.Environment{Name: "old", Vars: map[string]string{"a": "1"}})

	s.loadNew()

	if s.prevName != "" {
		t.Errorf("prevName after loadNew = %q, want empty", s.prevName)
	}
	if s.name.Value() != "" {
		t.Errorf("name value after loadNew = %q, want empty", s.name.Value())
	}
	if len(s.pairs) != 0 {
		t.Errorf("pairs after loadNew = %v, want empty", s.pairs)
	}
}

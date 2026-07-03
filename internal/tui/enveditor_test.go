package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestEnvEditorImportAddsNewKeys(t *testing.T) {
	s := newEnvEditorScreen()
	s.name.SetValue("dev")
	path := writeEnvFile(t, "BASE_URL=https://example.com\nTOKEN=abc\n")

	s.startImport()
	if !s.importing {
		t.Fatalf("expected importing=true after startImport")
	}
	s.pathInput.SetValue(path)
	s.commitImport()

	if s.importing {
		t.Errorf("expected modal closed after commitImport")
	}
	if s.importErr != "" {
		t.Errorf("importErr = %q, want empty", s.importErr)
	}
	if len(s.pairs) != 2 {
		t.Fatalf("pairs = %v, want 2 entries", s.pairs)
	}

	env := s.toEnvironment()
	if env.Vars["BASE_URL"] != "https://example.com" || env.Vars["TOKEN"] != "abc" {
		t.Errorf("Vars = %v", env.Vars)
	}
}

func TestEnvEditorImportUpdatesExistingKeyInPlace(t *testing.T) {
	s := newEnvEditorScreen()
	s.name.SetValue("dev")
	s.pairs = []kvPair{{key: "BASE_URL", value: "https://old.example.com"}, {key: "KEEP", value: "me"}}
	path := writeEnvFile(t, "BASE_URL=https://new.example.com\n")

	s.startImport()
	s.pathInput.SetValue(path)
	s.commitImport()

	if len(s.pairs) != 2 {
		t.Fatalf("pairs = %v, want 2 entries (update in place, not append)", s.pairs)
	}
	env := s.toEnvironment()
	if env.Vars["BASE_URL"] != "https://new.example.com" {
		t.Errorf(`Vars["BASE_URL"] = %q, want updated value`, env.Vars["BASE_URL"])
	}
	if env.Vars["KEEP"] != "me" {
		t.Errorf(`Vars["KEEP"] = %q, want untouched "me"`, env.Vars["KEEP"])
	}
}

func TestEnvEditorImportEmptyPathIsError(t *testing.T) {
	s := newEnvEditorScreen()
	s.startImport()
	s.pathInput.SetValue("   ")
	s.commitImport()

	if !s.importing {
		t.Errorf("expected modal to stay open on empty path")
	}
	if s.importErr == "" {
		t.Errorf("expected importErr to be set for an empty path")
	}
}

func TestEnvEditorImportMissingFileIsError(t *testing.T) {
	s := newEnvEditorScreen()
	s.startImport()
	s.pathInput.SetValue(filepath.Join(t.TempDir(), "does-not-exist.env"))
	s.commitImport()

	if !s.importing {
		t.Errorf("expected modal to stay open when the file can't be read")
	}
	if s.importErr == "" {
		t.Errorf("expected importErr to be set for a missing file")
	}
	if len(s.pairs) != 0 {
		t.Errorf("pairs = %v, want unchanged (empty) after a failed import", s.pairs)
	}
}

func TestEnvEditorCloseImportModal(t *testing.T) {
	s := newEnvEditorScreen()
	s.startImport()
	s.closeImportModal()

	if s.importing {
		t.Errorf("expected importing=false after closeImportModal")
	}
}

func TestEnvEditorSessionOnlyFlag(t *testing.T) {
	s := newEnvEditorScreen()
	if s.sessionOnly {
		t.Errorf("sessionOnly should default to false")
	}
	s.sessionOnly = true
	s.loadNew()
	if s.sessionOnly {
		t.Errorf("loadNew should reset sessionOnly to false")
	}
}

func TestEnvEditorClickSelectsRow(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{{key: "a", value: "1"}, {key: "b", value: "2"}, {key: "c", value: "3"}}

	// envRowsContentTop = headerLines(2) + 4 = 6; rows are one line each.
	handled := s.handleMouse(tea.MouseEvent{Y: envRowsContentTop + 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if !handled {
		t.Fatal("expected the click to be handled")
	}
	if s.selected != 2 {
		t.Errorf("selected = %d, want 2", s.selected)
	}
	if s.section != envSectionRows {
		t.Errorf("section = %d, want envSectionRows (%d)", s.section, envSectionRows)
	}
}

func TestEnvEditorClickMissIsNoop(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{{key: "a", value: "1"}}
	s.selected = 0

	// Above the rows.
	if s.handleMouse(tea.MouseEvent{Y: envRowsContentTop - 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}) {
		t.Error("expected a click above the rows to be a no-op")
	}
	// Below the only row.
	if s.handleMouse(tea.MouseEvent{Y: envRowsContentTop + 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}) {
		t.Error("expected a click below the rows to be a no-op")
	}
	if s.selected != 0 {
		t.Errorf("selected changed to %d, want unchanged 0", s.selected)
	}
}

func TestEnvEditorClickIgnoredWhileModalOpen(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{{key: "a", value: "1"}, {key: "b", value: "2"}}
	s.selected = 0
	s.startAddRow() // opens the row-edit modal

	if s.handleMouse(tea.MouseEvent{Y: envRowsContentTop + 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}) {
		t.Error("expected a click to be ignored while the row-edit modal is open")
	}
	if s.selected != 0 {
		t.Errorf("selected changed to %d, want unchanged 0", s.selected)
	}
}

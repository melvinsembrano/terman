package tui

import (
	"fmt"
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

// ─────────────────────────────────────────────
// Viewport scroll
// ─────────────────────────────────────────────

func TestEnvEditorScrollKeepsSelectionVisible(t *testing.T) {
	s := newEnvEditorScreen()
	s.setSize(80, 20) // viewport height = 20-9 = 11

	// Populate more rows than the viewport height.
	for i := 0; i < 20; i++ {
		s.pairs = append(s.pairs, kvPair{key: fmt.Sprintf("key_%02d", i), value: fmt.Sprintf("val_%d", i)})
	}
	s.section = envSectionRows
	s.selected = 0

	// Navigate down past the viewport bottom — YOffset should advance.
	for s.selected < 15 {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // no-op key first
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	if s.selected != 15 {
		t.Fatalf("selected = %d, want 15", s.selected)
	}
	// The selected row (15) must be within the viewport window.
	if s.selected < s.rowOffset || s.selected >= s.rowOffset+s.vp.Height {
		t.Errorf("selected=%d not in viewport window [%d, %d)",
			s.selected, s.rowOffset, s.rowOffset+s.vp.Height)
	}
}

func TestEnvEditorScrollKeepsSelectionVisibleGoingUp(t *testing.T) {
	s := newEnvEditorScreen()
	s.setSize(80, 20)

	for i := 0; i < 20; i++ {
		s.pairs = append(s.pairs, kvPair{key: fmt.Sprintf("key_%02d", i), value: "v"})
	}
	s.section = envSectionRows
	s.selected = 19
	s.rowOffset = 15 // viewport shows rows 15..25 — row 0 is above it

	// Navigate up all the way to the top.
	for s.selected > 0 {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyUp})
	}

	if s.selected != 0 {
		t.Fatalf("selected = %d, want 0", s.selected)
	}
	if s.rowOffset != 0 {
		t.Errorf("rowOffset = %d, want 0 after scrolling back to top", s.rowOffset)
	}
}

func TestEnvEditorClickWithViewportOffset(t *testing.T) {
	s := newEnvEditorScreen()
	s.setSize(80, 20)

	for i := 0; i < 20; i++ {
		s.pairs = append(s.pairs, kvPair{key: fmt.Sprintf("key_%02d", i), value: "v"})
	}
	s.section = envSectionRows
	// Scroll so the viewport shows rows starting at index 5.
	s.rowOffset = 5

	// Click on the first visible line in the viewport (terminal row = envRowsContentTop + 0).
	// With rowOffset=5, that should select row 5.
	handled := s.handleMouse(tea.MouseEvent{
		Y:      envRowsContentTop,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if !handled {
		t.Fatal("expected click to be handled")
	}
	if s.selected != 5 {
		t.Errorf("selected = %d, want 5 (rowOffset 5, click at viewport row 0)", s.selected)
	}
}

// ─────────────────────────────────────────────
// Filter
// ─────────────────────────────────────────────

func TestEnvEditorVisiblePairsNoFilter(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{{key: "alpha", value: "1"}, {key: "beta", value: "2"}}

	visible := s.visiblePairs()
	if len(visible) != 2 {
		t.Errorf("visiblePairs with no filter = %d, want 2", len(visible))
	}
	if visible[0].idx != 0 || visible[1].idx != 1 {
		t.Errorf("indices = %d/%d, want 0/1", visible[0].idx, visible[1].idx)
	}
}

func TestEnvEditorVisiblePairsFilterByKey(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{
		{key: "base_url", value: "https://example.com"},
		{key: "auth_token", value: "secret"},
		{key: "timeout", value: "30"},
	}
	s.filter.SetValue("auth")

	visible := s.visiblePairs()
	if len(visible) != 1 {
		t.Fatalf("visiblePairs filtered by 'auth' = %d, want 1", len(visible))
	}
	if visible[0].pair.key != "auth_token" {
		t.Errorf("visible pair key = %q, want auth_token", visible[0].pair.key)
	}
	if visible[0].idx != 1 {
		t.Errorf("visible pair original index = %d, want 1", visible[0].idx)
	}
}

func TestEnvEditorVisiblePairsFilterByValue(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{
		{key: "base_url", value: "https://api.example.com"},
		{key: "cdn_url", value: "https://cdn.example.com"},
		{key: "timeout", value: "30"},
	}
	s.filter.SetValue("cdn")

	visible := s.visiblePairs()
	if len(visible) != 1 {
		t.Fatalf("visiblePairs filtered by 'cdn' = %d, want 1", len(visible))
	}
	if visible[0].pair.key != "cdn_url" {
		t.Errorf("visible[0].key = %q, want cdn_url", visible[0].pair.key)
	}
}

func TestEnvEditorVisiblePairsFilterCaseInsensitive(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{
		{key: "BASE_URL", value: "https://example.com"},
		{key: "token", value: "abc"},
	}
	s.filter.SetValue("base")

	visible := s.visiblePairs()
	if len(visible) != 1 || visible[0].pair.key != "BASE_URL" {
		t.Errorf("expected case-insensitive match on BASE_URL, got %+v", visible)
	}
}

func TestEnvEditorVisiblePairsNoMatchReturnsEmpty(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{{key: "alpha", value: "1"}, {key: "beta", value: "2"}}
	s.filter.SetValue("zzz")

	visible := s.visiblePairs()
	if len(visible) != 0 {
		t.Errorf("expected 0 visible pairs for non-matching filter, got %d", len(visible))
	}
}

func TestEnvEditorFilterKeyOpensFilterMode(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{{key: "a", value: "1"}}
	s.section = envSectionRows

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if !s.filtering {
		t.Error("expected filtering=true after pressing 'f'")
	}
}

func TestEnvEditorFilterSlashKeyOpensFilterMode(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{{key: "a", value: "1"}}
	s.section = envSectionRows

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !s.filtering {
		t.Error("expected filtering=true after pressing '/'")
	}
}

func TestEnvEditorFilterEscClearsFilter(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{{key: "base_url", value: "x"}, {key: "token", value: "y"}}
	s.section = envSectionRows
	s.filtering = true
	s.filter.SetValue("base")
	s.selected = 0

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if s.filtering {
		t.Error("filtering should be false after esc")
	}
	if s.filter.Value() != "" {
		t.Errorf("filter value = %q, want empty after esc", s.filter.Value())
	}
	if len(s.visiblePairs()) != 2 {
		t.Errorf("expected all pairs visible after clear, got %d", len(s.visiblePairs()))
	}
}

func TestEnvEditorFilterEnterCommitsFilter(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{{key: "base_url", value: "x"}, {key: "token", value: "y"}}
	s.section = envSectionRows
	s.filtering = true
	s.filter.SetValue("base")

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.filtering {
		t.Error("filtering should be false after enter")
	}
	// Filter value persists — rows should still be filtered.
	if s.filter.Value() != "base" {
		t.Errorf("filter value = %q, want 'base' still set after enter", s.filter.Value())
	}
	if len(s.visiblePairs()) != 1 {
		t.Errorf("expected 1 visible pair after committing filter, got %d", len(s.visiblePairs()))
	}
}

func TestEnvEditorFilterEditRowUsesVisibleIndex(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{
		{key: "alpha", value: "1"},
		{key: "beta", value: "2"},
		{key: "gamma", value: "3"},
	}
	s.filter.SetValue("beta")
	s.section = envSectionRows
	s.selected = 0 // first (and only) visible row = "beta"

	s.startEditRow()
	if !s.editing {
		t.Fatal("expected editing=true")
	}
	if s.keyInput.Value() != "beta" {
		t.Errorf("keyInput = %q, want 'beta' (mapped through filter)", s.keyInput.Value())
	}
	// editIdx should be 1 (real index in s.pairs)
	if s.editIdx != 1 {
		t.Errorf("editIdx = %d, want 1", s.editIdx)
	}
}

func TestEnvEditorFilterDeleteRowUsesVisibleIndex(t *testing.T) {
	s := newEnvEditorScreen()
	s.pairs = []kvPair{
		{key: "alpha", value: "1"},
		{key: "beta", value: "2"},
		{key: "gamma", value: "3"},
	}
	s.filter.SetValue("beta")
	s.section = envSectionRows
	s.selected = 0

	s.deleteSelectedRow()

	if len(s.pairs) != 2 {
		t.Fatalf("pairs = %d after deleting filtered row, want 2", len(s.pairs))
	}
	for _, p := range s.pairs {
		if p.key == "beta" {
			t.Error("'beta' should have been deleted")
		}
	}
}

func TestEnvEditorFilterDoesNotDeleteOnSave(t *testing.T) {
	// Filtering is view-only: saving while a filter is active must include
	// ALL variables, not just the visible ones.
	s := newEnvEditorScreen()
	s.name.SetValue("dev")
	s.pairs = []kvPair{
		{key: "base_url", value: "https://example.com"},
		{key: "token", value: "secret"},
	}
	s.filter.SetValue("base")

	env := s.toEnvironment()
	if len(env.Vars) != 2 {
		t.Errorf("expected 2 vars in saved env (filter is view-only), got %d: %v", len(env.Vars), env.Vars)
	}
}

func TestEnvEditorClickWithFilterBarAdjustsOffset(t *testing.T) {
	s := newEnvEditorScreen()
	s.setSize(80, 20)
	s.pairs = []kvPair{
		{key: "alpha", value: "1"},
		{key: "beta", value: "2"},
		{key: "gamma", value: "3"},
	}
	s.section = envSectionRows
	s.filter.SetValue("a") // filter bar is visible (matches alpha + gamma)

	// With filter bar shown, viewport top shifts down by 1.
	// Click at envRowsContentTop+1 (one row below filter bar) → row 0.
	handled := s.handleMouse(tea.MouseEvent{
		Y:      envRowsContentTop + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if !handled {
		t.Fatal("expected click to be handled")
	}
	if s.selected != 0 {
		t.Errorf("selected = %d, want 0", s.selected)
	}
}

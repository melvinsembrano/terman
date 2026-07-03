package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
)

func TestCycleActiveEnvRotation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := appModel{
		envs: []model.Environment{
			{Name: "dev"},
			{Name: "prod"},
		},
	}

	// Starts at "" (no environment) -> first cycle lands on "dev".
	m.cycleActiveEnv()
	if m.activeEnv != "dev" {
		t.Fatalf("after 1st cycle, activeEnv = %q, want %q", m.activeEnv, "dev")
	}
	persisted, err := store.GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if persisted != "dev" {
		t.Errorf("persisted active env = %q, want %q", persisted, "dev")
	}

	m.cycleActiveEnv()
	if m.activeEnv != "prod" {
		t.Fatalf("after 2nd cycle, activeEnv = %q, want %q", m.activeEnv, "prod")
	}

	// Cycling past the last env wraps back to "" (no environment).
	m.cycleActiveEnv()
	if m.activeEnv != "" {
		t.Fatalf("after 3rd cycle, activeEnv = %q, want empty", m.activeEnv)
	}
}

func TestCycleActiveEnvNoEnvsStaysEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := appModel{}
	m.cycleActiveEnv()
	if m.activeEnv != "" {
		t.Errorf("activeEnv with no saved envs = %q, want empty", m.activeEnv)
	}
}

func TestActiveEnvVarsLookup(t *testing.T) {
	m := appModel{
		activeEnv: "Dev", // case mismatch on purpose
		envs: []model.Environment{
			{Name: "dev", Vars: map[string]string{"base_url": "https://dev.example.com"}},
			{Name: "prod", Vars: map[string]string{"base_url": "https://prod.example.com"}},
		},
	}

	got := m.activeEnvVars()
	if got["base_url"] != "https://dev.example.com" {
		t.Errorf("activeEnvVars()[base_url] = %q, want %q", got["base_url"], "https://dev.example.com")
	}
}

func TestActiveEnvVarsNoMatch(t *testing.T) {
	m := appModel{
		activeEnv: "staging",
		envs:      []model.Environment{{Name: "dev"}},
	}
	if got := m.activeEnvVars(); got != nil {
		t.Errorf("activeEnvVars() = %v, want nil", got)
	}
}

func TestReloadEnvsClearsActiveWhenDeleted(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if err := store.SetActiveEnv("dev"); err != nil {
		t.Fatalf("SetActiveEnv: %v", err)
	}

	m := appModel{activeEnv: "dev"}
	// Simulate the active environment having just been deleted elsewhere
	// (e.g. via "d" on the env list screen) before reloadEnvs runs.
	if err := store.DeleteEnv("dev"); err != nil {
		t.Fatalf("DeleteEnv: %v", err)
	}

	if err := m.reloadEnvs(); err != nil {
		t.Fatalf("reloadEnvs: %v", err)
	}
	if m.activeEnv != "" {
		t.Errorf("activeEnv after reloadEnvs = %q, want empty", m.activeEnv)
	}
	persisted, err := store.GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if persisted != "" {
		t.Errorf("persisted active env = %q, want empty", persisted)
	}
}

func TestReloadEnvsKeepsExistingActiveEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	m := appModel{activeEnv: "dev"}
	if err := m.reloadEnvs(); err != nil {
		t.Fatalf("reloadEnvs: %v", err)
	}
	if m.activeEnv != "dev" {
		t.Errorf("activeEnv = %q, want %q", m.activeEnv, "dev")
	}
	if len(m.envs) != 1 || m.envs[0].Name != "dev" {
		t.Errorf("envs = %v, want [dev]", m.envs)
	}
}

func TestReloadEnvsNoActiveEnvStaysEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := appModel{}
	if err := m.reloadEnvs(); err != nil {
		t.Fatalf("reloadEnvs: %v", err)
	}
	if m.activeEnv != "" {
		t.Errorf("activeEnv = %q, want empty", m.activeEnv)
	}
}

func TestPressVOpensEnvManager(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	updated, _ := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateList returned %T, want appModel", updated)
	}
	if am.screen != screenEnvList {
		t.Errorf("screen = %v, want screenEnvList (%v)", am.screen, screenEnvList)
	}
}

func TestEnvListEscReturnsToRequestList(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	m.screen = screenEnvList

	updated, _ := m.updateEnvList(tea.KeyMsg{Type: tea.KeyEsc})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateEnvList returned %T, want appModel", updated)
	}
	if am.screen != screenList {
		t.Errorf("screen = %v, want screenList (%v)", am.screen, screenList)
	}
}

func TestEnvListNewOpensEditor(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	m.screen = screenEnvList

	updated, _ := m.updateEnvList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateEnvList returned %T, want appModel", updated)
	}
	if am.screen != screenEnvEditor {
		t.Errorf("screen = %v, want screenEnvEditor (%v)", am.screen, screenEnvEditor)
	}
}

func TestEnvEditorSaveCreatesEnvAndReturnsToList(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	m.screen = screenEnvEditor
	m.envEditor.loadNew()
	m.envEditor.name.SetValue("dev")

	updated, _ := m.updateEnvEditor(tea.KeyMsg{Type: tea.KeyCtrlS})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateEnvEditor returned %T, want appModel", updated)
	}
	if am.screen != screenList && am.screen != screenEnvList {
		t.Errorf("screen after save = %v, want to have left the editor", am.screen)
	}
	if am.screen != screenEnvList {
		t.Errorf("screen = %v, want screenEnvList (%v)", am.screen, screenEnvList)
	}

	if _, err := store.LoadEnv("dev"); err != nil {
		t.Errorf("expected env 'dev' to be saved: %v", err)
	}
}

func TestPressLFlagsSessionOnly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	m.screen = screenEnvList

	updated, _ := m.updateEnvList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateEnvList returned %T, want appModel", updated)
	}
	if am.screen != screenEnvEditor {
		t.Errorf("screen = %v, want screenEnvEditor (%v)", am.screen, screenEnvEditor)
	}
	if !am.envEditor.sessionOnly {
		t.Errorf("expected envEditor.sessionOnly=true after pressing L")
	}
}

func TestEnvEditorSaveSessionOnlyDoesNotTouchStore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	m.screen = screenEnvEditor
	m.envEditor.loadNew()
	m.envEditor.sessionOnly = true
	m.envEditor.name.SetValue("temp")
	m.envEditor.pairs = []kvPair{{key: "base_url", value: "https://example.com"}}

	updated, _ := m.updateEnvEditor(tea.KeyMsg{Type: tea.KeyCtrlS})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateEnvEditor returned %T, want appModel", updated)
	}
	if am.screen != screenEnvList {
		t.Errorf("screen = %v, want screenEnvList (%v)", am.screen, screenEnvList)
	}
	if !am.isSessionEnv("temp") {
		t.Errorf("expected 'temp' to be tracked as a session env")
	}
	if am.activeEnv != "temp" {
		t.Errorf("activeEnv = %q, want %q (session save should auto-activate)", am.activeEnv, "temp")
	}

	// Nothing should have been written to disk.
	if _, err := store.LoadEnv("temp"); err == nil {
		t.Error("expected session env NOT to be persisted to the store")
	}
	persistedActive, err := store.GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if persistedActive != "" {
		t.Errorf("persisted active env = %q, want empty (session activation must not persist)", persistedActive)
	}
}

func TestEnvListDeleteSessionEnvDoesNotTouchStore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := appModel{
		envs:        []model.Environment{{Name: "temp"}},
		sessionEnvs: map[string]bool{"temp": true},
		activeEnv:   "temp",
		envList:     newEnvListScreen([]model.Environment{{Name: "temp"}}, "temp", map[string]bool{"temp": true}),
	}
	m.envList.lst.Select(0)
	m.screen = screenEnvList

	updated, _ := m.updateEnvList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateEnvList returned %T, want appModel", updated)
	}
	if am.isSessionEnv("temp") {
		t.Errorf("expected 'temp' to be removed from sessionEnvs")
	}
	if len(am.envs) != 0 {
		t.Errorf("envs = %v, want empty after deleting the only session env", am.envs)
	}
	if am.activeEnv != "" {
		t.Errorf("activeEnv = %q, want empty after deleting the active session env", am.activeEnv)
	}
}

func TestEnvListEditSkipsSessionEnv(t *testing.T) {
	envs := []model.Environment{{Name: "temp"}}
	sessionEnvs := map[string]bool{"temp": true}
	m := appModel{
		envs:        envs,
		sessionEnvs: sessionEnvs,
		envList:     newEnvListScreen(envs, "", sessionEnvs),
		envEditor:   newEnvEditorScreen(),
	}
	m.envList.lst.Select(0)
	m.screen = screenEnvList

	updated, _ := m.updateEnvList(tea.KeyMsg{Type: tea.KeyEnter})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateEnvList returned %T, want appModel", updated)
	}
	if am.screen != screenEnvList {
		t.Errorf("screen = %v, want to stay on screenEnvList (editing a session env is not supported)", am.screen)
	}
}

func TestCycleActiveEnvSkipsPersistingSessionEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := appModel{
		envs:        []model.Environment{{Name: "temp"}},
		sessionEnvs: map[string]bool{"temp": true},
	}

	m.cycleActiveEnv() // "" -> "temp"
	if m.activeEnv != "temp" {
		t.Fatalf("activeEnv = %q, want %q", m.activeEnv, "temp")
	}
	persisted, err := store.GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if persisted != "" {
		t.Errorf("persisted active env = %q, want empty (session env must not be persisted)", persisted)
	}
}

func TestEnvListSetActiveSkipsPersistingSessionEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	envs := []model.Environment{{Name: "temp"}}
	sessionEnvs := map[string]bool{"temp": true}
	m := appModel{
		envs:        envs,
		sessionEnvs: sessionEnvs,
		envList:     newEnvListScreen(envs, "", sessionEnvs),
	}
	m.envList.lst.Select(0)
	m.screen = screenEnvList

	updated, _ := m.updateEnvList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateEnvList returned %T, want appModel", updated)
	}
	if am.activeEnv != "temp" {
		t.Errorf("activeEnv = %q, want %q", am.activeEnv, "temp")
	}
	persisted, err := store.GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if persisted != "" {
		t.Errorf("persisted active env = %q, want empty", persisted)
	}
}

func TestReloadEnvsKeepsUnshadowedSessionEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "prod"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	m := appModel{
		envs:        []model.Environment{{Name: "prod"}, {Name: "temp", Vars: map[string]string{"a": "1"}}},
		sessionEnvs: map[string]bool{"temp": true},
		activeEnv:   "temp",
	}

	if err := m.reloadEnvs(); err != nil {
		t.Fatalf("reloadEnvs: %v", err)
	}

	if !m.isSessionEnv("temp") {
		t.Errorf("expected 'temp' to remain a session env")
	}
	if m.activeEnv != "temp" {
		t.Errorf("activeEnv = %q, want %q (unshadowed session env should survive reload)", m.activeEnv, "temp")
	}
	found := false
	for _, e := range m.envs {
		if e.Name == "temp" {
			found = true
			if e.Vars["a"] != "1" {
				t.Errorf("temp.Vars = %v, want a=1 preserved", e.Vars)
			}
		}
	}
	if !found {
		t.Errorf("envs = %v, want 'temp' to still be present", m.envs)
	}
}

func TestReloadEnvsDropsShadowedSessionEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// A persisted env now exists with the same name as the session env.
	if err := store.SaveEnv(model.Environment{Name: "temp", Vars: map[string]string{"persisted": "yes"}}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	m := appModel{
		envs:        []model.Environment{{Name: "temp", Vars: map[string]string{"session": "yes"}}},
		sessionEnvs: map[string]bool{"temp": true},
		activeEnv:   "temp",
	}

	if err := m.reloadEnvs(); err != nil {
		t.Fatalf("reloadEnvs: %v", err)
	}

	if m.isSessionEnv("temp") {
		t.Errorf("expected the session marker for 'temp' to be dropped once shadowed by a persisted env")
	}
	if len(m.envs) != 1 {
		t.Fatalf("envs = %v, want exactly 1 (deduped) entry", m.envs)
	}
	if m.envs[0].Vars["persisted"] != "yes" {
		t.Errorf("expected the persisted version of 'temp' to win, got %v", m.envs[0])
	}
	if m.activeEnv != "temp" {
		t.Errorf("activeEnv = %q, want %q (still exists, now as the persisted env)", m.activeEnv, "temp")
	}
}

func TestEnvEditorSaveBlockedWhileRowModalOpen(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	m.screen = screenEnvEditor
	m.envEditor.loadNew()
	m.envEditor.name.SetValue("dev")
	m.envEditor.startAddRow() // opens the row-edit modal

	updated, _ := m.updateEnvEditor(tea.KeyMsg{Type: tea.KeyCtrlS})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateEnvEditor returned %T, want appModel", updated)
	}
	if am.screen != screenEnvEditor {
		t.Errorf("screen = %v, want to still be in the editor (ctrl+s should be blocked)", am.screen)
	}
	if _, err := store.LoadEnv("dev"); err == nil {
		t.Error("expected env NOT to be saved while the row modal is open")
	}
}

func TestPressIOpensCurlImport(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	updated, _ := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("I")})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateList returned %T, want appModel", updated)
	}
	if am.screen != screenCurlImport {
		t.Errorf("screen = %v, want screenCurlImport (%v)", am.screen, screenCurlImport)
	}
}

func TestCurlImportEscReturnsToRequestList(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	m.screen = screenCurlImport

	updated, _ := m.updateCurlImport(tea.KeyMsg{Type: tea.KeyEsc})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateCurlImport returned %T, want appModel", updated)
	}
	if am.screen != screenList {
		t.Errorf("screen = %v, want screenList (%v)", am.screen, screenList)
	}
}

func TestCurlImportSuccessHandsOffToEditor(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	m.screen = screenCurlImport
	m.curlImport.loadNew()
	m.curlImport.name.SetValue("Get Users")
	m.curlImport.cmd.SetValue(`curl -X POST 'https://example.com/users' -H 'Accept: application/json' -d 'a=1'`)

	updated, _ := m.updateCurlImport(tea.KeyMsg{Type: tea.KeyCtrlS})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateCurlImport returned %T, want appModel", updated)
	}
	if am.screen != screenEditor {
		t.Fatalf("screen = %v, want screenEditor (%v)", am.screen, screenEditor)
	}
	if am.editor.prevName != "" {
		t.Errorf("editor.prevName = %q, want empty (this is a new, unsaved request)", am.editor.prevName)
	}

	got := am.editor.toRequest()
	if got.Method != "POST" || got.URL != "https://example.com/users" || got.Body != "a=1" {
		t.Errorf("editor populated with = %+v", got)
	}
	if got.Headers["Accept"] != "application/json" {
		t.Errorf("editor Headers[Accept] = %q", got.Headers["Accept"])
	}

	// Nothing should be saved to the store until the (unchanged) editor
	// save flow runs.
	if _, err := store.LoadRequest("Get Users"); err == nil {
		t.Error("expected the imported request NOT to be saved yet")
	}
}

func TestCurlImportParseErrorStaysOnScreen(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	m.screen = screenCurlImport
	m.curlImport.loadNew()
	m.curlImport.name.SetValue("Bad")
	m.curlImport.cmd.SetValue(`curl -X GET`) // no URL

	updated, _ := m.updateCurlImport(tea.KeyMsg{Type: tea.KeyCtrlS})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateCurlImport returned %T, want appModel", updated)
	}
	if am.screen != screenCurlImport {
		t.Errorf("screen = %v, want to stay on screenCurlImport (%v) after a parse error", am.screen, screenCurlImport)
	}
	if am.curlImport.err == "" {
		t.Error("expected curlImport.err to be set after a parse error")
	}
}

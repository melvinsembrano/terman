package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
	"github.com/melvinsembrano/terman/internal/version"
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

func TestViewHeaderShowsVersion(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	got := m.View()
	want := "v" + version.Version
	if !strings.Contains(got, want) {
		t.Errorf("View() header does not contain %q, got:\n%s", want, got)
	}
}

func newAppModelWithRequests(t *testing.T, names ...string) appModel {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, n := range names {
		if err := store.SaveRequest(model.Request{Name: n, Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
			t.Fatalf("SaveRequest(%q): %v", n, err)
		}
	}
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	return m
}

func pressMouse(t *testing.T, m appModel, updateFn func(appModel, tea.Msg) (tea.Model, tea.Cmd), msg tea.MouseMsg) appModel {
	t.Helper()
	updated, _ := updateFn(m, msg)
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("update returned %T, want appModel", updated)
	}
	return am
}

func TestListWheelMovesSelection(t *testing.T) {
	m := newAppModelWithRequests(t, "Alpha", "Bravo", "Charlie")
	if req, ok := m.list.selected(); !ok || req.Name != "Alpha" {
		t.Fatalf("initial selection = %+v, want Alpha", req)
	}

	m = pressMouse(t, m, appModel.updateList, tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	if req, ok := m.list.selected(); !ok || req.Name != "Bravo" {
		t.Fatalf("after wheel down, selection = %+v, want Bravo", req)
	}

	m = pressMouse(t, m, appModel.updateList, tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	if req, ok := m.list.selected(); !ok || req.Name != "Alpha" {
		t.Fatalf("after wheel up, selection = %+v, want Alpha", req)
	}
}

func TestListClickSelectsRow(t *testing.T) {
	m := newAppModelWithRequests(t, "Alpha", "Bravo", "Charlie")
	m.list.setSize(80, 20)

	// listContentTop = headerLines + list title block(2); each row's
	// stride = delegate.Height()(2) + delegate.Spacing()(1) = 3.
	m2 := pressMouse(t, m, appModel.updateList, tea.MouseMsg{Y: listContentTop + 2*3, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if req, ok := m2.list.selected(); !ok || req.Name != "Charlie" {
		t.Fatalf("click on row 2 selected = %+v, want Charlie", req)
	}

	m3 := pressMouse(t, m, appModel.updateList, tea.MouseMsg{Y: listContentTop, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if req, ok := m3.list.selected(); !ok || req.Name != "Alpha" {
		t.Fatalf("click on row 0 selected = %+v, want Alpha", req)
	}
}

func TestListClickOutsideContentIsNoop(t *testing.T) {
	m := newAppModelWithRequests(t, "Alpha", "Bravo", "Charlie")
	m.list.setSize(80, 20)

	// Above the content block (on the title).
	above := pressMouse(t, m, appModel.updateList, tea.MouseMsg{Y: 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if req, ok := above.list.selected(); !ok || req.Name != "Alpha" {
		t.Errorf("click above content changed selection to %+v, want unchanged Alpha", req)
	}

	// Well below the last item.
	below := pressMouse(t, m, appModel.updateList, tea.MouseMsg{Y: 100, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if req, ok := below.list.selected(); !ok || req.Name != "Alpha" {
		t.Errorf("click below content changed selection to %+v, want unchanged Alpha", req)
	}
}

func TestEnvListWheelMovesSelection(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := appModel{
		envs:    []model.Environment{{Name: "dev"}, {Name: "prod"}},
		envList: newEnvListScreen([]model.Environment{{Name: "dev"}, {Name: "prod"}}, "", nil),
	}
	m.envList.setSize(80, 20)

	updated, _ := m.updateEnvList(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateEnvList returned %T, want appModel", updated)
	}
	if env, ok := am.envList.selected(); !ok || env.Name != "prod" {
		t.Fatalf("after wheel down, selection = %+v, want prod", env)
	}
}

func TestToggleMouseKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	if !m.mouseEnabled {
		t.Fatal("expected mouseEnabled to start true")
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("Update returned %T, want appModel", updated)
	}
	if am.mouseEnabled {
		t.Error("expected mouseEnabled to flip to false")
	}
	if cmd == nil {
		t.Fatal("expected a non-nil tea.Cmd disabling the mouse")
	}
	if cmd() == nil {
		t.Error("expected the returned cmd to produce a message when invoked")
	}
	if !strings.Contains(am.View(), "mouse: off") {
		t.Errorf("View() does not show the mouse-off indicator, got:\n%s", am.View())
	}

	updated2, cmd2 := am.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	am2, ok := updated2.(appModel)
	if !ok {
		t.Fatalf("Update returned %T, want appModel", updated2)
	}
	if !am2.mouseEnabled {
		t.Error("expected mouseEnabled to flip back to true")
	}
	if cmd2 == nil || cmd2() == nil {
		t.Error("expected a non-nil tea.Cmd re-enabling the mouse")
	}
	if strings.Contains(am2.View(), "mouse: off") {
		t.Errorf("View() still shows the mouse-off indicator after re-enabling, got:\n%s", am2.View())
	}
}

func pressKey(t *testing.T, m appModel, k string) appModel {
	t.Helper()
	updated, _ := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateList returned %T, want appModel", updated)
	}
	return am
}

func pressSpecialKey(t *testing.T, m appModel, typ tea.KeyType) appModel {
	t.Helper()
	updated, _ := m.updateList(tea.KeyMsg{Type: typ})
	am, ok := updated.(appModel)
	if !ok {
		t.Fatalf("updateList returned %T, want appModel", updated)
	}
	return am
}

func TestListEnterOpensFolderThenRequest(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveRequest(model.Request{Name: "Login", Group: "auth", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	// At the top level, the only item is the "auth" folder.
	if _, ok := m.list.selected(); ok {
		t.Fatalf("expected no selected request at the top level")
	}
	name, ok := m.list.selectedFolder()
	if !ok || name != "auth" {
		t.Fatalf("selectedFolder() = %q, %v, want \"auth\", true", name, ok)
	}

	m = pressSpecialKey(t, m, tea.KeyEnter)
	if m.list.curGroup != "auth" {
		t.Fatalf("curGroup after entering folder = %q, want %q", m.list.curGroup, "auth")
	}
	req, ok := m.list.selected()
	if !ok || req.Name != "Login" {
		t.Fatalf("selected() inside auth/ = %+v, %v, want Login", req, ok)
	}

	m = pressSpecialKey(t, m, tea.KeyEnter)
	if m.screen != screenResponse {
		t.Errorf("screen after entering a request = %v, want screenResponse", m.screen)
	}
}

func TestListEscGoesUpAFolder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveRequest(model.Request{Name: "OAuth", Group: "auth/oauth", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	m = pressSpecialKey(t, m, tea.KeyEnter) // -> auth
	m = pressSpecialKey(t, m, tea.KeyEnter) // -> auth/oauth
	if m.list.curGroup != "auth/oauth" {
		t.Fatalf("curGroup = %q, want %q", m.list.curGroup, "auth/oauth")
	}

	m = pressSpecialKey(t, m, tea.KeyEsc)
	if m.list.curGroup != "auth" {
		t.Fatalf("curGroup after esc = %q, want %q", m.list.curGroup, "auth")
	}
	m = pressSpecialKey(t, m, tea.KeyEsc)
	if m.list.curGroup != "" {
		t.Fatalf("curGroup after second esc = %q, want top level", m.list.curGroup)
	}
	// Esc at the top level is a no-op, not a quit.
	m = pressSpecialKey(t, m, tea.KeyEsc)
	if m.screen != screenList {
		t.Errorf("esc at the top level changed screen to %v, want screenList", m.screen)
	}
}

func TestPressNDefaultsNewRequestToCurrentFolder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveRequest(model.Request{Name: "Login", Group: "auth", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	m = pressSpecialKey(t, m, tea.KeyEnter) // descend into auth
	m = pressKey(t, m, "n")
	if m.screen != screenEditor {
		t.Fatalf("screen after \"n\" = %v, want screenEditor", m.screen)
	}
	if got := m.editor.group.Value(); got != "auth" {
		t.Errorf("new request's default folder = %q, want %q", got, "auth")
	}
}

// TestListSearchShowsResultsAcrossAllGroups checks the view-swapping logic
// that backs search: entering the list's own "/" filter should widen the
// underlying item pool from the current folder to every request across
// every group, so a request nested several folders deep is reachable
// without navigating there first. bubbles/list's own fuzzy-matching (which
// runs asynchronously, via a tea.Cmd) is exercised by its own test suite,
// not re-tested here — this only checks list.Model.Items(), the
// synchronously-set underlying pool, not the async-filtered subset.
func TestListSearchShowsResultsAcrossAllGroups(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveRequest(model.Request{Name: "Login", Group: "auth", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	if err := store.SaveRequest(model.Request{Name: "Health", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	// At the top level, browsing shows the "auth" folder plus "Health";
	// "Login" (nested inside auth/) isn't directly visible yet.
	if len(m.list.lst.Items()) != 2 {
		t.Fatalf("top-level items = %d, want 2 (1 folder + 1 request)", len(m.list.lst.Items()))
	}

	m = pressKey(t, m, "/")
	items := m.list.lst.Items()
	if len(items) != 2 {
		t.Fatalf("items while filtering = %d, want 2 (every request across every group)", len(items))
	}
	foundLogin := false
	for _, it := range items {
		if ri, ok := it.(requestItem); ok && ri.req.Name == "Login" {
			foundLogin = true
		}
	}
	if !foundLogin {
		t.Errorf("items while filtering = %+v, want to include the nested \"Login\" request", items)
	}

	// Clearing the filter should restore the top-level folder view.
	m = pressSpecialKey(t, m, tea.KeyEsc)
	if len(m.list.lst.Items()) != 2 {
		t.Fatalf("items after clearing filter = %d, want 2 (the auth folder + Health)", len(m.list.lst.Items()))
	}
	if _, ok := m.list.selectedFolder(); !ok {
		t.Errorf("expected the top-level folder view to be restored after clearing the filter")
	}
}

func TestPressDDeletesFromCorrectGroup(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveRequest(model.Request{Name: "Login", Group: "auth", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	m = pressSpecialKey(t, m, tea.KeyEnter) // descend into auth
	m = pressKey(t, m, "d")

	if _, err := store.LoadRequest("auth/Login"); err == nil {
		t.Errorf("expected request to be deleted")
	}
}

func TestPressXOnRequestEnqueuesExportCmd(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	req := model.Request{Name: "Get Users", Method: "GET", URL: "https://example.com/users"}
	if err := store.SaveRequest(req, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	_, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd == nil {
		t.Error("pressing 'x' on a request should return a non-nil tea.Cmd for the export")
	}
}

func TestPressXWithNoSelectionIsNoop(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// No saved requests — nothing selected.
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	_, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd != nil {
		t.Error("pressing 'x' with no request selected should return nil Cmd")
	}
}

// ---------------------------------------------------------------------------
// env clone (TUI)
// ---------------------------------------------------------------------------

func TestPressCClonesPersistedEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	env := model.Environment{Name: "prod", Vars: map[string]string{"url": "https://prod.example.com"}}
	if err := store.SaveEnv(env, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	_ = m.reloadEnvs()
	m.envList.refresh(m.envs, m.activeEnv, m.sessionEnvs)
	m.screen = screenEnvList

	updated, _ := m.updateEnvList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	am := updated.(appModel)

	// The clone should appear in m.envs.
	var found bool
	for _, e := range am.envs {
		if e.Name == "prod copy" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, len(am.envs))
		for i, e := range am.envs {
			names[i] = e.Name
		}
		t.Errorf("clone %q not found in envs: %v", "prod copy", names)
	}
}

func TestPressCClonesSessionEnvAsSession(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}

	// Add a session-only environment.
	sessionEnv := model.Environment{Name: "local", Vars: map[string]string{"host": "localhost"}}
	m.addSessionEnv(sessionEnv)
	m.envList.refresh(m.envs, m.activeEnv, m.sessionEnvs)
	m.screen = screenEnvList

	updated, _ := m.updateEnvList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	am := updated.(appModel)

	// The clone must exist in m.envs and must itself be a session env.
	var cloneEnv *model.Environment
	for i, e := range am.envs {
		if e.Name == "local copy" {
			cloneEnv = &am.envs[i]
			break
		}
	}
	if cloneEnv == nil {
		t.Fatalf("clone %q not found in envs", "local copy")
	}
	if !am.isSessionEnv("local copy") {
		t.Error("clone of a session env should itself be a session env")
	}
	if cloneEnv.Vars["host"] != "localhost" {
		t.Errorf("clone.Vars[host] = %q, want %q", cloneEnv.Vars["host"], "localhost")
	}

	// Confirm the clone was NOT persisted to disk.
	if _, err := store.LoadEnv("local copy"); err == nil {
		t.Error("session-env clone should not have been written to disk")
	}
}

func TestPressCWithNoEnvSelectedIsNoop(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// No environments saved — list is empty, nothing selected.
	m, err := newAppModel()
	if err != nil {
		t.Fatalf("newAppModel: %v", err)
	}
	m.screen = screenEnvList

	before := len(m.envs)
	updated, _ := m.updateEnvList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	am := updated.(appModel)
	if len(am.envs) != before {
		t.Errorf("pressing 'c' with no selection should not change envs (got %d, want %d)", len(am.envs), before)
	}
}

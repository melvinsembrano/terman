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

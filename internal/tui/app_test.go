package tui

import (
	"testing"

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

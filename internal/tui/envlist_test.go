package tui

import (
	"testing"

	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
)

func TestEnvItemAccessors(t *testing.T) {
	item := envItem{
		env:    model.Environment{Name: "dev", Vars: map[string]string{"a": "1", "b": "2"}},
		active: true,
	}

	if item.Title() != "dev (active)" {
		t.Errorf("Title() = %q, want %q", item.Title(), "dev (active)")
	}
	if item.Description() != "2 variables" {
		t.Errorf("Description() = %q, want %q", item.Description(), "2 variables")
	}
	if item.FilterValue() != "dev" {
		t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), "dev")
	}
}

func TestEnvItemTitleInactive(t *testing.T) {
	item := envItem{env: model.Environment{Name: "prod"}, active: false}
	if item.Title() != "prod" {
		t.Errorf("Title() = %q, want %q", item.Title(), "prod")
	}
}

func TestEnvItemDescriptionSingular(t *testing.T) {
	item := envItem{env: model.Environment{Name: "dev", Vars: map[string]string{"a": "1"}}}
	if item.Description() != "1 variable" {
		t.Errorf("Description() = %q, want %q", item.Description(), "1 variable")
	}
}

func TestNewEnvListScreenMarksActive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if err := store.SaveEnv(model.Environment{Name: "prod"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	lst, err := newEnvListScreen("prod")
	if err != nil {
		t.Fatalf("newEnvListScreen: %v", err)
	}
	items := lst.lst.Items()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, it := range items {
		ei := it.(envItem)
		wantActive := ei.env.Name == "prod"
		if ei.active != wantActive {
			t.Errorf("envItem(%q).active = %v, want %v", ei.env.Name, ei.active, wantActive)
		}
	}
}

func TestEnvListScreenRefreshAndSelected(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	lst, err := newEnvListScreen("")
	if err != nil {
		t.Fatalf("newEnvListScreen: %v", err)
	}
	if len(lst.lst.Items()) != 0 {
		t.Fatalf("expected 0 items initially, got %d", len(lst.lst.Items()))
	}

	if err := store.SaveEnv(model.Environment{Name: "dev"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if err := lst.refresh("dev"); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	env, ok := lst.selected()
	if !ok {
		t.Fatalf("expected a selected environment")
	}
	if env.Name != "dev" {
		t.Errorf("selected().Name = %q, want %q", env.Name, "dev")
	}
}

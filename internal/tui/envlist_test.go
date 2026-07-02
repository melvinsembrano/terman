package tui

import (
	"testing"

	"github.com/melvinsembrano/terman/internal/model"
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

func TestEnvItemTitleSessionTag(t *testing.T) {
	item := envItem{env: model.Environment{Name: "temp"}, session: true}
	if item.Title() != "temp (session)" {
		t.Errorf("Title() = %q, want %q", item.Title(), "temp (session)")
	}
}

func TestEnvItemTitleActiveAndSessionTags(t *testing.T) {
	item := envItem{env: model.Environment{Name: "temp"}, active: true, session: true}
	if item.Title() != "temp (active, session)" {
		t.Errorf("Title() = %q, want %q", item.Title(), "temp (active, session)")
	}
}

func TestNewEnvListScreenMarksActiveAndSession(t *testing.T) {
	envs := []model.Environment{{Name: "dev"}, {Name: "temp"}}
	sessionEnvs := map[string]bool{"temp": true}

	lst := newEnvListScreen(envs, "temp", sessionEnvs)
	items := lst.lst.Items()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, it := range items {
		ei := it.(envItem)
		switch ei.env.Name {
		case "dev":
			if ei.active || ei.session {
				t.Errorf("dev: active=%v session=%v, want both false", ei.active, ei.session)
			}
		case "temp":
			if !ei.active || !ei.session {
				t.Errorf("temp: active=%v session=%v, want both true", ei.active, ei.session)
			}
		}
	}
}

func TestEnvListScreenRefreshAndSelected(t *testing.T) {
	lst := newEnvListScreen(nil, "", nil)
	if len(lst.lst.Items()) != 0 {
		t.Fatalf("expected 0 items initially, got %d", len(lst.lst.Items()))
	}

	envs := []model.Environment{{Name: "dev"}}
	lst.refresh(envs, "dev", nil)

	env, ok := lst.selected()
	if !ok {
		t.Fatalf("expected a selected environment")
	}
	if env.Name != "dev" {
		t.Errorf("selected().Name = %q, want %q", env.Name, "dev")
	}
}

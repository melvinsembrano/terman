package tui

import (
	"testing"

	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
)

func TestRequestItemAccessors(t *testing.T) {
	item := requestItem{req: model.Request{
		Name:   "Get Widget",
		Method: "GET",
		URL:    "https://example.com/widgets",
	}}

	if item.Title() != "Get Widget" {
		t.Errorf("Title() = %q, want %q", item.Title(), "Get Widget")
	}
	if item.Description() != "GET  https://example.com/widgets" {
		t.Errorf("Description() = %q, want %q", item.Description(), "GET  https://example.com/widgets")
	}
	if item.FilterValue() != "Get Widget" {
		t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), "Get Widget")
	}
}

func TestNewListScreenAndRefresh(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	lst, err := newListScreen()
	if err != nil {
		t.Fatalf("newListScreen: %v", err)
	}
	if len(lst.lst.Items()) != 0 {
		t.Fatalf("expected 0 items initially, got %d", len(lst.lst.Items()))
	}

	req := model.Request{Name: "New Req", Method: "GET", URL: "https://example.com"}
	if err := store.SaveRequest(req, ""); err != nil {
		t.Fatalf("save request: %v", err)
	}

	if err := lst.refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	items := lst.lst.Items()
	if len(items) != 1 {
		t.Fatalf("expected 1 item after refresh, got %d", len(items))
	}
	got, ok := items[0].(requestItem)
	if !ok {
		t.Fatalf("item is not a requestItem: %T", items[0])
	}
	if got.req.Name != "New Req" {
		t.Errorf("item Name = %q, want %q", got.req.Name, "New Req")
	}
}

func TestListScreenSelected(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := store.SaveRequest(model.Request{Name: "Only Req", Method: "GET", URL: "https://example.com"}, ""); err != nil {
		t.Fatalf("save request: %v", err)
	}

	lst, err := newListScreen()
	if err != nil {
		t.Fatalf("newListScreen: %v", err)
	}
	req, ok := lst.selected()
	if !ok {
		t.Fatalf("expected a selected item")
	}
	if req.Name != "Only Req" {
		t.Errorf("selected().Name = %q, want %q", req.Name, "Only Req")
	}
}

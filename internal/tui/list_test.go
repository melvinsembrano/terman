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
	if want := "Get Widget GET https://example.com/widgets"; item.FilterValue() != want {
		t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), want)
	}
}

func TestRequestItemDescriptionShowsGroupWhenSearching(t *testing.T) {
	item := requestItem{
		req:       model.Request{Name: "Login", Group: "auth", Method: "POST", URL: "https://example.com/login"},
		showGroup: true,
	}
	if want := "auth  •  POST  https://example.com/login"; item.Description() != want {
		t.Errorf("Description() = %q, want %q", item.Description(), want)
	}
	if want := "Login POST https://example.com/login auth"; item.FilterValue() != want {
		t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), want)
	}
}

func TestChildItemsGroupsSubfoldersAboveRequests(t *testing.T) {
	all := []model.Request{
		{Name: "Health", Method: "GET", URL: "https://example.com/health"},
		{Name: "Login", Group: "auth", Method: "POST", URL: "https://example.com/login"},
		{Name: "Refresh", Group: "auth", Method: "POST", URL: "https://example.com/refresh"},
		{Name: "OAuth Start", Group: "auth/oauth", Method: "GET", URL: "https://example.com/oauth"},
	}

	top := childItems(all, "")
	if len(top) != 2 {
		t.Fatalf("top-level items = %d, want 2 (1 folder + 1 request)", len(top))
	}
	folder, ok := top[0].(folderItem)
	if !ok || folder.name != "auth" || folder.count != 3 {
		t.Errorf("top[0] = %+v, want folderItem{name: auth, count: 3}", top[0])
	}
	if _, ok := top[1].(requestItem); !ok {
		t.Errorf("top[1] = %T, want requestItem", top[1])
	}

	inAuth := childItems(all, "auth")
	if len(inAuth) != 3 {
		t.Fatalf("items in \"auth\" = %d, want 3 (1 folder + 2 requests)", len(inAuth))
	}
	if f, ok := inAuth[0].(folderItem); !ok || f.name != "oauth" || f.count != 1 {
		t.Errorf("inAuth[0] = %+v, want folderItem{name: oauth, count: 1}", inAuth[0])
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
	if err := store.SaveRequest(req, "", ""); err != nil {
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

	if err := store.SaveRequest(model.Request{Name: "Only Req", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
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

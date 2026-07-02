package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/melvinsembrano/terman/internal/model"
)

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Get User (v2)": "get-user-v2",
		"  spaced  ":     "spaced",
		"":                "request",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRequestSaveLoadRenameDelete(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	req := model.Request{Name: "My Request", Method: "GET", URL: "https://example.com"}
	if err := SaveRequest(req, ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	got, err := LoadRequest("my request")
	if err != nil {
		t.Fatalf("LoadRequest: %v", err)
	}
	if got.URL != req.URL {
		t.Errorf("LoadRequest URL = %q, want %q", got.URL, req.URL)
	}

	// Rename should remove the old file.
	renamed := req
	renamed.Name = "Renamed Request"
	if err := SaveRequest(renamed, req.Name); err != nil {
		t.Fatalf("SaveRequest (rename): %v", err)
	}
	if _, err := LoadRequest("My Request"); err == nil {
		t.Errorf("expected old name to be gone after rename")
	}
	if _, err := LoadRequest("Renamed Request"); err != nil {
		t.Errorf("LoadRequest (renamed): %v", err)
	}

	// A hand-edited file whose filename no longer matches its Name field
	// should still resolve by scanning stored content.
	dir, _ := RequestsDir()
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 request file, got %d", len(entries))
	}
	if err := os.Rename(filepath.Join(dir, entries[0].Name()), filepath.Join(dir, "unrelated-filename.yaml")); err != nil {
		t.Fatalf("rename file: %v", err)
	}
	if _, err := LoadRequest("Renamed Request"); err != nil {
		t.Errorf("LoadRequest fallback by content: %v", err)
	}

	if err := DeleteRequest("Renamed Request"); err != nil {
		t.Fatalf("DeleteRequest: %v", err)
	}
	if _, err := LoadRequest("Renamed Request"); err == nil {
		t.Errorf("expected request to be deleted")
	}
}

func TestActiveEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	active, err := GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if active != "" {
		t.Errorf("expected empty active env initially, got %q", active)
	}

	if err := SetActiveEnv("dev"); err != nil {
		t.Fatalf("SetActiveEnv: %v", err)
	}
	active, err = GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if active != "dev" {
		t.Errorf("GetActiveEnv = %q, want %q", active, "dev")
	}
}

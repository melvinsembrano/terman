package store

import (
	"os"
	"path/filepath"
	"strings"
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

func TestEnvSaveLoadDelete(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	env := model.Environment{Name: "Dev", Vars: map[string]string{"base_url": "https://example.com"}}
	if err := SaveEnv(env); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	got, err := LoadEnv("dev") // case-insensitive
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	if got.Vars["base_url"] != env.Vars["base_url"] {
		t.Errorf("LoadEnv Vars[base_url] = %q, want %q", got.Vars["base_url"], env.Vars["base_url"])
	}

	if err := DeleteEnv("Dev"); err != nil {
		t.Fatalf("DeleteEnv: %v", err)
	}
	if _, err := LoadEnv("Dev"); err == nil {
		t.Errorf("expected env to be deleted")
	}
}

func TestLoadEnvUnknown(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, err := LoadEnv("nope"); err == nil {
		t.Errorf("expected error for unknown environment")
	}
}

func TestLoadRequestUnknown(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, err := LoadRequest("nope"); err == nil {
		t.Errorf("expected error for unknown request")
	}
}

func TestDeleteRequestUnknown(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := DeleteRequest("nope"); err == nil {
		t.Errorf("expected error deleting unknown request")
	}
}

func TestLoadRequestsEmptyDirReturnsNoError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	reqs, err := LoadRequests()
	if err != nil {
		t.Fatalf("LoadRequests: %v", err)
	}
	if len(reqs) != 0 {
		t.Errorf("expected 0 requests, got %d", len(reqs))
	}
}

func TestLoadRequestsSortedCaseInsensitive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for _, name := range []string{"banana", "Apple", "cherry"} {
		if err := SaveRequest(model.Request{Name: name, Method: "GET", URL: "https://example.com"}, ""); err != nil {
			t.Fatalf("SaveRequest(%q): %v", name, err)
		}
	}

	reqs, err := LoadRequests()
	if err != nil {
		t.Fatalf("LoadRequests: %v", err)
	}
	got := make([]string, len(reqs))
	for i, r := range reqs {
		got[i] = r.Name
	}
	want := []string{"Apple", "banana", "cherry"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("LoadRequests()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadEnvsSortedCaseInsensitive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for _, name := range []string{"staging", "Dev", "prod"} {
		if err := SaveEnv(model.Environment{Name: name}); err != nil {
			t.Fatalf("SaveEnv(%q): %v", name, err)
		}
	}

	envs, err := LoadEnvs()
	if err != nil {
		t.Fatalf("LoadEnvs: %v", err)
	}
	got := make([]string, len(envs))
	for i, e := range envs {
		got[i] = e.Name
	}
	want := []string{"Dev", "prod", "staging"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("LoadEnvs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBaseDirHonorsXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir, err := BaseDir()
	if err != nil {
		t.Fatalf("BaseDir: %v", err)
	}
	want := filepath.Join(tmp, "terman")
	if dir != want {
		t.Errorf("BaseDir() = %q, want %q", dir, want)
	}
}

func TestBaseDirFallsBackToHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	dir, err := BaseDir()
	if err != nil {
		t.Fatalf("BaseDir: %v", err)
	}
	if !strings.HasSuffix(dir, filepath.Join(".config", "terman")) {
		t.Errorf("BaseDir() = %q, want suffix %q", dir, filepath.Join(".config", "terman"))
	}
}

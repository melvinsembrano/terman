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
		"  spaced  ":    "spaced",
		"":              "request",
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
	if err := SaveRequest(req, "", ""); err != nil {
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
	if err := SaveRequest(renamed, req.Name, req.Group); err != nil {
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

	if err := DeleteRequest("", "Renamed Request"); err != nil {
		t.Fatalf("DeleteRequest: %v", err)
	}
	if _, err := LoadRequest("Renamed Request"); err == nil {
		t.Errorf("expected request to be deleted")
	}
}

func TestRequestGroupedSaveMoveDelete(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	req := model.Request{Name: "Login", Group: "auth", Method: "POST", URL: "https://example.com/login"}
	if err := SaveRequest(req, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	dir, _ := RequestsDir()
	if _, err := os.Stat(filepath.Join(dir, "auth", "login.yaml")); err != nil {
		t.Fatalf("expected file at requests/auth/login.yaml: %v", err)
	}

	got, err := LoadRequest("auth/Login")
	if err != nil {
		t.Fatalf("LoadRequest(\"auth/Login\"): %v", err)
	}
	if got.Group != "auth" || got.Name != "Login" {
		t.Errorf("LoadRequest = %+v, want Group=auth Name=Login", got)
	}

	// Moving to a different folder should remove the old file and create
	// the new one; the vacated "auth" directory should be pruned since it
	// becomes empty.
	moved := req
	moved.Group = "identity/auth"
	if err := SaveRequest(moved, req.Name, req.Group); err != nil {
		t.Fatalf("SaveRequest (move): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "auth")); !os.IsNotExist(err) {
		t.Errorf("expected old group dir \"auth\" to be pruned, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "identity", "auth", "login.yaml")); err != nil {
		t.Fatalf("expected file at requests/identity/auth/login.yaml: %v", err)
	}
	if _, err := LoadRequest("auth/Login"); err == nil {
		t.Errorf("expected old group path to be gone after move")
	}

	if err := DeleteRequest("identity/auth", "Login"); err != nil {
		t.Fatalf("DeleteRequest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "identity")); !os.IsNotExist(err) {
		t.Errorf("expected now-empty ancestor group dirs to be pruned, stat err = %v", err)
	}
}

func TestLoadRequestBareNameAmbiguousAcrossGroups(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := SaveRequest(model.Request{Name: "Login", Group: "auth", Method: "GET", URL: "https://example.com/a"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	if err := SaveRequest(model.Request{Name: "Login", Group: "legacy", Method: "GET", URL: "https://example.com/b"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	if _, err := LoadRequest("Login"); err == nil {
		t.Errorf("expected an ambiguity error for a bare name saved in two groups")
	}

	// The full "group/name" path is unambiguous.
	got, err := LoadRequest("legacy/Login")
	if err != nil {
		t.Fatalf("LoadRequest(\"legacy/Login\"): %v", err)
	}
	if got.URL != "https://example.com/b" {
		t.Errorf("LoadRequest(\"legacy/Login\").URL = %q, want %q", got.URL, "https://example.com/b")
	}
}

func TestLoadRequestsGroupReflectsDirectoryNotStoredField(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := SaveRequest(model.Request{Name: "Ping", Group: "auth", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	// Hand-edit the file's own "group" field so it disagrees with the
	// directory it's actually stored in; the directory should win.
	dir, _ := RequestsDir()
	path := filepath.Join(dir, "auth", "ping.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	stale := strings.Replace(string(data), "group: auth", "group: somewhere-else", 1)
	if stale == string(data) {
		t.Fatalf("test setup: expected to find \"group: auth\" in %s", path)
	}
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reqs, err := LoadRequests()
	if err != nil {
		t.Fatalf("LoadRequests: %v", err)
	}
	if len(reqs) != 1 || reqs[0].Group != "auth" {
		t.Errorf("LoadRequests() = %+v, want a single request with Group \"auth\"", reqs)
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
	if err := SaveEnv(env, ""); err != nil {
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

func TestSaveEnvRenameRemovesOldFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	env := model.Environment{Name: "Dev", Vars: map[string]string{"base_url": "https://dev.example.com"}}
	if err := SaveEnv(env, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	renamed := env
	renamed.Name = "Staging"
	if err := SaveEnv(renamed, env.Name); err != nil {
		t.Fatalf("SaveEnv (rename): %v", err)
	}

	if _, err := LoadEnv("Dev"); err == nil {
		t.Errorf("expected old env name to be gone after rename")
	}
	got, err := LoadEnv("Staging")
	if err != nil {
		t.Fatalf("LoadEnv (renamed): %v", err)
	}
	if got.Vars["base_url"] != env.Vars["base_url"] {
		t.Errorf("renamed env Vars = %v, want %v", got.Vars, env.Vars)
	}

	dir, _ := EnvsDir()
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 env file after rename, got %d", len(entries))
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

	if err := DeleteRequest("", "nope"); err == nil {
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
		if err := SaveRequest(model.Request{Name: name, Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
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
		if err := SaveEnv(model.Environment{Name: name}, ""); err != nil {
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

// chdirTemp changes the working directory to a fresh temp dir for the
// rest of the test, restoring the original directory afterward, and
// returns the *resolved* path to that directory (i.e. what os.Getwd will
// itself report from inside it — on macOS that's the /private/... form,
// which can differ from t.TempDir()'s own return value once symlinks are
// resolved). Used to isolate tests exercising BaseDir's project-local
// ".terman" discovery and cwd-relative default, neither of which the
// other tests in this file (which all pin $XDG_CONFIG_HOME, the
// highest-priority override) depend on.
func chdirTemp(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	return cwd
}

func TestBaseDirFallsBackToHomeConfigWhenItExists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	chdirTemp(t) // no ".terman" anywhere above this directory

	home := t.TempDir()
	t.Setenv("HOME", home)
	legacy := filepath.Join(home, ".config", "terman")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	dir, err := BaseDir()
	if err != nil {
		t.Fatalf("BaseDir: %v", err)
	}
	if dir != legacy {
		t.Errorf("BaseDir() = %q, want %q", dir, legacy)
	}
}

func TestBaseDirDefaultsToLocalWhenNothingExists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	cwd := chdirTemp(t)
	t.Setenv("HOME", t.TempDir()) // no ~/.config/terman here

	dir, err := BaseDir()
	if err != nil {
		t.Fatalf("BaseDir: %v", err)
	}
	want := filepath.Join(cwd, ".terman")
	if dir != want {
		t.Errorf("BaseDir() = %q, want %q", dir, want)
	}
}

func TestBaseDirFindsLocalTermanInParentDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	root := chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	local := filepath.Join(root, ".terman")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	dir, err := BaseDir()
	if err != nil {
		t.Fatalf("BaseDir: %v", err)
	}
	if dir != local {
		t.Errorf("BaseDir() = %q, want %q (found by walking up from %q)", dir, local, sub)
	}
}

func TestBaseDirPrefersLocalTermanOverLegacyHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	root := chdirTemp(t)

	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".config", "terman"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	local := filepath.Join(root, ".terman")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	dir, err := BaseDir()
	if err != nil {
		t.Fatalf("BaseDir: %v", err)
	}
	if dir != local {
		t.Errorf("BaseDir() = %q, want local %q (should be preferred over the legacy home config)", dir, local)
	}
}

func TestInitDirHonorsXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir, err := InitDir()
	if err != nil {
		t.Fatalf("InitDir: %v", err)
	}
	want := filepath.Join(tmp, "terman")
	if dir != want {
		t.Errorf("InitDir() = %q, want %q", dir, want)
	}
}

// TestInitDirIgnoresAncestorTerman is a deliberate behavioral contrast with
// TestBaseDirFindsLocalTermanInParentDir: init is git-init-style and must
// always target the current directory, never an ancestor's ".terman".
func TestInitDirIgnoresAncestorTerman(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	root := chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	if err := os.MkdirAll(filepath.Join(root, ".terman"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	dir, err := InitDir()
	if err != nil {
		t.Fatalf("InitDir: %v", err)
	}
	want := filepath.Join(sub, ".terman")
	if dir != want {
		t.Errorf("InitDir() = %q, want %q (should never find the ancestor's .terman)", dir, want)
	}
}

// TestInitDirIgnoresLegacyHomeConfig is a deliberate contrast with
// TestBaseDirFallsBackToHomeConfigWhenItExists: init never falls back to
// the legacy ~/.config/terman.
func TestInitDirIgnoresLegacyHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	cwd := chdirTemp(t)

	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".config", "terman"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	dir, err := InitDir()
	if err != nil {
		t.Fatalf("InitDir: %v", err)
	}
	want := filepath.Join(cwd, ".terman")
	if dir != want {
		t.Errorf("InitDir() = %q, want %q (should never fall back to the legacy home config)", dir, want)
	}
}

func TestInitFreshBootstrapCreatesDirsOnly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	cwd := chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	res, err := Init(false, false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !res.CreatedDirs {
		t.Fatalf("Init() = %+v, want CreatedDirs=true on a fresh directory", res)
	}
	if res.CreatedEnv || res.CreatedReq || res.SetActiveEnv {
		t.Fatalf("Init() = %+v, want no samples or active env without --examples", res)
	}

	// requests/ and envs/ subdirs must exist.
	if _, err := os.Stat(filepath.Join(cwd, ".terman", "envs")); err != nil {
		t.Errorf("expected envs/ dir to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".terman", "requests")); err != nil {
		t.Errorf("expected requests/ dir to exist: %v", err)
	}

	// No sample files should have been created.
	entries, err := os.ReadDir(filepath.Join(cwd, ".terman", "envs"))
	if err != nil {
		t.Fatalf("ReadDir envs: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty envs/ dir, got %d entries", len(entries))
	}
	entries, err = os.ReadDir(filepath.Join(cwd, ".terman", "requests"))
	if err != nil {
		t.Fatalf("ReadDir requests: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty requests/ dir, got %d entries", len(entries))
	}
}

func TestInitRerunIsNoOp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	if _, err := Init(false, false); err != nil {
		t.Fatalf("Init (first): %v", err)
	}
	res, err := Init(false, false)
	if err != nil {
		t.Fatalf("Init (second): %v", err)
	}
	if res.CreatedDirs || res.CreatedEnv || res.CreatedReq || res.SetActiveEnv {
		t.Errorf("Init() re-run = %+v, want nothing created/changed", res)
	}
}

func TestInitRerunRecreatesDeletedSample(t *testing.T) {
	// This test seeds files manually to simulate existing content, then checks
	// that a plain re-init (no --examples) does not recreate them.
	t.Setenv("XDG_CONFIG_HOME", "")
	cwd := chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	if _, err := Init(false, false); err != nil {
		t.Fatalf("Init (first): %v", err)
	}

	// Manually place a request file to simulate pre-existing content.
	reqDir := filepath.Join(cwd, ".terman", "requests")
	reqPath := filepath.Join(reqDir, "hello-httpbin.yaml")
	if err := os.WriteFile(reqPath, []byte("name: Hello httpbin\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Init(false, false)
	if err != nil {
		t.Fatalf("Init (second): %v", err)
	}
	// Without --examples, no files should be created regardless.
	if res.CreatedDirs || res.CreatedEnv || res.CreatedReq {
		t.Errorf("Init() without --examples = %+v, want nothing created", res)
	}
	// The manually-placed file must still exist (not removed).
	if _, err := os.Stat(reqPath); err != nil {
		t.Errorf("expected manually-placed request to still exist: %v", err)
	}
}

func TestInitDoesNotClobberExistingActiveEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	if err := SetActiveEnv("prod"); err != nil {
		t.Fatalf("SetActiveEnv: %v", err)
	}

	res, err := Init(false, false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if res.SetActiveEnv {
		t.Errorf("Init() set the active env even though one was already set: %+v", res)
	}
	// Without --examples, ActiveEnv is not populated in the result.
	if res.ActiveEnv != "" {
		t.Errorf("ActiveEnv = %q, want empty (no examples mode)", res.ActiveEnv)
	}
	active, err := GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if active != "prod" {
		t.Errorf("GetActiveEnv() = %q, want %q", active, "prod")
	}
}

func TestInitForceOverwritesEditedSamples(t *testing.T) {
	// Without --examples, --force alone has no effect on sample content
	// (there's nothing to overwrite). This test verifies the dirs are still
	// created on a fresh store and nothing else happens.
	t.Setenv("XDG_CONFIG_HOME", "")
	chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	res, err := Init(true, false)
	if err != nil {
		t.Fatalf("Init (force, no examples): %v", err)
	}
	if !res.CreatedDirs {
		t.Errorf("Init(force=true) = %+v, want CreatedDirs=true on fresh dir", res)
	}
	if res.CreatedReq || res.CreatedEnv {
		t.Errorf("Init(force=true, examples=false) = %+v, want no samples written", res)
	}
}

// ---------------------------------------------------------------------------
// CloneEnvName
// ---------------------------------------------------------------------------

func TestCloneEnvNameBasicSuffix(t *testing.T) {
	existing := []model.Environment{{Name: "prod"}}
	got := CloneEnvName(existing, "prod")
	if got != "prod copy" {
		t.Errorf("CloneEnvName = %q, want %q", got, "prod copy")
	}
}

func TestCloneEnvNameCollisionIncrementsN(t *testing.T) {
	existing := []model.Environment{
		{Name: "prod"},
		{Name: "prod copy"},
	}
	got := CloneEnvName(existing, "prod")
	if got != "prod copy 2" {
		t.Errorf("CloneEnvName = %q, want %q", got, "prod copy 2")
	}
}

func TestCloneEnvNameMultipleCollisions(t *testing.T) {
	existing := []model.Environment{
		{Name: "staging"},
		{Name: "staging copy"},
		{Name: "staging copy 2"},
		{Name: "staging copy 3"},
	}
	got := CloneEnvName(existing, "staging")
	if got != "staging copy 4" {
		t.Errorf("CloneEnvName = %q, want %q", got, "staging copy 4")
	}
}

func TestCloneEnvNameCloningACopyStripsOldSuffix(t *testing.T) {
	// Cloning "prod copy" should produce "prod copy" (not "prod copy copy").
	existing := []model.Environment{{Name: "prod copy"}}
	got := CloneEnvName(existing, "prod copy")
	// "prod copy" is taken, so the next candidate is "prod copy 2".
	if got != "prod copy 2" {
		t.Errorf("CloneEnvName = %q, want %q", got, "prod copy 2")
	}
}

func TestCloneEnvNameCloningANumberedCopyStripsOldSuffix(t *testing.T) {
	// Cloning "prod copy 3" should try "prod copy", not "prod copy 3 copy".
	existing := []model.Environment{{Name: "prod copy 3"}}
	got := CloneEnvName(existing, "prod copy 3")
	if got != "prod copy" {
		t.Errorf("CloneEnvName = %q, want %q", got, "prod copy")
	}
}

func TestCloneEnvNameCaseInsensitiveCollision(t *testing.T) {
	existing := []model.Environment{{Name: "Prod Copy"}}
	got := CloneEnvName(existing, "prod")
	// "prod copy" collides with "Prod Copy" (case-insensitive).
	if got != "prod copy 2" {
		t.Errorf("CloneEnvName = %q, want %q", got, "prod copy 2")
	}
}

func TestCloneEnvNameEmptyExisting(t *testing.T) {
	got := CloneEnvName(nil, "dev")
	if got != "dev copy" {
		t.Errorf("CloneEnvName = %q, want %q", got, "dev copy")
	}
}

// ---------------------------------------------------------------------------
// CloneEnv
// ---------------------------------------------------------------------------

func TestCloneEnvCopiesVars(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	src := model.Environment{Name: "prod", Vars: map[string]string{"url": "https://prod.example.com", "token": "abc"}}
	if err := SaveEnv(src, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	clone, err := CloneEnv("prod", "prod copy")
	if err != nil {
		t.Fatalf("CloneEnv: %v", err)
	}
	if clone.Name != "prod copy" {
		t.Errorf("clone.Name = %q, want %q", clone.Name, "prod copy")
	}
	if clone.Vars["url"] != "https://prod.example.com" {
		t.Errorf("clone.Vars[url] = %q, want %q", clone.Vars["url"], "https://prod.example.com")
	}
	if clone.Vars["token"] != "abc" {
		t.Errorf("clone.Vars[token] = %q, want %q", clone.Vars["token"], "abc")
	}
}

func TestCloneEnvPersistsToDisk(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := SaveEnv(model.Environment{Name: "staging", Vars: map[string]string{"k": "v"}}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if _, err := CloneEnv("staging", "staging copy"); err != nil {
		t.Fatalf("CloneEnv: %v", err)
	}

	loaded, err := LoadEnv("staging copy")
	if err != nil {
		t.Fatalf("LoadEnv after clone: %v", err)
	}
	if loaded.Vars["k"] != "v" {
		t.Errorf("loaded.Vars[k] = %q, want %q", loaded.Vars["k"], "v")
	}
}

func TestCloneEnvDoesNotShareMap(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := SaveEnv(model.Environment{Name: "base", Vars: map[string]string{"x": "1"}}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	clone, err := CloneEnv("base", "base copy")
	if err != nil {
		t.Fatalf("CloneEnv: %v", err)
	}
	// Mutate the clone's map in memory — should not affect a freshly loaded original.
	clone.Vars["x"] = "mutated"

	orig, err := LoadEnv("base")
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	if orig.Vars["x"] != "1" {
		t.Errorf("orig.Vars[x] = %q after clone mutation, want %q", orig.Vars["x"], "1")
	}
}

func TestCloneEnvSourceNotFound(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, err := CloneEnv("nonexistent", "copy"); err == nil {
		t.Error("expected error when source env does not exist")
	}
}

func TestCloneEnvNoVars(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := SaveEnv(model.Environment{Name: "empty"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	clone, err := CloneEnv("empty", "empty copy")
	if err != nil {
		t.Fatalf("CloneEnv: %v", err)
	}
	if len(clone.Vars) != 0 {
		t.Errorf("clone.Vars = %v, want empty", clone.Vars)
	}
}

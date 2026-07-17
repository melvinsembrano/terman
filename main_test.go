package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
	"github.com/melvinsembrano/terman/internal/version"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

// withStdin runs fn with os.Stdin replaced by a pipe fed the given
// content, restoring the original os.Stdin afterward.
func withStdin(t *testing.T, content string, fn func()) {
	t.Helper()
	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdin = r
	defer func() { os.Stdin = orig }()

	go func() {
		_, _ = w.WriteString(content)
		_ = w.Close()
	}()

	fn()
}

// chdirTemp changes the working directory to a fresh temp dir for the rest
// of the test, restoring the original directory afterward, and returns the
// *resolved* path to that directory (on macOS, os.Getwd inside it reports
// the "/private/..." form, which can differ from t.TempDir()'s own return
// value once symlinks are resolved). "init" operates on the current
// directory rather than $XDG_CONFIG_HOME, so its tests need this instead
// of the t.Setenv("XDG_CONFIG_HOME", ...) isolation the rest of this file
// uses. Mirrors the identical helper in internal/store/store_test.go.
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

func TestStringSliceSetAppends(t *testing.T) {
	var s stringSlice
	if err := s.Set("a=1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Set("b=2"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(s) != 2 || s[0] != "a=1" || s[1] != "b=2" {
		t.Errorf("stringSlice = %v, want [a=1 b=2]", s)
	}
}

func TestStringSliceString(t *testing.T) {
	s := stringSlice{"a=1", "b=2"}
	if got := s.String(); got != "a=1,b=2" {
		t.Errorf("String() = %q, want %q", got, "a=1,b=2")
	}
}

func TestParseVarOverrides(t *testing.T) {
	got, err := parseVarOverrides([]string{"msg=hi", "base_url=https://example.com"})
	if err != nil {
		t.Fatalf("parseVarOverrides: %v", err)
	}
	if got["msg"] != "hi" || got["base_url"] != "https://example.com" {
		t.Errorf("parseVarOverrides = %v", got)
	}
}

func TestParseVarOverridesValueContainsEquals(t *testing.T) {
	got, err := parseVarOverrides([]string{"token=a=b=c"})
	if err != nil {
		t.Fatalf("parseVarOverrides: %v", err)
	}
	if got["token"] != "a=b=c" {
		t.Errorf(`parseVarOverrides["token"] = %q, want %q`, got["token"], "a=b=c")
	}
}

func TestParseVarOverridesInvalid(t *testing.T) {
	if _, err := parseVarOverrides([]string{"badformat"}); err == nil {
		t.Error("expected error for pair without '='")
	}
}

func TestParseVarOverridesEmpty(t *testing.T) {
	got, err := parseVarOverrides(nil)
	if err != nil {
		t.Fatalf("parseVarOverrides: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("parseVarOverrides(nil) = %v, want empty map", got)
	}
}

func TestCmdListNoRequests(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	out := captureStdout(t, func() {
		if err := cmdList(nil); err != nil {
			t.Fatalf("cmdList: %v", err)
		}
	})
	if !strings.Contains(out, "no saved requests") {
		t.Errorf("cmdList output = %q, want to contain %q", out, "no saved requests")
	}
}

func TestCmdListShowsSavedRequests(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveRequest(model.Request{Name: "Get Widget", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	out := captureStdout(t, func() {
		if err := cmdList(nil); err != nil {
			t.Fatalf("cmdList: %v", err)
		}
	})
	if !strings.Contains(out, "Get Widget") || !strings.Contains(out, "GET") || !strings.Contains(out, "https://example.com") {
		t.Errorf("cmdList output = %q, missing expected fields", out)
	}
}

func TestCmdEnvListMarksActive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if err := store.SaveEnv(model.Environment{Name: "prod"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if err := store.SetActiveEnv("prod"); err != nil {
		t.Fatalf("SetActiveEnv: %v", err)
	}

	out := captureStdout(t, func() {
		if err := cmdEnv([]string{"list"}); err != nil {
			t.Fatalf("cmdEnv: %v", err)
		}
	})
	if !strings.Contains(out, "* prod") {
		t.Errorf("cmdEnv list output = %q, want to contain %q", out, "* prod")
	}
	if !strings.Contains(out, "  dev") {
		t.Errorf("cmdEnv list output = %q, want to contain unmarked %q", out, "dev")
	}
}

func TestCmdEnvUse(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "staging"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	if err := cmdEnv([]string{"use", "staging"}); err != nil {
		t.Fatalf("cmdEnv use: %v", err)
	}
	active, err := store.GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if active != "staging" {
		t.Errorf("active env = %q, want %q", active, "staging")
	}
}

func TestCmdEnvUseUnknown(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdEnv([]string{"use", "nope"}); err == nil {
		t.Error("expected error using an unknown environment")
	}
}

func TestCmdEnvMissingArgs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdEnv(nil); err == nil {
		t.Error("expected error for cmdEnv with no args")
	}
	if err := cmdEnv([]string{"use"}); err == nil {
		t.Error("expected error for cmdEnv use with no name")
	}
	if err := cmdEnv([]string{"bogus"}); err == nil {
		t.Error("expected error for unknown env subcommand")
	}
}

func TestCmdEnvSetCreatesNewEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := cmdEnv([]string{"set", "dev", "base_url=https://example.com", "token=abc"}); err != nil {
		t.Fatalf("cmdEnv set: %v", err)
	}

	env, err := store.LoadEnv("dev")
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	if env.Vars["base_url"] != "https://example.com" || env.Vars["token"] != "abc" {
		t.Errorf("env.Vars = %v, want base_url/token set", env.Vars)
	}
}

func TestCmdEnvSetUpdatesExistingEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev", Vars: map[string]string{"a": "1", "b": "2"}}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	// Overwrite "a" and add "c"; "b" should be left untouched.
	if err := cmdEnv([]string{"set", "dev", "a=updated", "c=3"}); err != nil {
		t.Fatalf("cmdEnv set: %v", err)
	}

	env, err := store.LoadEnv("dev")
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	want := map[string]string{"a": "updated", "b": "2", "c": "3"}
	for k, v := range want {
		if env.Vars[k] != v {
			t.Errorf("env.Vars[%q] = %q, want %q", k, env.Vars[k], v)
		}
	}
}

func TestCmdEnvSetInvalidPair(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdEnv([]string{"set", "dev", "badformat"}); err == nil {
		t.Error("expected error for malformed k=v pair")
	}
}

func TestCmdEnvSetMissingArgs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdEnv([]string{"set"}); err == nil {
		t.Error("expected error for env set with no name")
	}
}

func TestCmdEnvImportCreatesNewEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.dev")
	if err := os.WriteFile(path, []byte("BASE_URL=https://example.com\nTOKEN=abc\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := cmdEnv([]string{"import", path, "dev"}); err != nil {
		t.Fatalf("cmdEnv import: %v", err)
	}

	env, err := store.LoadEnv("dev")
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	if env.Vars["BASE_URL"] != "https://example.com" || env.Vars["TOKEN"] != "abc" {
		t.Errorf("env.Vars = %v", env.Vars)
	}
}

func TestCmdEnvImportMergesIntoExistingEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev", Vars: map[string]string{"a": "1", "b": "2"}}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	// Overwrite "b" and add "c"; "a" should be left untouched.
	if err := os.WriteFile(path, []byte("b=updated\nc=3\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := cmdEnv([]string{"import", path, "dev"}); err != nil {
		t.Fatalf("cmdEnv import: %v", err)
	}

	env, err := store.LoadEnv("dev")
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	want := map[string]string{"a": "1", "b": "updated", "c": "3"}
	for k, v := range want {
		if env.Vars[k] != v {
			t.Errorf("env.Vars[%q] = %q, want %q", k, env.Vars[k], v)
		}
	}
}

func TestCmdEnvImportMissingArgs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdEnv([]string{"import", "path-only"}); err == nil {
		t.Error("expected error for env import with no name")
	}
}

func TestCmdEnvImportMissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdEnv([]string{"import", filepath.Join(t.TempDir(), "nope.env"), "dev"}); err == nil {
		t.Error("expected error for a missing .env file")
	}
}

func TestCmdEnvImportMalformedFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("NOTAVAR\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := cmdEnv([]string{"import", path, "dev"}); err == nil {
		t.Error("expected error for a malformed .env file")
	}
}

func TestCmdEnvShow(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev", Vars: map[string]string{"base_url": "https://example.com"}}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	out := captureStdout(t, func() {
		if err := cmdEnv([]string{"show", "dev"}); err != nil {
			t.Fatalf("cmdEnv show: %v", err)
		}
	})
	if !strings.Contains(out, "base_url=https://example.com") {
		t.Errorf("cmdEnv show output = %q, want to contain %q", out, "base_url=https://example.com")
	}
}

func TestCmdEnvShowNoVariables(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "empty"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	out := captureStdout(t, func() {
		if err := cmdEnv([]string{"show", "empty"}); err != nil {
			t.Fatalf("cmdEnv show: %v", err)
		}
	})
	if !strings.Contains(out, "no variables") {
		t.Errorf("cmdEnv show output = %q, want to contain %q", out, "no variables")
	}
}

func TestCmdEnvShowUnknown(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdEnv([]string{"show", "nope"}); err == nil {
		t.Error("expected error showing an unknown environment")
	}
}

func TestCmdEnvUnset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev", Vars: map[string]string{"a": "1", "b": "2"}}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	if err := cmdEnv([]string{"unset", "dev", "a"}); err != nil {
		t.Fatalf("cmdEnv unset: %v", err)
	}

	env, err := store.LoadEnv("dev")
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	if _, ok := env.Vars["a"]; ok {
		t.Errorf("expected key 'a' to be removed, Vars = %v", env.Vars)
	}
	if env.Vars["b"] != "2" {
		t.Errorf("expected key 'b' to remain, Vars = %v", env.Vars)
	}
}

func TestCmdEnvUnsetMissingArgs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdEnv([]string{"unset", "dev"}); err == nil {
		t.Error("expected error for env unset with no keys")
	}
}

func TestCmdEnvUnsetUnknownEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdEnv([]string{"unset", "nope", "a"}); err == nil {
		t.Error("expected error unsetting a var on an unknown environment")
	}
}

func TestCmdEnvDelete(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	if err := cmdEnv([]string{"delete", "dev"}); err != nil {
		t.Fatalf("cmdEnv delete: %v", err)
	}
	if _, err := store.LoadEnv("dev"); err == nil {
		t.Error("expected env to be deleted")
	}
}

func TestCmdEnvDeleteClearsActive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if err := store.SetActiveEnv("dev"); err != nil {
		t.Fatalf("SetActiveEnv: %v", err)
	}

	if err := cmdEnv([]string{"delete", "dev"}); err != nil {
		t.Fatalf("cmdEnv delete: %v", err)
	}

	active, err := store.GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if active != "" {
		t.Errorf("active env after deleting it = %q, want empty", active)
	}
}

func TestCmdEnvDeleteLeavesOtherActiveEnvAlone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveEnv(model.Environment{Name: "dev"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if err := store.SaveEnv(model.Environment{Name: "prod"}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if err := store.SetActiveEnv("prod"); err != nil {
		t.Fatalf("SetActiveEnv: %v", err)
	}

	if err := cmdEnv([]string{"delete", "dev"}); err != nil {
		t.Fatalf("cmdEnv delete: %v", err)
	}

	active, err := store.GetActiveEnv()
	if err != nil {
		t.Fatalf("GetActiveEnv: %v", err)
	}
	if active != "prod" {
		t.Errorf("active env = %q, want %q (unaffected by deleting a different env)", active, "prod")
	}
}

func TestCmdEnvDeleteMissingArgs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdEnv([]string{"delete"}); err == nil {
		t.Error("expected error for env delete with no name")
	}
}

func TestCmdRunMissingArgs(t *testing.T) {
	if err := cmdRun(nil); err == nil {
		t.Error("expected error for cmdRun with no args")
	}
}

func TestCmdRunUnknownRequest(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdRun([]string{"nope"}); err == nil {
		t.Error("expected error for an unknown saved request")
	}
}

func TestCmdRunInvalidVarFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveRequest(model.Request{Name: "Req", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	if err := cmdRun([]string{"Req", "--var", "badformat"}); err == nil {
		t.Error("expected error for a malformed --var flag")
	}
}

func TestCmdRunUnknownEnvFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveRequest(model.Request{Name: "Req", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	if err := cmdRun([]string{"Req", "--env", "nope"}); err == nil {
		t.Error("expected error for an unknown --env")
	}
}

// TestCmdRunSuccess exercises the full happy path (env resolution + var
// override + HTTP call) without hitting the os.Exit(1) branch, which would
// kill the test process. That branch is only reached on non-2xx responses.
func TestCmdRunSuccess(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	}))
	defer srv.Close()

	if err := store.SaveEnv(model.Environment{Name: "dev", Vars: map[string]string{"base_url": srv.URL}}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if err := store.SaveRequest(model.Request{
		Name:   "Ping",
		Method: "GET",
		URL:    "{{base_url}}/ping?msg={{msg}}",
	}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	out := captureStdout(t, func() {
		if err := cmdRun([]string{"Ping", "--env", "dev", "--var", "msg=hi"}); err != nil {
			t.Fatalf("cmdRun: %v", err)
		}
	})

	if gotQuery != "msg=hi" {
		t.Errorf("server received query = %q, want %q", gotQuery, "msg=hi")
	}
	if !strings.Contains(out, "200") || !strings.Contains(out, "pong") {
		t.Errorf("cmdRun output = %q, want to contain status 200 and body", out)
	}
}

// TestCmdRunEnvFilePrecedence verifies the --env-file overlay: it sits
// between the active/--env environment and --var overrides. base_url comes
// only from the environment, token comes only from the file, and msg is
// set by all three layers to prove --var wins over --env-file which wins
// over the environment.
func TestCmdRunEnvFilePrecedence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := store.SaveEnv(model.Environment{
		Name: "dev",
		Vars: map[string]string{"base_url": srv.URL, "msg": "from-env"},
	}, ""); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}
	if err := store.SaveRequest(model.Request{
		Name:   "Ping",
		Method: "GET",
		URL:    "{{base_url}}/ping?msg={{msg}}&token={{token}}",
	}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	envFilePath := filepath.Join(t.TempDir(), ".env.local")
	if err := os.WriteFile(envFilePath, []byte("msg=from-file\ntoken=abc\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	captureStdout(t, func() {
		if err := cmdRun([]string{"Ping", "--env", "dev", "--env-file", envFilePath, "--var", "msg=from-var"}); err != nil {
			t.Fatalf("cmdRun: %v", err)
		}
	})

	want := "msg=from-var&token=abc"
	if gotQuery != want {
		t.Errorf("server received query = %q, want %q", gotQuery, want)
	}
}

func TestCmdRunEnvFileMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := store.SaveRequest(model.Request{Name: "Req", Method: "GET", URL: "https://example.com"}, "", ""); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	if err := cmdRun([]string{"Req", "--env-file", filepath.Join(t.TempDir(), "nope.env")}); err == nil {
		t.Error("expected error for a missing --env-file")
	}
}

func TestCmdImportCurlFromStdin(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	out := ""
	withStdin(t, `curl 'https://example.com/users' -H 'Accept: application/json' -d 'a=1'`, func() {
		out = captureStdout(t, func() {
			if err := cmdImport([]string{"curl", "Get Users"}); err != nil {
				t.Fatalf("cmdImport: %v", err)
			}
		})
	})

	if !strings.Contains(out, `Imported "Get Users"`) || !strings.Contains(out, "POST") || !strings.Contains(out, "https://example.com/users") {
		t.Errorf("cmdImport output = %q, missing expected fields", out)
	}

	req, err := store.LoadRequest("Get Users")
	if err != nil {
		t.Fatalf("LoadRequest: %v", err)
	}
	if req.Method != "POST" || req.URL != "https://example.com/users" || req.Body != "a=1" {
		t.Errorf("saved request = %+v", req)
	}
	if req.Headers["Accept"] != "application/json" {
		t.Errorf("saved Headers[Accept] = %q", req.Headers["Accept"])
	}
}

func TestCmdImportCurlFromFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path := filepath.Join(t.TempDir(), "curl.txt")
	if err := os.WriteFile(path, []byte(`curl -X PUT 'https://example.com/widgets/1'`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	captureStdout(t, func() {
		if err := cmdImport([]string{"curl", "Update Widget", path}); err != nil {
			t.Fatalf("cmdImport: %v", err)
		}
	})

	req, err := store.LoadRequest("Update Widget")
	if err != nil {
		t.Fatalf("LoadRequest: %v", err)
	}
	if req.Method != "PUT" || req.URL != "https://example.com/widgets/1" {
		t.Errorf("saved request = %+v", req)
	}
}

func TestCmdImportCurlMissingArgs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdImport(nil); err == nil {
		t.Error("expected error for cmdImport with no args")
	}
	if err := cmdImport([]string{"curl"}); err == nil {
		t.Error("expected error for import curl with no name")
	}
}

func TestCmdImportUnknownSubcommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdImport([]string{"postman"}); err == nil {
		t.Error("expected error for an unknown import subcommand")
	}
}

func TestCmdImportCurlMissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdImport([]string{"curl", "Name", filepath.Join(t.TempDir(), "nope.txt")}); err == nil {
		t.Error("expected error for a missing curl command file")
	}
}

func TestCmdImportCurlInvalidCommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	withStdin(t, "curl -X GET", func() { // no URL
		if err := cmdImport([]string{"curl", "Name"}); err == nil {
			t.Error("expected error for a curl command with no URL")
		}
	})
}

func TestCmdVersion(t *testing.T) {
	out := captureStdout(t, cmdVersion)
	if !strings.Contains(out, "terman") || !strings.Contains(out, version.Version) {
		t.Errorf("cmdVersion output = %q, want to contain %q and %q", out, "terman", version.Version)
	}
}

func TestCmdInitFreshCreatesDirsAndPrintsSummary(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	cwd := chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	out := captureStdout(t, func() {
		if err := cmdInit(nil); err != nil {
			t.Fatalf("cmdInit: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(cwd, ".terman", "envs")); err != nil {
		t.Errorf("expected envs/ dir to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".terman", "requests")); err != nil {
		t.Errorf("expected requests/ dir to exist: %v", err)
	}
	if !strings.Contains(out, "Initialized terman store in") {
		t.Errorf("cmdInit output missing init message, got:\n%s", out)
	}
	// No sample files should have been mentioned.
	for _, notwant := range []string{`Created environment`, `Created request`} {
		if strings.Contains(out, notwant) {
			t.Errorf("cmdInit output should not contain %q without --examples, got:\n%s", notwant, out)
		}
	}
}

func TestCmdInitRerunIsNoOp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	captureStdout(t, func() {
		if err := cmdInit(nil); err != nil {
			t.Fatalf("cmdInit (first): %v", err)
		}
	})
	out := captureStdout(t, func() {
		if err := cmdInit(nil); err != nil {
			t.Fatalf("cmdInit (second): %v", err)
		}
	})

	if !strings.Contains(out, "terman store already initialized in") {
		t.Errorf("re-run output missing already-initialized message, got:\n%s", out)
	}
	if strings.Contains(out, "Created") {
		t.Errorf("re-run output should not report anything created, got:\n%s", out)
	}
}

// TestCmdInitInSubdirCreatesItsOwnStore is the key regression test for why
// "init" can't just reuse store.BaseDir(): it must always target the
// current directory (git-init semantics), even inside a subdirectory of an
// already-initialized project.
func TestCmdInitInSubdirCreatesItsOwnStore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	root := chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	captureStdout(t, func() {
		if err := cmdInit(nil); err != nil {
			t.Fatalf("cmdInit (root): %v", err)
		}
	})

	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	captureStdout(t, func() {
		if err := cmdInit(nil); err != nil {
			t.Fatalf("cmdInit (sub): %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(sub, ".terman", "requests")); err != nil {
		t.Errorf("expected a new .terman/requests/ under %s, got: %v", sub, err)
	}
}

func TestCmdInitRespectsXDGConfigHome(t *testing.T) {
	chdirTemp(t) // a fresh dir with no local .terman
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	if err := cmdInit(nil); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}

	if _, err := os.Stat(filepath.Join(xdg, "terman", "requests")); err != nil {
		t.Errorf("expected requests/ dir under $XDG_CONFIG_HOME/terman, got: %v", err)
	}
}

func TestCmdInitForceIsNoOpWithoutExamples(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	// --force without --examples should still just create dirs, nothing else.
	out := captureStdout(t, func() {
		if err := cmdInit([]string{"--force"}); err != nil {
			t.Fatalf("cmdInit (force): %v", err)
		}
	})
	if !strings.Contains(out, "Initialized terman store in") {
		t.Errorf("force init output missing init message, got:\n%s", out)
	}
	if strings.Contains(out, "Created") {
		t.Errorf("force init without --examples should not report created files, got:\n%s", out)
	}
}

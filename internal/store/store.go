// Package store persists Requests and Environments as one YAML file per
// item under the terman data directory, plus a small config file that
// remembers the active environment.
//
// Requests are grouped into folders on disk: each Request lives at
// <requestsDir>/<group>/<slug(name)>.yaml, where group is a "/"-separated
// path ("" for the top level). Environments stay flat, one file per name
// directly under <envsDir>.
package store

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/melvinsembrano/terman/internal/model"
	"gopkg.in/yaml.v3"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slug turns one name/path-segment into a filesystem-safe stem, e.g.
// "Get User (v2)" -> "get-user-v2".
func slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "request"
	}
	return s
}

// slugGroup turns a "/"-separated group path into a filesystem-safe
// relative directory, slugging each segment individually (so stray
// characters, "..", or a leading "/" in a hand-typed folder name can't
// escape the requests directory) and dropping empty segments.
func slugGroup(group string) string {
	var parts []string
	for _, seg := range strings.Split(filepath.ToSlash(group), "/") {
		if seg = strings.TrimSpace(seg); seg != "" {
			parts = append(parts, slug(seg))
		}
	}
	return filepath.Join(parts...)
}

// SplitGroupName splits a "group/sub/name" path (as accepted by the CLI's
// "run" and "import curl" commands, and shown by "list") into its group
// ("" if name has no "/") and base name. Only the last "/" is significant,
// so group segments may themselves contain "/".
func SplitGroupName(name string) (group, base string) {
	name = filepath.ToSlash(strings.TrimSpace(name))
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[:idx], name[idx+1:]
	}
	return "", name
}

// FullPath renders a request's group and name back into the "group/name"
// form SplitGroupName parses, e.g. for CLI output and error messages.
func FullPath(r model.Request) string {
	if r.Group == "" {
		return r.Name
	}
	return r.Group + "/" + r.Name
}

// findUpward walks from dir upward through its ancestors looking for a
// child directory named name, returning its full path if found.
func findUpward(dir, name string) (string, bool) {
	for {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// BaseDir returns the terman data root to use for this run, resolved in
// order:
//
//  1. $XDG_CONFIG_HOME/terman, if $XDG_CONFIG_HOME is set — an explicit
//     override, honored unconditionally for backward compatibility.
//  2. The nearest ".terman" directory found by walking up from the
//     current working directory (project-local, git-style: works from any
//     subdirectory of a project that already has one).
//  3. The legacy global "~/.config/terman", if it already exists on disk
//     (so installs that predate project-local storage keep working).
//  4. "./.terman" in the current directory, created on first write.
func BaseDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "terman"), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if dir, ok := findUpward(cwd, ".terman"); ok {
		return dir, nil
	}

	if home, err := os.UserHomeDir(); err == nil {
		legacy := filepath.Join(home, ".config", "terman")
		if info, err := os.Stat(legacy); err == nil && info.IsDir() {
			return legacy, nil
		}
	}

	return filepath.Join(cwd, ".terman"), nil
}

// InitDir returns the terman data root that "terman init" should create:
//
//  1. $XDG_CONFIG_HOME/terman, if set — the same override BaseDir honors, so
//     data lands where later commands will find it.
//  2. "./.terman" in the current directory.
//
// Unlike BaseDir, it never walks up to an ancestor ".terman" and never falls
// back to the legacy ~/.config/terman: init behaves like "git init", always
// targeting the current directory, even inside a subdirectory of an
// existing project-local store.
func InitDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "terman"), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".terman"), nil
}

func RequestsDir() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "requests"), nil
}

func EnvsDir() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "envs"), nil
}

func configPath() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "config.yaml"), nil
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

func readYAMLFiles[T any](dir string) ([]T, error) {
	if err := ensureDir(dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []T
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var v T
		if err := yaml.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, v)
	}
	return out, nil
}

// walkRequestFiles walks every request YAML file nested anywhere under
// dir, calling fn with the file's path, its group (the "/"-separated
// directory path between dir and the file, "" at the top level), and its
// unmarshaled contents.
func walkRequestFiles(dir string, fn func(path, group string, r model.Request) error) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var r model.Request
		if err := yaml.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		rel, err := filepath.Rel(dir, filepath.Dir(path))
		if err != nil {
			return err
		}
		group := ""
		if rel != "." {
			group = filepath.ToSlash(rel)
		}
		return fn(path, group, r)
	})
}

// requestPath returns the file a request with the given group and name is
// (or should be) stored at.
func requestPath(requestsDir, group, name string) string {
	return filepath.Join(requestsDir, slugGroup(group), slug(name)+".yaml")
}

// pruneEmptyGroupDirs removes dir and any now-empty ancestor directories,
// stopping at (and never removing) root. Best-effort: any error, or a
// directory that isn't empty, just stops the walk — this is cleanup, not
// a required part of a save/delete succeeding.
func pruneEmptyGroupDirs(root, dir string) {
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// LoadRequests returns every saved request across every group, sorted by
// group then name (case-insensitive) — a stable order the UI's folder
// tree can render directly.
func LoadRequests() ([]model.Request, error) {
	dir, err := RequestsDir()
	if err != nil {
		return nil, err
	}
	if err := ensureDir(dir); err != nil {
		return nil, err
	}
	var out []model.Request
	err = walkRequestFiles(dir, func(_, group string, r model.Request) error {
		r.Group = group
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		gi, gj := strings.ToLower(out[i].Group), strings.ToLower(out[j].Group)
		if gi != gj {
			return gi < gj
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// LoadRequest finds a saved request by name (case-insensitive). name may
// be a bare request name or a "group/name" path (as shown by "terman
// list" and accepted by "terman run"). It first tries the conventional
// slug path implied by the parsed group, then falls back to scanning
// every saved request by its stored Group/Name (so a hand-renamed or
// hand-moved file still resolves). A bare name matching requests in more
// than one group is ambiguous and returns an error listing the
// candidates.
func LoadRequest(name string) (model.Request, error) {
	dir, err := RequestsDir()
	if err != nil {
		return model.Request{}, err
	}

	group, base := SplitGroupName(name)
	path := requestPath(dir, group, base)
	if data, err := os.ReadFile(path); err == nil {
		var r model.Request
		if err := yaml.Unmarshal(data, &r); err != nil {
			return model.Request{}, fmt.Errorf("%s: %w", path, err)
		}
		r.Group = group
		return r, nil
	}

	reqs, err := LoadRequests()
	if err != nil {
		return model.Request{}, err
	}
	var matches []model.Request
	for _, r := range reqs {
		switch {
		case strings.EqualFold(FullPath(r), name):
			matches = append(matches, r)
		case group == "" && strings.EqualFold(r.Name, name):
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return model.Request{}, fmt.Errorf("no saved request named %q", name)
	case 1:
		return matches[0], nil
	default:
		paths := make([]string, len(matches))
		for i, r := range matches {
			paths[i] = FullPath(r)
		}
		return model.Request{}, fmt.Errorf("%q is ambiguous, matches: %s", name, strings.Join(paths, ", "))
	}
}

// SaveRequest writes r to the file implied by its Group and Name. If
// prevName is non-empty and (prevGroup, prevName) resolves to a different
// file than (r.Group, r.Name) — i.e. the request was renamed and/or moved
// to a different folder — the old file is removed after the new one is
// written successfully, and its now-possibly-empty group directory is
// pruned.
func SaveRequest(r model.Request, prevName, prevGroup string) error {
	dir, err := RequestsDir()
	if err != nil {
		return err
	}
	path := requestPath(dir, r.Group, r.Name)
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	if prevName != "" {
		if oldPath := requestPath(dir, prevGroup, prevName); oldPath != path {
			_ = os.Remove(oldPath)
			pruneEmptyGroupDirs(dir, filepath.Dir(oldPath))
		}
	}
	return nil
}

// DeleteRequest removes the saved request with the given group ("" for
// the top level) and name, pruning its group directory if that leaves it
// empty.
func DeleteRequest(group, name string) error {
	dir, err := RequestsDir()
	if err != nil {
		return err
	}
	path := requestPath(dir, group, name)
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
		pruneEmptyGroupDirs(dir, filepath.Dir(path))
		return nil
	}

	// Fall back to matching by stored Name in case the filename doesn't
	// follow the slug convention.
	var found string
	err = walkRequestFiles(dir, func(path, fileGroup string, r model.Request) error {
		if found == "" && strings.EqualFold(fileGroup, group) && strings.EqualFold(r.Name, name) {
			found = path
		}
		return nil
	})
	if err != nil {
		return err
	}
	if found == "" {
		return fmt.Errorf("no saved request named %q", name)
	}
	if err := os.Remove(found); err != nil {
		return err
	}
	pruneEmptyGroupDirs(dir, filepath.Dir(found))
	return nil
}

// LoadEnvs returns all saved environments, sorted by name.
func LoadEnvs() ([]model.Environment, error) {
	dir, err := EnvsDir()
	if err != nil {
		return nil, err
	}
	envs, err := readYAMLFiles[model.Environment](dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(envs, func(i, j int) bool {
		return strings.ToLower(envs[i].Name) < strings.ToLower(envs[j].Name)
	})
	return envs, nil
}

// LoadEnv finds a saved environment by name (case-insensitive).
func LoadEnv(name string) (model.Environment, error) {
	envs, err := LoadEnvs()
	if err != nil {
		return model.Environment{}, err
	}
	for _, e := range envs {
		if strings.EqualFold(e.Name, name) {
			return e, nil
		}
	}
	return model.Environment{}, fmt.Errorf("no saved environment named %q", name)
}

// SaveEnv writes e to its slug-named file. If prevName is non-empty and its
// slug differs from e.Name's slug (i.e. the environment was renamed), the
// old file is removed after the new one is written successfully.
func SaveEnv(e model.Environment, prevName string) error {
	dir, err := EnvsDir()
	if err != nil {
		return err
	}
	if err := ensureDir(dir); err != nil {
		return err
	}
	data, err := yaml.Marshal(e)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, slug(e.Name)+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	if prevName != "" && slug(prevName) != slug(e.Name) {
		_ = os.Remove(filepath.Join(dir, slug(prevName)+".yaml"))
	}
	return nil
}

// DeleteEnv removes the saved environment with the given name.
func DeleteEnv(name string) error {
	dir, err := EnvsDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, slug(name)+".yaml")
	return os.Remove(path)
}

type localConfig struct {
	ActiveEnv string `yaml:"active_env,omitempty"`
}

// loadConfigAt reads the local config at path, or a zero-value localConfig
// if it doesn't exist yet.
func loadConfigAt(path string) (localConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return localConfig{}, nil
		}
		return localConfig{}, err
	}
	var c localConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return localConfig{}, err
	}
	return c, nil
}

// saveConfigAt writes c to path, creating dir (path's parent) if needed.
func saveConfigAt(dir, path string, c localConfig) error {
	if err := ensureDir(dir); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func loadConfig() (localConfig, error) {
	path, err := configPath()
	if err != nil {
		return localConfig{}, err
	}
	return loadConfigAt(path)
}

func saveConfig(c localConfig) error {
	base, err := BaseDir()
	if err != nil {
		return err
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	return saveConfigAt(base, path, c)
}

// GetActiveEnv returns the name of the currently active environment, or
// "" if none has been set.
func GetActiveEnv() (string, error) {
	c, err := loadConfig()
	if err != nil {
		return "", err
	}
	return c.ActiveEnv, nil
}

// SetActiveEnv persists name as the active environment.
func SetActiveEnv(name string) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	c.ActiveEnv = name
	return saveConfig(c)
}

const (
	sampleEnvName = "dev"
	sampleReqName = "Hello httpbin"
)

// sampleEnv and sampleRequest are the "terman init" starter content: a
// no-auth, no-signup demo (httpbin.org echoes back whatever you send) that
// works immediately after init, and demonstrates {{var}} substitution.
func sampleEnv() model.Environment {
	return model.Environment{
		Name: sampleEnvName,
		Vars: map[string]string{"base_url": "https://httpbin.org"},
	}
}

func sampleRequest() model.Request {
	return model.Request{
		Name:    sampleReqName,
		Method:  "GET",
		URL:     "{{base_url}}/get",
		Headers: map[string]string{"Accept": "application/json"},
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// InitResult reports what Init created versus what already existed and was
// left untouched, so the caller can print an accurate summary.
type InitResult struct {
	BaseDir string // the resolved data root (InitDir())

	CreatedDirs bool // the .terman tree didn't fully exist and was created

	RequestName   string
	RequestMethod string
	RequestURL    string
	CreatedReq    bool // sample request was (re)written, vs. already present

	EnvName    string
	EnvBaseURL string
	CreatedEnv bool // sample environment was (re)written, vs. already present

	SetActiveEnv bool   // true if Init set the active env (none was set before)
	ActiveEnv    string // the active env after Init (the existing one, if one was already set)

	FetchedFiles []string // files downloaded from GitHub when --examples was used
}

// Init bootstraps a project-local terman store in the directory InitDir
// resolves to (never BaseDir — see InitDir's doc comment): it creates the
// requests/ and envs/ subtrees. When examples is true, example files are
// fetched from the terman GitHub repository and written into the store; the
// first environment found is made active when none was previously set. When
// examples is false (the default), no sample files are seeded and the active
// environment is not touched. It is safe to re-run: existing user data is
// never overwritten unless force is set. The returned InitResult describes
// exactly what happened, for the caller to report back.
func Init(force, examples bool) (InitResult, error) {
	base, err := InitDir()
	if err != nil {
		return InitResult{}, err
	}
	res := InitResult{BaseDir: base}

	if info, statErr := os.Stat(base); statErr == nil {
		if !info.IsDir() {
			return InitResult{}, fmt.Errorf("%s exists but is not a directory", base)
		}
	} else if os.IsNotExist(statErr) {
		res.CreatedDirs = true
	} else {
		return InitResult{}, statErr
	}

	requestsDir := filepath.Join(base, "requests")
	envsDir := filepath.Join(base, "envs")
	if err := ensureDir(requestsDir); err != nil {
		return InitResult{}, err
	}
	if err := ensureDir(envsDir); err != nil {
		return InitResult{}, err
	}

	if examples {
		fetched, firstEnv, err := fetchExamples(base, force)
		if err != nil {
			return InitResult{}, err
		}
		res.FetchedFiles = fetched

		// Set the active environment to the first env fetched, if none is set.
		if firstEnv != "" {
			cfgPath := filepath.Join(base, "config.yaml")
			cfg, err := loadConfigAt(cfgPath)
			if err != nil {
				return InitResult{}, err
			}
			if cfg.ActiveEnv == "" {
				cfg.ActiveEnv = firstEnv
				if err := saveConfigAt(base, cfgPath, cfg); err != nil {
					return InitResult{}, err
				}
				res.SetActiveEnv = true
				res.ActiveEnv = firstEnv
			} else {
				res.ActiveEnv = cfg.ActiveEnv
			}
		}
	}

	return res, nil
}

// githubContentItem is a single entry returned by the GitHub Contents API.
type githubContentItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
}

const githubExamplesBase = "https://api.github.com/repos/melvinsembrano/terman/contents/.terman"

// fetchExamples downloads all files from the terman GitHub repository's
// .terman directory (recursively) and writes them into base, mirroring the
// directory structure under .terman/envs and .terman/requests. It skips
// config.yaml and any file that already exists unless force is true. It
// returns the list of relative paths written, the name of the first
// environment file found (for setting as active), and any error.
func fetchExamples(base string, force bool) (fetched []string, firstEnv string, err error) {
	fetched, firstEnv, err = fetchExamplesDir(githubExamplesBase, base, "", force)
	return
}

// fetchExamplesDir recursively fetches a GitHub Contents API directory.
// apiURL is the full API URL for the directory; base is the local root;
// relDir is the path relative to base represented by this directory.
func fetchExamplesDir(apiURL, base, relDir string, force bool) (fetched []string, firstEnv string, err error) {
	resp, err := http.Get(apiURL) //nolint:noctx
	if err != nil {
		return nil, "", fmt.Errorf("fetching examples index %s: %w", apiURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetching examples index %s: HTTP %d", apiURL, resp.StatusCode)
	}

	var items []githubContentItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, "", fmt.Errorf("decoding examples index: %w", err)
	}

	for _, item := range items {
		itemRel := filepath.Join(relDir, item.Name)
		switch item.Type {
		case "dir":
			subFetched, subFirst, err := fetchExamplesDir(
				githubExamplesBase+"/"+item.Path[len(".terman/"):],
				base, itemRel, force,
			)
			if err != nil {
				return fetched, firstEnv, err
			}
			fetched = append(fetched, subFetched...)
			if firstEnv == "" {
				firstEnv = subFirst
			}
		case "file":
			// Skip config.yaml — we manage active-env ourselves.
			if item.Name == "config.yaml" {
				continue
			}
			localPath := filepath.Join(base, itemRel)
			if !force && fileExists(localPath) {
				continue
			}
			if err := ensureDir(filepath.Dir(localPath)); err != nil {
				return fetched, firstEnv, err
			}
			if err := downloadFile(item.DownloadURL, localPath); err != nil {
				return fetched, firstEnv, err
			}
			fetched = append(fetched, itemRel)
			// Track the first env file (envs/<name>.yaml) to set as active.
			if firstEnv == "" && strings.HasPrefix(itemRel, "envs"+string(filepath.Separator)) {
				name := strings.TrimSuffix(item.Name, ".yaml")
				if name != "" {
					firstEnv = name
				}
			}
		}
	}
	return fetched, firstEnv, nil
}

// downloadFile fetches rawURL and writes the body to localPath.
func downloadFile(rawURL, localPath string) error {
	resp, err := http.Get(rawURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("downloading %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: HTTP %d", rawURL, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading %s: %w", rawURL, err)
	}
	return os.WriteFile(localPath, data, 0o644)
}

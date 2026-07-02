// Package store persists Requests and Environments as one YAML file per
// item under the user's config directory, plus a small config file that
// remembers the active environment.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/melvinsembrano/terman/internal/model"
	"gopkg.in/yaml.v3"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slug turns a request/environment name into a filesystem-safe filename
// stem, e.g. "Get User (v2)" -> "get-user-v2".
func slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "request"
	}
	return s
}

// BaseDir returns the terman config root, honoring $XDG_CONFIG_HOME and
// falling back to ~/.config/terman.
func BaseDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "terman"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "terman"), nil
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

// LoadRequests returns all saved requests, sorted by name.
func LoadRequests() ([]model.Request, error) {
	dir, err := RequestsDir()
	if err != nil {
		return nil, err
	}
	reqs, err := readYAMLFiles[model.Request](dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(reqs, func(i, j int) bool {
		return strings.ToLower(reqs[i].Name) < strings.ToLower(reqs[j].Name)
	})
	return reqs, nil
}

// LoadRequest finds a saved request by name (case-insensitive). It first
// tries the conventional slug filename, then falls back to scanning all
// requests by their stored Name field (so a hand-renamed Name inside a
// file, or a file that doesn't follow the slug convention, still resolves).
func LoadRequest(name string) (model.Request, error) {
	dir, err := RequestsDir()
	if err != nil {
		return model.Request{}, err
	}
	path := filepath.Join(dir, slug(name)+".yaml")
	if data, err := os.ReadFile(path); err == nil {
		var r model.Request
		if err := yaml.Unmarshal(data, &r); err != nil {
			return model.Request{}, fmt.Errorf("%s: %w", path, err)
		}
		return r, nil
	}

	reqs, err := LoadRequests()
	if err != nil {
		return model.Request{}, err
	}
	for _, r := range reqs {
		if strings.EqualFold(r.Name, name) {
			return r, nil
		}
	}
	return model.Request{}, fmt.Errorf("no saved request named %q", name)
}

// SaveRequest writes r to its slug-named file. If prevName is non-empty
// and its slug differs from r.Name's slug (i.e. the request was renamed),
// the old file is removed after the new one is written successfully.
func SaveRequest(r model.Request, prevName string) error {
	dir, err := RequestsDir()
	if err != nil {
		return err
	}
	if err := ensureDir(dir); err != nil {
		return err
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, slug(r.Name)+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	if prevName != "" && slug(prevName) != slug(r.Name) {
		_ = os.Remove(filepath.Join(dir, slug(prevName)+".yaml"))
	}
	return nil
}

// DeleteRequest removes the saved request with the given name.
func DeleteRequest(name string) error {
	dir, err := RequestsDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, slug(name)+".yaml")
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}
	// Fall back to matching by stored Name in case the filename doesn't
	// follow the slug convention.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		fp := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		var r model.Request
		if err := yaml.Unmarshal(data, &r); err == nil && strings.EqualFold(r.Name, name) {
			return os.Remove(fp)
		}
	}
	return fmt.Errorf("no saved request named %q", name)
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

func loadConfig() (localConfig, error) {
	path, err := configPath()
	if err != nil {
		return localConfig{}, err
	}
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

func saveConfig(c localConfig) error {
	base, err := BaseDir()
	if err != nil {
		return err
	}
	if err := ensureDir(base); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
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

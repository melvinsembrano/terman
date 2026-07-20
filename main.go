// Command terman is a Terminal API Client: a TUI and CLI for building,
// saving, and organizing HTTP requests and the environments that
// parameterize them, then running them interactively or straight from the
// command line for scripting and CI.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/melvinsembrano/terman/internal/curl"
	"github.com/melvinsembrano/terman/internal/dotenv"
	"github.com/melvinsembrano/terman/internal/httpx"
	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
	"github.com/melvinsembrano/terman/internal/swagger"
	"github.com/melvinsembrano/terman/internal/tui"
	"github.com/melvinsembrano/terman/internal/vars"
	"github.com/melvinsembrano/terman/internal/version"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		if err := tui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	var err error
	switch args[0] {
	case "init":
		err = cmdInit(args[1:])
	case "run":
		err = cmdRun(args[1:])
	case "list":
		err = cmdList(args[1:])
	case "env":
		err = cmdEnv(args[1:])
	case "import":
		err = cmdImport(args[1:])
	case "version", "-v", "--version":
		cmdVersion()
		return
	case "-h", "--help", "help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		printUsage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `terman - Terminal API Client

Usage:
  terman                                Launch the TUI
  terman init [--force] [--examples]        Set up .terman here (use --examples to seed sample requests + envs)
  terman run <name> [flags]             Run a saved request
  terman list                           List saved requests
  terman env list                       List saved environments
  terman env show <name>                Show an environment's variables
  terman env set <name> <k=v>...        Create/update an environment's variables
  terman env import <file> <name>       Merge a .env file's variables into an environment
  terman env unset <name> <key>...      Remove variables from an environment
  terman env delete <name>              Delete an environment
  terman env use <name>                 Set the active environment
  terman import curl <name> [file]      Save a request parsed from a curl command
  terman import swagger <file> [group] [flags]  Import requests from a Swagger/OpenAPI file
  terman version                        Show version information
  terman help                           Show this help

<name> above may be a bare request name or a "group/name" path (requests
can be organized into folders, e.g. "terman run auth/login"); "terman
list" shows each request's full path.

Flags for "run":
  --env <name>       Use this environment instead of the active one
  --env-file <path>  Load extra variables from a .env file for this run only (not saved)
  --var k=v          Override/add a variable (repeatable)
  -i                 Also print response headers

"import curl" reads the curl command from <file> if given, otherwise from
stdin (e.g. "pbpaste | terman import curl \"Get Users\"" or
"pbpaste | terman import curl \"auth/Login\"" to save it into a folder).

"import swagger" imports every path+method from a Swagger 2.x or OpenAPI 3.x
file as individual requests. All variable parts (base URL, path/query params,
request body top-level fields) are replaced with {{var}} placeholders so the
same requests can be reused across environments. An environment holding those
variable values is also created (or merged into if it already exists).

Flags for "import swagger":
  --env <name>   Environment name for extracted variables (default: group name)

Data is stored in ./.terman if that directory (searched for in the current
directory and its parents) already exists, otherwise in the legacy
~/.config/terman if that exists, otherwise a fresh ./.terman is created in
the current directory. Set $XDG_CONFIG_HOME to override this explicitly.

"init" always targets ./.terman in the current directory specifically
(never a parent's, even inside a subdirectory of an existing project) and
is safe to run again later: it only creates whatever is missing. Pass
--examples to seed example requests and environments fetched from the
terman GitHub repository; --force overwrites any files that already exist.
`)
}

// stringSlice is a repeatable --flag k=v collector.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// parseVarOverrides turns "k=v" pairs (as passed via repeated --var flags)
// into a map. Values may themselves contain "=" (only the first one splits
// the key from the value). Returns an error if any pair lacks a "=".
func parseVarOverrides(pairs []string) (map[string]string, error) {
	overrides := map[string]string{}
	for _, kv := range pairs {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --var %q, expected k=v", kv)
		}
		overrides[k] = v
	}
	return overrides, nil
}

// upsertEnvVars merges overrides into the saved environment named name,
// creating it if it doesn't already exist, and persists the result. Used
// by both "env set" (from --var-style pairs) and "env import" (from a
// parsed .env file).
func upsertEnvVars(name string, overrides map[string]string) error {
	env, err := store.LoadEnv(name)
	if err != nil {
		env = model.Environment{Name: name}
	}
	if env.Vars == nil {
		env.Vars = map[string]string{}
	}
	for k, v := range overrides {
		env.Vars[k] = v
	}
	return store.SaveEnv(env, "")
}

// cmdInit bootstraps a project-local terman store in the current
// directory (see store.Init/store.InitDir), then prints a summary of what
// was created versus what was already there.
func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	force := fs.Bool("force", false, "overwrite existing files")
	examples := fs.Bool("examples", false, "seed example requests and environments from github.com/melvinsembrano/terman")
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := store.Init(*force, *examples)
	if err != nil {
		return err
	}

	if res.CreatedDirs {
		fmt.Printf("Initialized terman store in %s\n", res.BaseDir)
	} else {
		fmt.Printf("terman store already initialized in %s\n", res.BaseDir)
	}

	if *examples {
		if len(res.FetchedFiles) > 0 {
			fmt.Printf("Fetched %d example file(s):\n", len(res.FetchedFiles))
			for _, f := range res.FetchedFiles {
				fmt.Printf("  %s\n", f)
			}
		} else {
			fmt.Println("No new example files fetched (all already present; use --force to overwrite)")
		}
		if res.SetActiveEnv {
			fmt.Printf("Set active environment to %q\n", res.ActiveEnv)
		} else if res.ActiveEnv != "" {
			fmt.Printf("Active environment is %q, left unchanged\n", res.ActiveEnv)
		}
		if len(res.FetchedFiles) > 0 {
			fmt.Printf("\nTry it:  terman list\n")
		}
	}

	return nil
}

func cmdRun(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: terman run <name> [--env <name>] [--env-file <path>] [--var k=v] [-i]")
	}
	name := args[0]

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	envName := fs.String("env", "", "environment to use")
	envFile := fs.String("env-file", "", "load additional variables from a .env file for this run only")
	printHeaders := fs.Bool("i", false, "also print response headers")
	var varOverrides stringSlice
	fs.Var(&varOverrides, "var", "override/add a variable, k=v (repeatable)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	req, err := store.LoadRequest(name)
	if err != nil {
		return err
	}

	if *envName == "" {
		*envName, err = store.GetActiveEnv()
		if err != nil {
			return err
		}
	}

	envVars := map[string]string{}
	if *envName != "" {
		env, err := store.LoadEnv(*envName)
		if err != nil {
			return err
		}
		envVars = env.Vars
	}

	var fileVars map[string]string
	if *envFile != "" {
		fileVars, err = dotenv.ParseFile(*envFile)
		if err != nil {
			return err
		}
	}

	overrides, err := parseVarOverrides(varOverrides)
	if err != nil {
		return err
	}

	// Precedence, lowest to highest: active/--env environment, --env-file,
	// --var. Nothing here is persisted — this is a single-invocation overlay.
	resolved := vars.Merge(envVars, fileVars, overrides)

	resp, err := httpx.Do(req, resolved)
	if err != nil {
		return err
	}

	fmt.Printf("%s  (%s)\n", resp.Status, resp.Duration.Round(1000000))
	if *printHeaders {
		fmt.Print(resp.HeadersString())
		fmt.Println()
	}
	if resp.Body != "" {
		fmt.Println(resp.Body)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		os.Exit(1)
	}
	return nil
}

func cmdList(args []string) error {
	reqs, err := store.LoadRequests()
	if err != nil {
		return err
	}
	if len(reqs) == 0 {
		fmt.Println("no saved requests")
		return nil
	}
	for _, r := range reqs {
		fmt.Printf("%-24s %-6s %s\n", store.FullPath(r), r.Method, r.URL)
	}
	return nil
}

func cmdEnv(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: terman env list|show|set|unset|delete|use ...")
	}
	switch args[0] {
	case "list":
		envs, err := store.LoadEnvs()
		if err != nil {
			return err
		}
		active, err := store.GetActiveEnv()
		if err != nil {
			return err
		}
		if len(envs) == 0 {
			fmt.Println("no saved environments")
			return nil
		}
		for _, e := range envs {
			marker := " "
			if strings.EqualFold(e.Name, active) {
				marker = "*"
			}
			fmt.Printf("%s %s\n", marker, e.Name)
		}
		return nil
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("usage: terman env show <name>")
		}
		env, err := store.LoadEnv(args[1])
		if err != nil {
			return err
		}
		if len(env.Vars) == 0 {
			fmt.Println("no variables")
			return nil
		}
		keys := make([]string, 0, len(env.Vars))
		for k := range env.Vars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s=%s\n", k, env.Vars[k])
		}
		return nil
	case "set":
		if len(args) < 2 {
			return fmt.Errorf("usage: terman env set <name> <k=v>...")
		}
		overrides, err := parseVarOverrides(args[2:])
		if err != nil {
			return err
		}
		return upsertEnvVars(args[1], overrides)
	case "import":
		if len(args) < 3 {
			return fmt.Errorf("usage: terman env import <file> <name>")
		}
		file, name := args[1], args[2]
		parsed, err := dotenv.ParseFile(file)
		if err != nil {
			return err
		}
		return upsertEnvVars(name, parsed)
	case "unset":
		if len(args) < 3 {
			return fmt.Errorf("usage: terman env unset <name> <key>...")
		}
		name := args[1]
		env, err := store.LoadEnv(name)
		if err != nil {
			return err
		}
		for _, k := range args[2:] {
			delete(env.Vars, k)
		}
		return store.SaveEnv(env, "")
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("usage: terman env delete <name>")
		}
		name := args[1]
		if err := store.DeleteEnv(name); err != nil {
			return err
		}
		active, err := store.GetActiveEnv()
		if err != nil {
			return err
		}
		if strings.EqualFold(active, name) {
			return store.SetActiveEnv("")
		}
		return nil
	case "use":
		if len(args) < 2 {
			return fmt.Errorf("usage: terman env use <name>")
		}
		if _, err := store.LoadEnv(args[1]); err != nil {
			return err
		}
		return store.SetActiveEnv(args[1])
	default:
		return fmt.Errorf("unknown env subcommand %q", args[0])
	}
}

func cmdImport(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: terman import curl <name> [file]\n       terman import swagger <file> [group] [--env <name>]")
	}
	switch args[0] {
	case "curl":
		return cmdImportCurl(args[1:])
	case "swagger":
		return cmdImportSwagger(args[1:])
	default:
		return fmt.Errorf("unknown import subcommand %q", args[0])
	}
}

func cmdImportCurl(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: terman import curl <name> [file]")
	}
	name := args[0]

	var r io.Reader = os.Stdin
	if len(args) >= 2 {
		f, err := os.Open(args[1])
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	req, err := curl.Parse(string(data))
	if err != nil {
		return err
	}
	req.Group, req.Name = store.SplitGroupName(name)

	if err := store.SaveRequest(req, "", ""); err != nil {
		return err
	}

	fmt.Printf("Imported %q: %s %s", store.FullPath(req), req.Method, req.URL)
	if n := len(req.Headers); n > 0 {
		if n == 1 {
			fmt.Print(" (1 header)")
		} else {
			fmt.Printf(" (%d headers)", n)
		}
	}
	fmt.Println()
	return nil
}

// cmdImportSwagger handles "terman import swagger <file> [group] [--env <name>]".
//
// It reads a Swagger 2.x or OpenAPI 3.x file (JSON or YAML), converts every
// path+method into a terman Request with {{var}} placeholders, and merges the
// extracted variable values into a named environment.
//
//	<file>   path to the swagger/openapi file
//	[group]  optional request group name; defaults to the base name of the
//	         file's directory (or filename stem when the file is in the cwd)
//	--env    environment name (defaults to the group name)
func cmdImportSwagger(args []string) error {
	// Pre-scan for --env flag so it works regardless of position relative to
	// positional args (Go's flag package stops at the first non-flag argument).
	envName := ""
	filtered := args[:0:0] // start fresh, same underlying array
	for i := 0; i < len(args); i++ {
		if args[i] == "--env" {
			if i+1 >= len(args) {
				return fmt.Errorf("--env requires a value")
			}
			envName = args[i+1]
			i++ // skip value
		} else if strings.HasPrefix(args[i], "--env=") {
			envName = strings.TrimPrefix(args[i], "--env=")
		} else {
			filtered = append(filtered, args[i])
		}
	}
	args = filtered

	if len(args) == 0 {
		return fmt.Errorf("usage: terman import swagger <file> [group] [--env <name>]")
	}
	filePath := args[0]

	// Determine the group name.
	group := ""
	if len(args) >= 2 {
		group = args[1]
	}
	if group == "" {
		// Default: base name of the file's containing directory.
		dir := filepath.Dir(filePath)
		base := filepath.Base(dir)
		if base == "." || base == "/" {
			// File is in the cwd — use the filename stem instead.
			stem := filepath.Base(filePath)
			stem = strings.TrimSuffix(stem, filepath.Ext(stem))
			base = stem
		}
		group = base
	}

	if envName == "" {
		envName = group
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	result, err := swagger.Parse(data, filepath.Base(filePath), envName)
	if err != nil {
		return err
	}

	if len(result.Requests) == 0 {
		fmt.Println("No requests found in the spec.")
		return nil
	}

	// Save requests under the specified group.
	for i := range result.Requests {
		result.Requests[i].Group = group
		if err := store.SaveRequest(result.Requests[i], "", ""); err != nil {
			return fmt.Errorf("save request %q: %w", result.Requests[i].Name, err)
		}
	}

	// Merge environment variables (upsert — same pattern as env import).
	if err := upsertEnvVars(envName, result.Environment.Vars); err != nil {
		return fmt.Errorf("save environment %q: %w", envName, err)
	}

	fmt.Printf("Imported %d request(s) into group %q, environment %q\n",
		len(result.Requests), group, envName)
	return nil
}

func cmdVersion() {
	fmt.Println("terman " + version.Version)
}

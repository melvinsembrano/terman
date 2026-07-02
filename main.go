// Command terman is a terminal, simplified Postman: a TUI for building and
// saving HTTP requests, and a CLI for running saved requests directly.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/melvinsembrano/terman/internal/httpx"
	"github.com/melvinsembrano/terman/internal/store"
	"github.com/melvinsembrano/terman/internal/tui"
	"github.com/melvinsembrano/terman/internal/vars"
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
	case "run":
		err = cmdRun(args[1:])
	case "list":
		err = cmdList(args[1:])
	case "env":
		err = cmdEnv(args[1:])
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
	fmt.Fprint(os.Stderr, `terman - a terminal, simplified Postman

Usage:
  terman                                Launch the TUI
  terman run <name> [flags]             Run a saved request
  terman list                           List saved requests
  terman env list                       List saved environments
  terman env use <name>                 Set the active environment
  terman help                           Show this help

Flags for "run":
  --env <name>      Use this environment instead of the active one
  --var k=v          Override/add a variable (repeatable)
  -i                 Also print response headers
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

func cmdRun(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: terman run <name> [--env <name>] [--var k=v] [-i]")
	}
	name := args[0]

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	envName := fs.String("env", "", "environment to use")
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

	overrides, err := parseVarOverrides(varOverrides)
	if err != nil {
		return err
	}

	resolved := vars.Merge(envVars, overrides)

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
		fmt.Printf("%-24s %-6s %s\n", r.Name, r.Method, r.URL)
	}
	return nil
}

func cmdEnv(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: terman env list | terman env use <name>")
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

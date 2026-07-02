// Package dotenv parses .env-style files (KEY=VALUE per line) used to
// import or session-load variables into a terman environment.
package dotenv

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Parse reads .env-style content from r: one KEY=VALUE pair per line.
// Blank lines and lines starting with '#' (after trimming leading
// whitespace) are ignored. A line may start with "export " before the key.
// Values may be wrapped in matching single or double quotes; double-quoted
// values support \n, \t, \", \\ escape sequences, single-quoted values are
// taken literally. Unquoted values have a trailing " #comment" stripped.
// A non-blank, non-comment line without an "=" is a parse error.
func Parse(r io.Reader) (map[string]string, error) {
	vars := map[string]string{}
	scanner := bufio.NewScanner(r)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("dotenv: line %d: missing '=': %q", lineNo, line)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("dotenv: line %d: empty key", lineNo)
		}
		vars[key] = parseValue(strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return vars, nil
}

// ParseFile opens path and parses it with Parse.
func ParseFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return vars, nil
}

// parseValue strips surrounding quotes (processing escapes for
// double-quoted values) or, for unquoted values, a trailing inline
// "  #comment".
func parseValue(v string) string {
	if len(v) >= 2 {
		if v[0] == '"' && v[len(v)-1] == '"' {
			return unescapeDouble(v[1 : len(v)-1])
		}
		if v[0] == '\'' && v[len(v)-1] == '\'' {
			return v[1 : len(v)-1]
		}
	}
	if idx := strings.Index(v, " #"); idx >= 0 {
		v = strings.TrimSpace(v[:idx])
	}
	return v
}

var doubleQuoteEscapes = strings.NewReplacer(
	`\n`, "\n",
	`\t`, "\t",
	`\"`, `"`,
	`\\`, `\`,
)

func unescapeDouble(s string) string {
	return doubleQuoteEscapes.Replace(s)
}

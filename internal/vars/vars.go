// Package vars implements {{key}} variable substitution used to inject
// environment values into a saved request's URL, headers, and body.
package vars

import "regexp"

var tokenRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

// Apply replaces every {{key}} token in s with vars[key]. Unknown tokens
// are left untouched so the caller can spot unresolved variables.
func Apply(s string, vars map[string]string) string {
	if len(vars) == 0 {
		return s
	}
	return tokenRe.ReplaceAllStringFunc(s, func(tok string) string {
		key := tokenRe.FindStringSubmatch(tok)[1]
		if v, ok := vars[key]; ok {
			return v
		}
		return tok
	})
}

// Merge layers override maps onto a base map, later maps winning. Nil
// maps are skipped. Used to combine an environment's vars with CLI
// --var overrides.
func Merge(layers ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, l := range layers {
		for k, v := range l {
			out[k] = v
		}
	}
	return out
}

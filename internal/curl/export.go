package curl

import (
	"sort"
	"strings"

	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/vars"
)

// singleQuote wraps s in single quotes, safely escaping any embedded
// single-quote characters using the shell idiom: end the single-quoted
// string, emit a literal ' inside double quotes, then restart the
// single-quoted string (e.g. "it's" → 'it'"'"'s').
func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// ToCurl converts req into a curl command string with all {{var}}
// placeholders resolved using v. Headers are emitted in sorted key order
// for deterministic output. Method defaults to GET when empty.
func ToCurl(req model.Request, v map[string]string) string {
	method := req.Method
	if method == "" {
		method = "GET"
	}

	url := vars.Apply(req.URL, v)
	body := vars.Apply(req.Body, v)

	var b strings.Builder
	b.WriteString("curl -X ")
	b.WriteString(method)
	b.WriteString(" ")
	b.WriteString(singleQuote(url))

	// Sort header keys for deterministic output.
	keys := make([]string, 0, len(req.Headers))
	for k := range req.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		val := vars.Apply(req.Headers[k], v)
		b.WriteString(" \\\n  -H ")
		b.WriteString(singleQuote(k + ": " + val))
	}

	if body != "" {
		b.WriteString(" \\\n  --data-raw ")
		b.WriteString(singleQuote(body))
	}

	return b.String()
}

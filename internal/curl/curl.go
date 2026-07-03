// Package curl parses a curl command (as copied from a terminal, API docs,
// or a browser devtools "Copy as cURL") into a model.Request.
package curl

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/melvinsembrano/terman/internal/model"
)

// shortFlagsWithValue are single-dash, single-letter flags that take a
// value, either as the next argument ("-H foo") or glued directly onto
// the flag ("-Hfoo", or "-H'foo'" which the tokenizer merges into one
// token since there's no whitespace between them).
var shortFlagsWithValue = map[byte]bool{
	'X': true,
	'H': true,
	'd': true,
	'u': true,
	'A': true,
	'e': true,
	'b': true,
}

// needsValue lists the canonical (long or short) flag names ParseArgs
// understands that require a value; splitFlag has already extracted an
// inline value ("--flag=value" or glued short form) when possible, so
// this is only consulted when a following argument still needs consuming.
var needsValue = map[string]bool{
	"-X": true, "--request": true,
	"-H": true, "--header": true,
	"-d": true, "--data": true, "--data-raw": true, "--data-ascii": true, "--data-binary": true, "--data-urlencode": true,
	"-u": true, "--user": true,
	"-A": true, "--user-agent": true,
	"-e": true, "--referer": true,
	"-b": true, "--cookie": true,
	"--url": true,
}

// splitFlag extracts the canonical flag name and, if present inline, its
// value from a single argv token: "--flag=value" (GNU long form) or a
// glued short form like "-XPOST"/"-H'Accept: json'".
func splitFlag(a string) (name, value string, hasValue bool) {
	if strings.HasPrefix(a, "--") {
		if idx := strings.Index(a, "="); idx >= 0 {
			return a[:idx], a[idx+1:], true
		}
		return a, "", false
	}
	if len(a) > 2 && a[0] == '-' && shortFlagsWithValue[a[1]] {
		return a[:2], a[2:], true
	}
	return a, "", false
}

// ParseArgs builds a model.Request from an already-tokenized curl argv
// (e.g. os.Args-style, or the output of tokenize). Name is left empty —
// callers set it.
//
// Recognized flags: -X/--request, -H/--header (repeatable),
// -d/--data/--data-raw/--data-ascii/--data-binary/--data-urlencode
// (repeatable, joined with "&"), -u/--user (-> Basic auth header),
// -G/--get (moves the assembled body into the URL's query string),
// -A/--user-agent, -e/--referer, -b/--cookie, and --url. Unrecognized
// flags are silently ignored rather than erroring, since real-world curl
// commands (especially ones copied from browser devtools) are full of
// flags irrelevant to building the request (--compressed, -k, -s, -L,
// --connect-timeout <n>, -o <file>, etc.).
//
// URL detection deliberately does not assume an unrecognized flag takes
// no argument: every bare token not consumed as a known flag's value is
// collected as a URL candidate, and the first one containing "://" wins
// (falling back to the first candidate at all, or the explicit --url
// value if given). This keeps a flag like "--connect-timeout 5" from
// having "5" mistaken for the request URL.
func ParseArgs(args []string) (model.Request, error) {
	if len(args) > 0 && strings.EqualFold(args[0], "curl") {
		args = args[1:]
	}

	headers := map[string]string{}
	var explicitURL string
	var candidates []string
	var dataParts []string
	var method string
	var user string
	var isGet bool

	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			candidates = append(candidates, a)
			continue
		}

		name, value, hasValue := splitFlag(a)
		if needsValue[name] && !hasValue {
			if i+1 >= len(args) {
				return model.Request{}, fmt.Errorf("curl: %s requires a value", name)
			}
			i++
			value = args[i]
		}

		switch name {
		case "-X", "--request":
			method = value
		case "-H", "--header":
			if k, v, ok := strings.Cut(value, ":"); ok {
				headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		case "-d", "--data", "--data-raw", "--data-ascii", "--data-binary", "--data-urlencode":
			dataParts = append(dataParts, value)
		case "-u", "--user":
			user = value
		case "-G", "--get":
			isGet = true
		case "-A", "--user-agent":
			headers["User-Agent"] = value
		case "-e", "--referer":
			headers["Referer"] = value
		case "-b", "--cookie":
			headers["Cookie"] = value
		case "--url":
			explicitURL = value
		default:
			// Unrecognized flag: ignored.
		}
	}

	url := explicitURL
	if url == "" {
		for _, c := range candidates {
			if strings.Contains(c, "://") {
				url = c
				break
			}
		}
	}
	if url == "" && len(candidates) > 0 {
		url = candidates[0]
	}
	if url == "" {
		return model.Request{}, fmt.Errorf("curl: no URL found")
	}

	body := strings.Join(dataParts, "&")
	if isGet && body != "" {
		sep := "?"
		if strings.Contains(url, "?") {
			sep = "&"
		}
		url += sep + body
		body = ""
	}

	if user != "" {
		headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(user))
	}

	if method == "" {
		if body != "" {
			method = "POST"
		} else {
			method = "GET"
		}
	}

	if len(headers) == 0 {
		headers = nil
	}

	return model.Request{
		Method:  strings.ToUpper(method),
		URL:     url,
		Headers: headers,
		Body:    body,
	}, nil
}

// Parse tokenizes a raw curl command string (as pasted from a terminal or
// browser devtools — single/double quotes, backslash escapes, and
// "\"-newline line continuations are all handled) and calls ParseArgs.
func Parse(cmd string) (model.Request, error) {
	args, err := tokenize(cmd)
	if err != nil {
		return model.Request{}, err
	}
	return ParseArgs(args)
}

package curl

import (
	"fmt"
	"strings"
)

// tokenize splits s the way a POSIX-ish shell would: single quotes are
// literal (no escapes), double quotes allow \" and \\ escapes only,
// unquoted text lets a backslash escape the very next character
// (including a newline, which is how shells implement the trailing
// "\"-newline line continuation seen in multi-line curl commands), and
// runs of unquoted whitespace separate tokens.
func tokenize(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	hasCur := false
	n := len(s)

	flush := func() {
		if hasCur {
			tokens = append(tokens, cur.String())
			cur.Reset()
			hasCur = false
		}
	}

	for i := 0; i < n; {
		c := s[i]
		switch {
		case c == '\'':
			hasCur = true
			i++
			j := strings.IndexByte(s[i:], '\'')
			if j < 0 {
				return nil, fmt.Errorf("curl: unterminated '")
			}
			cur.WriteString(s[i : i+j])
			i += j + 1

		case c == '"':
			hasCur = true
			i++
			closed := false
			for i < n {
				if s[i] == '"' {
					closed = true
					i++
					break
				}
				if s[i] == '\\' && i+1 < n && (s[i+1] == '"' || s[i+1] == '\\') {
					cur.WriteByte(s[i+1])
					i += 2
					continue
				}
				cur.WriteByte(s[i])
				i++
			}
			if !closed {
				return nil, fmt.Errorf(`curl: unterminated "`)
			}

		case c == '\\':
			if i+1 < n {
				if s[i+1] != '\n' {
					hasCur = true
					cur.WriteByte(s[i+1])
				}
				i += 2
			} else {
				i++ // trailing backslash with nothing after it; drop it
			}

		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
			i++

		default:
			hasCur = true
			cur.WriteByte(c)
			i++
		}
	}
	flush()
	return tokens, nil
}

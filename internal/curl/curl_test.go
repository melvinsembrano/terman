package curl

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseArgsBasicGET(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "https://example.com/api"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.Method != "GET" {
		t.Errorf("Method = %q, want GET", req.Method)
	}
	if req.URL != "https://example.com/api" {
		t.Errorf("URL = %q", req.URL)
	}
	if req.Headers != nil {
		t.Errorf("Headers = %v, want nil", req.Headers)
	}
}

func TestParseArgsExplicitMethod(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "-X", "PUT", "https://example.com"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.Method != "PUT" {
		t.Errorf("Method = %q, want PUT", req.Method)
	}
}

func TestParseArgsGluedMethod(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "-XPATCH", "https://example.com"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.Method != "PATCH" {
		t.Errorf("Method = %q, want PATCH", req.Method)
	}
}

func TestParseArgsDataImpliesPOST(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "https://example.com", "-d", "a=1"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST (implied by -d)", req.Method)
	}
	if req.Body != "a=1" {
		t.Errorf("Body = %q, want %q", req.Body, "a=1")
	}
}

func TestParseArgsMultipleDataJoinedWithAmpersand(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "https://example.com", "-d", "a=1", "--data", "b=2", "--data-raw", "c=3"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	want := "a=1&b=2&c=3"
	if req.Body != want {
		t.Errorf("Body = %q, want %q", req.Body, want)
	}
}

func TestParseArgsRepeatedHeaders(t *testing.T) {
	req, err := ParseArgs([]string{
		"curl", "https://example.com",
		"-H", "Accept: application/json",
		"-H", "X-Trace-Id:   abc123  ",
	})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.Headers["Accept"] != "application/json" {
		t.Errorf("Headers[Accept] = %q", req.Headers["Accept"])
	}
	if req.Headers["X-Trace-Id"] != "abc123" {
		t.Errorf("Headers[X-Trace-Id] = %q, want trimmed %q", req.Headers["X-Trace-Id"], "abc123")
	}
}

func TestParseArgsMalformedHeaderIsSkipped(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "https://example.com", "-H", "not-a-header"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if len(req.Headers) != 0 {
		t.Errorf("Headers = %v, want empty (malformed header skipped)", req.Headers)
	}
}

func TestParseArgsBasicAuth(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "-u", "alice:s3cret", "https://example.com"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:s3cret"))
	if req.Headers["Authorization"] != want {
		t.Errorf("Headers[Authorization] = %q, want %q", req.Headers["Authorization"], want)
	}
}

func TestParseArgsGetFlagMovesDataToQueryString(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "-G", "https://example.com/search", "-d", "q=cats", "-d", "limit=10"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.Body != "" {
		t.Errorf("Body = %q, want empty (moved to query string)", req.Body)
	}
	want := "https://example.com/search?q=cats&limit=10"
	if req.URL != want {
		t.Errorf("URL = %q, want %q", req.URL, want)
	}
	if req.Method != "GET" {
		t.Errorf("Method = %q, want GET", req.Method)
	}
}

func TestParseArgsGetFlagAppendsToExistingQueryString(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "-G", "https://example.com/search?existing=1", "-d", "q=cats"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	want := "https://example.com/search?existing=1&q=cats"
	if req.URL != want {
		t.Errorf("URL = %q, want %q", req.URL, want)
	}
}

func TestParseArgsExplicitURLFlag(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "--url", "https://example.com/from-flag"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.URL != "https://example.com/from-flag" {
		t.Errorf("URL = %q", req.URL)
	}
}

func TestParseArgsGNUEqualsForm(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "--request=DELETE", "--url=https://example.com", "--header=X-A: 1"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.Method != "DELETE" {
		t.Errorf("Method = %q, want DELETE", req.Method)
	}
	if req.URL != "https://example.com" {
		t.Errorf("URL = %q", req.URL)
	}
	if req.Headers["X-A"] != "1" {
		t.Errorf("Headers[X-A] = %q, want %q", req.Headers["X-A"], "1")
	}
}

func TestParseArgsUnknownFlagsIgnored(t *testing.T) {
	req, err := ParseArgs([]string{
		"curl", "-s", "-S", "-L", "-k", "--compressed", "-v", "-i",
		"https://example.com",
	})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.URL != "https://example.com" {
		t.Errorf("URL = %q", req.URL)
	}
	if req.Method != "GET" {
		t.Errorf("Method = %q, want GET", req.Method)
	}
}

func TestParseArgsUnknownValueFlagDoesNotHijackURL(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "--connect-timeout", "5", "https://example.com"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.URL != "https://example.com" {
		t.Errorf("URL = %q, want the real URL, not the unrecognized flag's value", req.URL)
	}
}

func TestParseArgsUnknownValueFlagBeforeURLDoesNotHijackURL(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "-o", "out.txt", "https://example.com"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.URL != "https://example.com" {
		t.Errorf("URL = %q, want the real URL, not the unrecognized flag's value", req.URL)
	}
}

func TestParseArgsFallsBackToFirstCandidateWithoutScheme(t *testing.T) {
	req, err := ParseArgs([]string{"curl", "example.com/api", "-d", "a=1"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.URL != "example.com/api" {
		t.Errorf("URL = %q, want %q", req.URL, "example.com/api")
	}
}

func TestParseArgsNoLeadingCurlWord(t *testing.T) {
	req, err := ParseArgs([]string{"-X", "POST", "https://example.com"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if req.Method != "POST" || req.URL != "https://example.com" {
		t.Errorf("req = %+v", req)
	}
}

func TestParseArgsMissingURLIsError(t *testing.T) {
	_, err := ParseArgs([]string{"curl", "-X", "GET"})
	if err == nil {
		t.Error("expected an error when no URL can be found")
	}
}

func TestParseArgsMissingFlagValueIsError(t *testing.T) {
	_, err := ParseArgs([]string{"curl", "https://example.com", "-H"})
	if err == nil {
		t.Error("expected an error when a value-requiring flag has nothing after it")
	}
}

func TestParseRealBrowserStyleMultilineCommand(t *testing.T) {
	cmd := "curl 'https://api.example.com/v1/users' \\\n" +
		"  -H 'accept: application/json' \\\n" +
		"  -H 'content-type: application/json' \\\n" +
		"  -H 'authorization: Bearer abc123' \\\n" +
		"  --data-raw '{\"name\":\"Ada\"}' \\\n" +
		"  --compressed"

	req, err := Parse(cmd)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST (implied by --data-raw)", req.Method)
	}
	if req.URL != "https://api.example.com/v1/users" {
		t.Errorf("URL = %q", req.URL)
	}
	if req.Headers["accept"] != "application/json" {
		t.Errorf("Headers[accept] = %q", req.Headers["accept"])
	}
	if req.Headers["authorization"] != "Bearer abc123" {
		t.Errorf("Headers[authorization] = %q", req.Headers["authorization"])
	}
	if req.Body != `{"name":"Ada"}` {
		t.Errorf("Body = %q, want %q", req.Body, `{"name":"Ada"}`)
	}
	if strings.Contains(req.URL, "compressed") {
		t.Errorf("--compressed leaked into the URL: %q", req.URL)
	}
}

func TestParsePropagatesTokenizeError(t *testing.T) {
	_, err := Parse(`curl https://example.com -d 'unterminated`)
	if err == nil {
		t.Error("expected an error from an unterminated quote")
	}
}

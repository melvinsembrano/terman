package httpx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/melvinsembrano/terman/internal/model"
)

// echoServer records the last request it received and responds with the
// given content type and body.
type echoServer struct {
	*httptest.Server
	lastMethod  string
	lastPath    string
	lastQuery   string
	lastBody    string
	lastHeaders http.Header
}

func newEchoServer(t *testing.T, contentType, respBody string, status int) *echoServer {
	t.Helper()
	es := &echoServer{}
	es.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		es.lastMethod = r.Method
		es.lastPath = r.URL.Path
		es.lastQuery = r.URL.RawQuery
		es.lastHeaders = r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		es.lastBody = string(body)

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(es.Close)
	return es
}

func TestDoDefaultsMethodToGET(t *testing.T) {
	srv := newEchoServer(t, "text/plain", "ok", http.StatusOK)

	req := model.Request{URL: srv.URL} // Method left blank.
	if _, err := Do(req, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if srv.lastMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", srv.lastMethod)
	}
}

func TestDoSubstitutesVarsInURLBodyAndHeaders(t *testing.T) {
	srv := newEchoServer(t, "text/plain", "ok", http.StatusOK)

	req := model.Request{
		Method:  "POST",
		URL:     srv.URL + "/{{path}}?msg={{msg}}",
		Headers: map[string]string{"X-Test": "{{msg}}"},
		Body:    "hello {{msg}}",
	}
	v := map[string]string{"path": "widgets", "msg": "world"}

	if _, err := Do(req, v); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if srv.lastPath != "/widgets" {
		t.Errorf("path = %q, want /widgets", srv.lastPath)
	}
	if srv.lastQuery != "msg=world" {
		t.Errorf("query = %q, want msg=world", srv.lastQuery)
	}
	if got := srv.lastHeaders.Get("X-Test"); got != "world" {
		t.Errorf("X-Test header = %q, want world", got)
	}
	if srv.lastBody != "hello world" {
		t.Errorf("body = %q, want %q", srv.lastBody, "hello world")
	}
}

func TestDoCapturesResponse(t *testing.T) {
	srv := newEchoServer(t, "application/json", `{"a":1}`, http.StatusCreated)

	resp, err := Do(model.Request{URL: srv.URL}, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if !strings.Contains(resp.Status, "201") {
		t.Errorf("Status = %q, want to contain 201", resp.Status)
	}
	if resp.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", resp.Duration)
	}
	wantBody := "{\n  \"a\": 1\n}"
	if resp.Body != wantBody {
		t.Errorf("Body = %q, want %q", resp.Body, wantBody)
	}
}

func TestDoErrorOnUnreachableHost(t *testing.T) {
	// Port 0 on loopback is not listening; the dial should fail fast.
	_, err := Do(model.Request{URL: "http://127.0.0.1:0/"}, nil)
	if err == nil {
		t.Fatal("expected an error for an unreachable host, got nil")
	}
}

func TestPrettyBody(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		raw         string
		want        string
	}{
		{"empty", "application/json", "", ""},
		{"whitespace only", "text/plain", "   \n  ", ""},
		{"json indents", "application/json; charset=utf-8", `{"a":1,"b":[1,2]}`, "{\n  \"a\": 1,\n  \"b\": [\n    1,\n    2\n  ]\n}"},
		{"invalid json passes through", "application/json", "not json", "not json"},
		{"non-json passes through trimmed", "text/plain", "  hello world  \n", "hello world"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := prettyBody(c.contentType, []byte(c.raw))
			if got != c.want {
				t.Errorf("prettyBody(%q, %q) = %q, want %q", c.contentType, c.raw, got, c.want)
			}
		})
	}
}

func TestResponseHeadersString(t *testing.T) {
	resp := Response{Headers: http.Header{
		"Content-Type": {"application/json"},
		"X-Multi":      {"a", "b"},
	}}
	got := resp.HeadersString()
	want := "Content-Type: application/json\nX-Multi: a, b\n"
	if got != want {
		t.Errorf("HeadersString() = %q, want %q", got, want)
	}
}

func TestResponseHeadersStringEmpty(t *testing.T) {
	resp := Response{}
	if got := resp.HeadersString(); got != "" {
		t.Errorf("HeadersString() = %q, want empty", got)
	}
}

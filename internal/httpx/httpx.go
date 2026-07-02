// Package httpx builds and executes an http.Request from a saved
// model.Request, substituting variables and capturing the response.
package httpx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/vars"
)

const defaultTimeout = 30 * time.Second

// Response captures the result of executing a request.
type Response struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       string
	Duration   time.Duration
}

// Do substitutes vars into req, executes it, and returns the response.
func Do(req model.Request, v map[string]string) (Response, error) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	url := vars.Apply(req.URL, v)
	body := vars.Apply(req.Body, v)

	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}

	httpReq, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}
	for k, val := range req.Headers {
		httpReq.Header.Set(k, vars.Apply(val, v))
	}

	client := &http.Client{Timeout: defaultTimeout}
	start := time.Now()
	resp, err := client.Do(httpReq)
	duration := time.Since(start)
	if err != nil {
		return Response{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response body: %w", err)
	}

	return Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header,
		Body:       prettyBody(resp.Header.Get("Content-Type"), raw),
		Duration:   duration,
	}, nil
}

// prettyBody indents JSON response bodies for readability; anything else
// is returned as-is (trimmed).
func prettyBody(contentType string, raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return ""
	}
	if strings.Contains(contentType, "json") {
		var buf bytes.Buffer
		if err := json.Indent(&buf, raw, "", "  "); err == nil {
			return buf.String()
		}
	}
	return s
}

// HeadersString renders response headers as sorted "Key: Value" lines.
func (r Response) HeadersString() string {
	keys := make([]string, 0, len(r.Headers))
	for k := range r.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %s\n", k, strings.Join(r.Headers[k], ", "))
	}
	return b.String()
}

package model

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRequestYAMLRoundTrip(t *testing.T) {
	req := Request{
		Name:    "Get Widget",
		Method:  "GET",
		URL:     "https://api.example.com/widgets/{{id}}",
		Headers: map[string]string{"Authorization": "Bearer {{token}}"},
		Body:    `{"note":"hi"}`,
	}

	data, err := yaml.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Request
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Name != req.Name || got.Method != req.Method || got.URL != req.URL || got.Body != req.Body {
		t.Errorf("round-tripped Request = %+v, want %+v", got, req)
	}
	if got.Headers["Authorization"] != req.Headers["Authorization"] {
		t.Errorf("round-tripped Headers = %v, want %v", got.Headers, req.Headers)
	}
}

func TestRequestYAMLOmitsEmptyFields(t *testing.T) {
	req := Request{Name: "Bare", Method: "GET", URL: "https://example.com"}

	data, err := yaml.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "headers:") {
		t.Errorf("expected no headers key in output, got:\n%s", s)
	}
	if strings.Contains(s, "body:") {
		t.Errorf("expected no body key in output, got:\n%s", s)
	}
}

func TestEnvironmentYAMLRoundTrip(t *testing.T) {
	env := Environment{
		Name: "dev",
		Vars: map[string]string{"base_url": "https://dev.example.com"},
	}

	data, err := yaml.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Environment
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Name != env.Name {
		t.Errorf("round-tripped Name = %q, want %q", got.Name, env.Name)
	}
	if got.Vars["base_url"] != env.Vars["base_url"] {
		t.Errorf("round-tripped Vars = %v, want %v", got.Vars, env.Vars)
	}
}

func TestEnvironmentYAMLOmitsEmptyVars(t *testing.T) {
	env := Environment{Name: "empty"}

	data, err := yaml.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "vars:") {
		t.Errorf("expected no vars key in output, got:\n%s", string(data))
	}
}

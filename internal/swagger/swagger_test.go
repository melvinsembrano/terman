package swagger_test

import (
	"strings"
	"testing"

	"github.com/melvinsembrano/terman/internal/swagger"
)

// ─────────────────────────────────────────────
// OAS2 (Swagger) tests
// ─────────────────────────────────────────────

const oas2YAML = `
swagger: "2.0"
host: api.example.com
basePath: /v1
schemes:
  - https
securityDefinitions:
  bearer:
    type: apiKey
paths:
  /users:
    get:
      operationId: listUsers
      parameters:
        - name: page
          in: query
          type: integer
          default: "1"
        - name: limit
          in: query
          type: integer
  /users/{userId}:
    parameters:
      - name: userId
        in: path
        type: string
    get:
      operationId: getUser
    put:
      operationId: updateUser
      parameters:
        - name: body
          in: body
          schema:
            properties:
              name:
                type: string
              email:
                type: string
`

func TestParseOAS2_BaseURL(t *testing.T) {
	res, err := swagger.Parse([]byte(oas2YAML), "spec.yaml", "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := res.Environment.Vars["base_url"]
	want := "https://api.example.com/v1"
	if got != want {
		t.Errorf("base_url = %q, want %q", got, want)
	}
}

func TestParseOAS2_AuthToken(t *testing.T) {
	res, err := swagger.Parse([]byte(oas2YAML), "spec.yaml", "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := res.Environment.Vars["auth_token"]; !ok {
		t.Error("expected auth_token var to be present when securityDefinitions exists")
	}
}

func TestParseOAS2_RequestCount(t *testing.T) {
	res, err := swagger.Parse([]byte(oas2YAML), "spec.yaml", "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Requests) != 3 {
		t.Errorf("got %d requests, want 3", len(res.Requests))
	}
}

func TestParseOAS2_QueryParams(t *testing.T) {
	res, err := swagger.Parse([]byte(oas2YAML), "spec.yaml", "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var req *struct{ URL string }
	for _, r := range res.Requests {
		if r.Name == "listUsers" {
			req = &struct{ URL string }{r.URL}
			break
		}
	}
	if req == nil {
		t.Fatal("listUsers request not found")
	}
	if !strings.Contains(req.URL, "{{base_url}}") {
		t.Errorf("URL %q missing {{base_url}}", req.URL)
	}
	if !strings.Contains(req.URL, "limit={{limit}}") {
		t.Errorf("URL %q missing limit query param", req.URL)
	}
	if !strings.Contains(req.URL, "page={{page}}") {
		t.Errorf("URL %q missing page query param", req.URL)
	}
}

func TestParseOAS2_PathParams(t *testing.T) {
	res, err := swagger.Parse([]byte(oas2YAML), "spec.yaml", "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range res.Requests {
		if r.Name == "getUser" {
			if !strings.Contains(r.URL, "{{user_id}}") {
				t.Errorf("getUser URL %q missing {{user_id}}", r.URL)
			}
			if strings.Contains(r.URL, "{userId}") {
				t.Errorf("getUser URL %q still contains raw {userId}", r.URL)
			}
			return
		}
	}
	t.Fatal("getUser request not found")
}

func TestParseOAS2_BodyTemplate(t *testing.T) {
	res, err := swagger.Parse([]byte(oas2YAML), "spec.yaml", "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range res.Requests {
		if r.Name == "updateUser" {
			if !strings.Contains(r.Body, "{{name}}") {
				t.Errorf("updateUser body %q missing {{name}}", r.Body)
			}
			if !strings.Contains(r.Body, "{{email}}") {
				t.Errorf("updateUser body %q missing {{email}}", r.Body)
			}
			if r.Headers["Content-Type"] != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", r.Headers["Content-Type"])
			}
			return
		}
	}
	t.Fatal("updateUser request not found")
}

func TestParseOAS2_Methods(t *testing.T) {
	res, err := swagger.Parse([]byte(oas2YAML), "spec.yaml", "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	methods := map[string]string{}
	for _, r := range res.Requests {
		methods[r.Name] = r.Method
	}
	cases := map[string]string{
		"listUsers":  "GET",
		"getUser":    "GET",
		"updateUser": "PUT",
	}
	for name, want := range cases {
		if methods[name] != want {
			t.Errorf("request %q method = %q, want %q", name, methods[name], want)
		}
	}
}

func TestParseOAS2_EnvName(t *testing.T) {
	res, err := swagger.Parse([]byte(oas2YAML), "spec.yaml", "my-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Environment.Name != "my-env" {
		t.Errorf("environment name = %q, want %q", res.Environment.Name, "my-env")
	}
}

// ─────────────────────────────────────────────
// OAS3 tests
// ─────────────────────────────────────────────

const oas3YAML = `
openapi: "3.0.0"
info:
  title: Test API
  version: "1.0"
servers:
  - url: https://api.example.com/v2
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
paths:
  /items:
    post:
      operationId: createItem
      requestBody:
        content:
          application/json:
            schema:
              properties:
                title:
                  type: string
                price:
                  type: number
      security:
        - bearerAuth: []
  /items/{itemId}:
    get:
      operationId: getItem
      parameters:
        - name: itemId
          in: path
          schema:
            type: string
        - name: format
          in: query
          schema:
            type: string
            example: json
`

func TestParseOAS3_BaseURL(t *testing.T) {
	res, err := swagger.Parse([]byte(oas3YAML), "spec.yaml", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := res.Environment.Vars["base_url"]
	want := "https://api.example.com/v2"
	if got != want {
		t.Errorf("base_url = %q, want %q", got, want)
	}
}

func TestParseOAS3_RequestCount(t *testing.T) {
	res, err := swagger.Parse([]byte(oas3YAML), "spec.yaml", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Requests) != 2 {
		t.Errorf("got %d requests, want 2", len(res.Requests))
	}
}

func TestParseOAS3_SecurityHeader(t *testing.T) {
	res, err := swagger.Parse([]byte(oas3YAML), "spec.yaml", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range res.Requests {
		if r.Name == "createItem" {
			if r.Headers["Authorization"] != "Bearer {{auth_token}}" {
				t.Errorf("Authorization = %q, want Bearer {{auth_token}}", r.Headers["Authorization"])
			}
			return
		}
	}
	t.Fatal("createItem request not found")
}

func TestParseOAS3_PathAndQueryParams(t *testing.T) {
	res, err := swagger.Parse([]byte(oas3YAML), "spec.yaml", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range res.Requests {
		if r.Name == "getItem" {
			if !strings.Contains(r.URL, "{{item_id}}") {
				t.Errorf("getItem URL %q missing {{item_id}}", r.URL)
			}
			if !strings.Contains(r.URL, "format={{format}}") {
				t.Errorf("getItem URL %q missing format query param", r.URL)
			}
			if v, ok := res.Environment.Vars["format"]; !ok || v != "json" {
				t.Errorf("format env var = %q (ok=%v), want \"json\"", v, ok)
			}
			return
		}
	}
	t.Fatal("getItem request not found")
}

func TestParseOAS3_BodyVars(t *testing.T) {
	res, err := swagger.Parse([]byte(oas3YAML), "spec.yaml", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range res.Requests {
		if r.Name == "createItem" {
			if !strings.Contains(r.Body, "{{title}}") {
				t.Errorf("createItem body %q missing {{title}}", r.Body)
			}
			if !strings.Contains(r.Body, "{{price}}") {
				t.Errorf("createItem body %q missing {{price}}", r.Body)
			}
			if _, ok := res.Environment.Vars["price"]; !ok {
				t.Error("price var not in environment")
			}
			return
		}
	}
	t.Fatal("createItem request not found")
}

// ─────────────────────────────────────────────
// JSON format
// ─────────────────────────────────────────────

const oas3JSON = `{
  "openapi": "3.0.0",
  "info": {"title": "JSON API", "version": "1"},
  "servers": [{"url": "https://json.example.com"}],
  "paths": {
    "/ping": {
      "get": {
        "operationId": "ping"
      }
    }
  }
}`

func TestParseJSON(t *testing.T) {
	res, err := swagger.Parse([]byte(oas3JSON), "spec.json", "json-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Requests) != 1 || res.Requests[0].Name != "ping" {
		t.Errorf("unexpected requests: %+v", res.Requests)
	}
	if res.Environment.Vars["base_url"] != "https://json.example.com" {
		t.Errorf("base_url = %q", res.Environment.Vars["base_url"])
	}
}

// ─────────────────────────────────────────────
// Global security propagation
// ─────────────────────────────────────────────

// OAS3: top-level security array → every operation gets Authorization header.
const oas3GlobalSecurityYAML = `
openapi: "3.0.0"
info:
  title: Test API
  version: "1.0"
servers:
  - url: https://api.example.com
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
security:
  - BearerAuth: []
paths:
  /users:
    get:
      operationId: listUsers
  /users/{userId}:
    get:
      operationId: getUser
      parameters:
        - name: userId
          in: path
          schema:
            type: string
  /public:
    get:
      operationId: publicEndpoint
      security: []
`

func TestParseOAS3_GlobalSecurityAddsAuthHeader(t *testing.T) {
	res, err := swagger.Parse([]byte(oas3GlobalSecurityYAML), "spec.yaml", "env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range res.Requests {
		switch r.Name {
		case "listUsers", "getUser":
			if r.Headers["Authorization"] != "Bearer {{auth_token}}" {
				t.Errorf("request %q: Authorization = %q, want \"Bearer {{auth_token}}\"", r.Name, r.Headers["Authorization"])
			}
		case "publicEndpoint":
			if r.Headers["Authorization"] != "" {
				t.Errorf("request %q: Authorization should be absent (security: []), got %q", r.Name, r.Headers["Authorization"])
			}
		}
	}
}

func TestParseOAS3_GlobalSecuritySeedsAuthTokenVar(t *testing.T) {
	res, err := swagger.Parse([]byte(oas3GlobalSecurityYAML), "spec.yaml", "env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := res.Environment.Vars["auth_token"]; !ok {
		t.Error("expected auth_token var when global security is set")
	}
}

// OAS2: top-level security array → every operation gets Authorization header.
const oas2GlobalSecurityYAML = `
swagger: "2.0"
host: api.example.com
basePath: /v1
securityDefinitions:
  bearer:
    type: apiKey
    in: header
    name: Authorization
security:
  - bearer: []
paths:
  /items:
    get:
      operationId: listItems
  /items/{id}:
    delete:
      operationId: deleteItem
      parameters:
        - name: id
          in: path
          type: string
  /open:
    get:
      operationId: openEndpoint
      security: []
`

func TestParseOAS2_GlobalSecurityAddsAuthHeader(t *testing.T) {
	res, err := swagger.Parse([]byte(oas2GlobalSecurityYAML), "spec.yaml", "env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range res.Requests {
		switch r.Name {
		case "listItems", "deleteItem":
			if r.Headers["Authorization"] != "Bearer {{auth_token}}" {
				t.Errorf("request %q: Authorization = %q, want \"Bearer {{auth_token}}\"", r.Name, r.Headers["Authorization"])
			}
		case "openEndpoint":
			if r.Headers["Authorization"] != "" {
				t.Errorf("request %q: Authorization should be absent (security: []), got %q", r.Name, r.Headers["Authorization"])
			}
		}
	}
}

// ─────────────────────────────────────────────
// Error cases
// ─────────────────────────────────────────────

func TestParseUnknownSpec(t *testing.T) {
	_, err := swagger.Parse([]byte("foo: bar\n"), "spec.yaml", "env")
	if err == nil {
		t.Error("expected error for unknown spec format")
	}
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := swagger.Parse([]byte(":\tbad\tyaml:::"), "spec.yaml", "env")
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

// ─────────────────────────────────────────────
// slugVar helper (tested indirectly via path params)
// ─────────────────────────────────────────────

func TestSlugVarViaPathParam(t *testing.T) {
	// "user-id" should become "user_id" as a var name
	spec := `
swagger: "2.0"
host: h.example.com
paths:
  /foo/{user-id}:
    get:
      operationId: getFoo
      parameters:
        - name: user-id
          in: path
          type: string
`
	res, err := swagger.Parse([]byte(spec), "spec.yaml", "e")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Requests) == 0 {
		t.Fatal("no requests")
	}
	url := res.Requests[0].URL
	if !strings.Contains(url, "{{user_id}}") {
		t.Errorf("URL %q: expected {{user_id}} from param name 'user-id'", url)
	}
}

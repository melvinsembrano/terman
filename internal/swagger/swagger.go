// Package swagger parses OpenAPI 2.x (Swagger) and OpenAPI 3.x specification
// files and converts them into terman Request and Environment values.
//
// All variable parts of a request (base URL, path parameters, query
// parameters, top-level request-body fields) are replaced with {{var}}
// placeholders so that the same set of requests can be used across multiple
// environments (e.g. dev, staging, production) simply by swapping the active
// environment.
package swagger

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/melvinsembrano/terman/internal/model"
	"gopkg.in/yaml.v3"
)

// Result holds everything produced by parsing a swagger/openapi file.
type Result struct {
	Requests    []model.Request
	Environment model.Environment
}

// Parse reads a Swagger/OpenAPI file (JSON or YAML, auto-detected by
// extension; YAML is the fallback) and returns the extracted requests and a
// matching environment. The envName is used as the Environment.Name.
func Parse(data []byte, filename, envName string) (Result, error) {
	var raw map[string]interface{}
	if err := unmarshal(data, filename, &raw); err != nil {
		return Result{}, fmt.Errorf("parse %s: %w", filename, err)
	}

	// Detect spec version.
	if _, ok := raw["swagger"]; ok {
		return parseOAS2(raw, envName)
	}
	if _, ok := raw["openapi"]; ok {
		return parseOAS3(raw, envName)
	}
	return Result{}, fmt.Errorf("unrecognised spec: missing top-level \"swagger\" or \"openapi\" key")
}

// unmarshal decodes JSON or YAML based on the filename extension (.json →
// JSON, anything else → YAML).
func unmarshal(data []byte, filename string, v interface{}) error {
	if strings.EqualFold(filepath.Ext(filename), ".json") {
		return json.Unmarshal(data, v)
	}
	return yaml.Unmarshal(data, v)
}

// ─────────────────────────────────────────────
// OpenAPI 2.x (Swagger)
// ─────────────────────────────────────────────

func parseOAS2(raw map[string]interface{}, envName string) (Result, error) {
	vars := map[string]string{}

	// Base URL: scheme://host/basePath
	host := stringField(raw, "host")
	basePath := stringField(raw, "basePath")
	if basePath == "/" {
		basePath = ""
	}
	scheme := "https"
	if schemes, ok := raw["schemes"].([]interface{}); ok && len(schemes) > 0 {
		scheme = fmt.Sprintf("%v", schemes[0])
	}
	baseURL := fmt.Sprintf("%s://%s%s", scheme, host, basePath)
	if host == "" {
		baseURL = ""
	}
	vars["base_url"] = baseURL

	// Security → auth_token placeholder
	if sec := raw["securityDefinitions"]; sec != nil {
		vars["auth_token"] = ""
	}

	paths, _ := raw["paths"].(map[string]interface{})
	reqs, err := buildRequests(paths, vars, "2")
	if err != nil {
		return Result{}, err
	}

	return Result{
		Requests:    reqs,
		Environment: model.Environment{Name: envName, Vars: vars},
	}, nil
}

// ─────────────────────────────────────────────
// OpenAPI 3.x
// ─────────────────────────────────────────────

func parseOAS3(raw map[string]interface{}, envName string) (Result, error) {
	vars := map[string]string{}

	// Base URL from first server entry
	baseURL := ""
	if servers, ok := raw["servers"].([]interface{}); ok && len(servers) > 0 {
		if srv, ok := servers[0].(map[string]interface{}); ok {
			baseURL = stringField(srv, "url")
			// Substitute server variables if present
			if srvVars, ok := srv["variables"].(map[string]interface{}); ok {
				for k, v := range srvVars {
					def := ""
					if vm, ok := v.(map[string]interface{}); ok {
						def = stringField(vm, "default")
					}
					// Replace {varName} in the URL with our {{varName}} syntax
					baseURL = strings.ReplaceAll(baseURL, "{"+k+"}", "{{"+k+"}}")
					vars[k] = def
				}
			}
		}
	}
	vars["base_url"] = baseURL

	// Security → auth_token placeholder
	if raw["components"] != nil {
		if comp, ok := raw["components"].(map[string]interface{}); ok {
			if comp["securitySchemes"] != nil {
				vars["auth_token"] = ""
			}
		}
	}

	paths, _ := raw["paths"].(map[string]interface{})
	reqs, err := buildRequests(paths, vars, "3")
	if err != nil {
		return Result{}, err
	}

	return Result{
		Requests:    reqs,
		Environment: model.Environment{Name: envName, Vars: vars},
	}, nil
}

// ─────────────────────────────────────────────
// Shared request building
// ─────────────────────────────────────────────

var httpMethods = []string{"get", "post", "put", "patch", "delete", "head", "options"}

// buildRequests iterates over all path+method combinations and builds
// model.Request values. vars is mutated to accumulate discovered variable
// names. specVersion is "2" or "3" (affects where request body lives).
func buildRequests(paths map[string]interface{}, vars map[string]string, specVersion string) ([]model.Request, error) {
	var reqs []model.Request

	// Sort paths for deterministic output.
	pathKeys := make([]string, 0, len(paths))
	for p := range paths {
		pathKeys = append(pathKeys, p)
	}
	sort.Strings(pathKeys)

	for _, path := range pathKeys {
		pathItem, ok := paths[path].(map[string]interface{})
		if !ok {
			continue
		}

		// Path-level parameters (shared across all methods on this path).
		pathLevelParams, _ := pathItem["parameters"].([]interface{})

		for _, method := range httpMethods {
			opRaw, ok := pathItem[method]
			if !ok {
				continue
			}
			op, ok := opRaw.(map[string]interface{})
			if !ok {
				continue
			}

			req, err := buildRequest(path, method, op, pathLevelParams, vars, specVersion)
			if err != nil {
				return nil, err
			}
			reqs = append(reqs, req)
		}
	}
	return reqs, nil
}

// buildRequest constructs one model.Request from a single operation object.
func buildRequest(
	path, method string,
	op map[string]interface{},
	pathLevelParams []interface{},
	vars map[string]string,
	specVersion string,
) (model.Request, error) {
	// Request name: operationId or "METHOD /path"
	name := stringField(op, "operationId")
	if name == "" {
		name = strings.ToUpper(method) + " " + path
	}

	// Merge path-level and operation-level parameters.
	opParams, _ := op["parameters"].([]interface{})
	params := mergeParams(pathLevelParams, opParams)

	// Build URL: replace {param} with {{param}} and collect query params.
	urlPath := path
	var queryParts []string

	for _, pRaw := range params {
		p, ok := pRaw.(map[string]interface{})
		if !ok {
			continue
		}
		pName := stringField(p, "name")
		pIn := stringField(p, "in")
		if pName == "" {
			continue
		}
		varName := slugVar(pName)
		example := exampleValue(p, specVersion)

		switch pIn {
		case "path":
			urlPath = strings.ReplaceAll(urlPath, "{"+pName+"}", "{{"+varName+"}}")
			if _, exists := vars[varName]; !exists {
				vars[varName] = example
			}
		case "query":
			queryParts = append(queryParts, pName+"={{"+varName+"}}")
			if _, exists := vars[varName]; !exists {
				vars[varName] = example
			}
		case "header":
			// Header params are handled below when building headers.
		}
	}

	fullURL := "{{base_url}}" + urlPath
	if len(queryParts) > 0 {
		sort.Strings(queryParts)
		fullURL += "?" + strings.Join(queryParts, "&")
	}

	// Headers
	headers := map[string]string{}

	// Security hint → Authorization header
	if op["security"] != nil || op["x-security-scopes"] != nil {
		headers["Authorization"] = "Bearer {{auth_token}}"
	}

	// Header parameters
	for _, pRaw := range params {
		p, ok := pRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if stringField(p, "in") != "header" {
			continue
		}
		pName := stringField(p, "name")
		varName := slugVar(pName)
		headers[pName] = "{{" + varName + "}}"
		if _, exists := vars[varName]; !exists {
			vars[varName] = exampleValue(p, specVersion)
		}
	}

	// Body
	body := ""
	contentType := ""

	if specVersion == "2" {
		body, contentType = buildBodyOAS2(op, params, vars)
	} else {
		body, contentType = buildBodyOAS3(op, vars)
	}

	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	if len(headers) == 0 {
		headers = nil
	}

	return model.Request{
		Name:    name,
		Method:  strings.ToUpper(method),
		URL:     fullURL,
		Headers: headers,
		Body:    body,
	}, nil
}

// buildBodyOAS2 handles OAS2 body/formData parameters.
func buildBodyOAS2(op map[string]interface{}, params []interface{}, vars map[string]string) (body, contentType string) {
	for _, pRaw := range params {
		p, ok := pRaw.(map[string]interface{})
		if !ok {
			continue
		}
		switch stringField(p, "in") {
		case "body":
			schema, _ := p["schema"].(map[string]interface{})
			body = schemaToBody(schema, vars)
			contentType = "application/json"
			return
		case "formData":
			// Collect all formData params into a simple key={{var}} body.
			pName := stringField(p, "name")
			varName := slugVar(pName)
			if body != "" {
				body += "&"
			}
			body += pName + "={{" + varName + "}}"
			if _, exists := vars[varName]; !exists {
				vars[varName] = exampleValue(p, "2")
			}
			contentType = "application/x-www-form-urlencoded"
		}
	}
	return
}

// buildBodyOAS3 handles OAS3 requestBody.
func buildBodyOAS3(op map[string]interface{}, vars map[string]string) (body, contentType string) {
	rb, ok := op["requestBody"].(map[string]interface{})
	if !ok {
		return
	}
	content, ok := rb["content"].(map[string]interface{})
	if !ok {
		return
	}

	// Prefer JSON, fall back to first available type.
	ct := ""
	for _, prefer := range []string{"application/json", "application/x-www-form-urlencoded"} {
		if _, ok := content[prefer]; ok {
			ct = prefer
			break
		}
	}
	if ct == "" {
		for k := range content {
			ct = k
			break
		}
	}
	if ct == "" {
		return
	}

	mediaType, ok := content[ct].(map[string]interface{})
	if !ok {
		return
	}
	schema, _ := mediaType["schema"].(map[string]interface{})
	body = schemaToBody(schema, vars)
	contentType = ct
	return
}

// schemaToBody generates a JSON template from a schema's top-level properties,
// replacing each value with a {{var}} placeholder.
func schemaToBody(schema map[string]interface{}, vars map[string]string) string {
	if schema == nil {
		return ""
	}

	// Handle $ref — we don't resolve refs, so just return empty.
	if _, ok := schema["$ref"]; ok {
		return ""
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok || len(props) == 0 {
		return ""
	}

	// Sort property names for deterministic output.
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		varName := slugVar(k)
		if _, exists := vars[varName]; !exists {
			// Use the property's example or default as the env value placeholder.
			propSchema, _ := props[k].(map[string]interface{})
			vars[varName] = exampleFromSchema(propSchema)
		}
		parts = append(parts, fmt.Sprintf("  %q: {{%s}}", k, varName))
	}

	return "{\n" + strings.Join(parts, ",\n") + "\n}"
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

// mergeParams combines path-level and operation-level parameters, with
// operation-level taking precedence (matched by name+in).
func mergeParams(pathLevel, opLevel []interface{}) []interface{} {
	merged := make([]interface{}, 0, len(pathLevel)+len(opLevel))
	merged = append(merged, pathLevel...)
outer:
	for _, op := range opLevel {
		opm, ok := op.(map[string]interface{})
		if !ok {
			merged = append(merged, op)
			continue
		}
		for i, pl := range merged {
			plm, ok := pl.(map[string]interface{})
			if !ok {
				continue
			}
			if stringField(opm, "name") == stringField(plm, "name") &&
				stringField(opm, "in") == stringField(plm, "in") {
				merged[i] = op // override
				continue outer
			}
		}
		merged = append(merged, op)
	}
	return merged
}

// slugVar converts a parameter name like "userId" or "user-id" into a safe
// env var name like "user_id". It splits on camelCase boundaries, lowercases,
// and replaces non-alnum chars with underscores, collapsing runs.
func slugVar(name string) string {
	// Insert underscore before each uppercase letter that follows a lowercase
	// letter or digit (camelCase → snake_case).
	var expanded strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
				expanded.WriteRune('_')
			}
		}
		expanded.WriteRune(r)
	}

	var b strings.Builder
	prev := '_'
	for _, r := range strings.ToLower(expanded.String()) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prev = r
		} else if prev != '_' {
			b.WriteRune('_')
			prev = '_'
		}
	}
	s := strings.Trim(b.String(), "_")
	if s == "" {
		return "param"
	}
	return s
}

// stringField safely reads a string from a map, returning "" if missing or
// not a string.
func stringField(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// exampleValue returns an example/default value string for a parameter object.
func exampleValue(p map[string]interface{}, specVersion string) string {
	if v := stringField(p, "example"); v != "" {
		return v
	}
	if v := stringField(p, "default"); v != "" {
		return v
	}
	if specVersion == "3" {
		if schema, ok := p["schema"].(map[string]interface{}); ok {
			return exampleFromSchema(schema)
		}
	}
	return exampleFromType(stringField(p, "type"))
}

// exampleFromSchema returns a simple placeholder value for a JSON schema node.
func exampleFromSchema(schema map[string]interface{}) string {
	if schema == nil {
		return ""
	}
	if v := stringField(schema, "example"); v != "" {
		return v
	}
	if v := stringField(schema, "default"); v != "" {
		return v
	}
	return exampleFromType(stringField(schema, "type"))
}

// exampleFromType returns a simple type-based placeholder.
func exampleFromType(t string) string {
	switch t {
	case "integer", "number":
		return "0"
	case "boolean":
		return "false"
	case "array":
		return "[]"
	default:
		return ""
	}
}

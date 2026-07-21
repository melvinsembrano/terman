package curl

import (
	"testing"

	"github.com/melvinsembrano/terman/internal/model"
)

func TestToCurl(t *testing.T) {
	tests := []struct {
		name string
		req  model.Request
		vars map[string]string
		want string
	}{
		{
			name: "simple GET",
			req:  model.Request{Method: "GET", URL: "https://example.com/users"},
			want: "curl -X GET 'https://example.com/users'",
		},
		{
			name: "POST with body",
			req: model.Request{
				Method: "POST",
				URL:    "https://example.com/users",
				Body:   `{"name":"alice"}`,
			},
			want: "curl -X POST 'https://example.com/users' \\\n  --data-raw '{\"name\":\"alice\"}'",
		},
		{
			name: "request with headers sorted",
			req: model.Request{
				Method: "GET",
				URL:    "https://example.com",
				Headers: map[string]string{
					"Authorization": "Bearer token",
					"Accept":        "application/json",
				},
			},
			want: "curl -X GET 'https://example.com' \\\n  -H 'Accept: application/json' \\\n  -H 'Authorization: Bearer token'",
		},
		{
			name: "variable substitution in URL",
			req:  model.Request{Method: "GET", URL: "{{base_url}}/users"},
			vars: map[string]string{"base_url": "https://api.example.com"},
			want: "curl -X GET 'https://api.example.com/users'",
		},
		{
			name: "variable substitution in header",
			req: model.Request{
				Method:  "GET",
				URL:     "https://example.com",
				Headers: map[string]string{"Authorization": "Bearer {{token}}"},
			},
			vars: map[string]string{"token": "abc123"},
			want: "curl -X GET 'https://example.com' \\\n  -H 'Authorization: Bearer abc123'",
		},
		{
			name: "variable substitution in body",
			req: model.Request{
				Method: "POST",
				URL:    "https://example.com/login",
				Body:   `{"password":"{{password}}"}`,
			},
			vars: map[string]string{"password": "s3cr3t"},
			want: "curl -X POST 'https://example.com/login' \\\n  --data-raw '{\"password\":\"s3cr3t\"}'",
		},
		{
			name: "unresolved variable left intact",
			req:  model.Request{Method: "GET", URL: "{{base_url}}/users"},
			vars: map[string]string{},
			want: "curl -X GET '{{base_url}}/users'",
		},
		{
			name: "single quote in URL is escaped",
			req:  model.Request{Method: "GET", URL: "https://example.com/it's"},
			want: "curl -X GET 'https://example.com/it'\"'\"'s'",
		},
		{
			name: "empty method defaults to GET",
			req:  model.Request{URL: "https://example.com"},
			want: "curl -X GET 'https://example.com'",
		},
		{
			name: "DELETE with headers and no body",
			req: model.Request{
				Method:  "DELETE",
				URL:     "https://example.com/users/1",
				Headers: map[string]string{"X-Api-Key": "key"},
			},
			want: "curl -X DELETE 'https://example.com/users/1' \\\n  -H 'X-Api-Key: key'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ToCurl(tc.req, tc.vars)
			if got != tc.want {
				t.Errorf("\ngot:  %s\nwant: %s", got, tc.want)
			}
		})
	}
}

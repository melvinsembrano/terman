// Package model defines the persisted data structures for terman:
// saved HTTP requests and environments of variables.
package model

// Request is a single saved HTTP request. It is serialized to its own
// YAML file under the requests store directory, nested in a subdirectory
// per Group.
//
// Group is a "/"-separated folder path (e.g. "auth/oauth") mirroring the
// request's location on disk; the directory is authoritative, so Group is
// always re-derived from a request's actual file location when it's
// loaded (see store.LoadRequests) rather than trusted from this field
// alone. "" means the request lives at the top level.
type Request struct {
	Name    string            `yaml:"name"`
	Group   string            `yaml:"group,omitempty"`
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
}

// Environment is a named collection of variables substitutable into a
// Request's URL, headers, and body via {{key}} tokens.
type Environment struct {
	Name string            `yaml:"name"`
	Vars map[string]string `yaml:"vars,omitempty"`
}

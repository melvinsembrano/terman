package vars

import "testing"

func TestApply(t *testing.T) {
	v := map[string]string{"base_url": "https://api.example.com", "id": "42"}

	got := Apply("{{base_url}}/users/{{id}}", v)
	want := "https://api.example.com/users/42"
	if got != want {
		t.Errorf("Apply = %q, want %q", got, want)
	}

	// Unknown tokens are left untouched.
	got = Apply("{{unknown}}", v)
	if got != "{{unknown}}" {
		t.Errorf("Apply with unknown token = %q, want unchanged", got)
	}
}

func TestApplyTokenWhitespace(t *testing.T) {
	v := map[string]string{"key": "value"}
	got := Apply("{{ key }}", v)
	if got != "value" {
		t.Errorf("Apply with whitespace token = %q, want %q", got, "value")
	}
}

func TestApplyDottedAndHyphenatedKeys(t *testing.T) {
	v := map[string]string{"a.b-c": "resolved"}
	got := Apply("{{a.b-c}}", v)
	if got != "resolved" {
		t.Errorf("Apply(dotted/hyphenated key) = %q, want %q", got, "resolved")
	}
}

func TestApplyEmptyVarsReturnsInputUnchanged(t *testing.T) {
	s := "{{anything}}/path"
	if got := Apply(s, nil); got != s {
		t.Errorf("Apply with nil vars = %q, want unchanged %q", got, s)
	}
	if got := Apply(s, map[string]string{}); got != s {
		t.Errorf("Apply with empty vars = %q, want unchanged %q", got, s)
	}
}

func TestApplyReplacesRepeatedTokens(t *testing.T) {
	v := map[string]string{"id": "42"}
	got := Apply("{{id}}-{{id}}-{{id}}", v)
	want := "42-42-42"
	if got != want {
		t.Errorf("Apply(repeated tokens) = %q, want %q", got, want)
	}
}

func TestMergeSkipsNilLayers(t *testing.T) {
	got := Merge(nil, map[string]string{"a": "1"}, nil)
	if len(got) != 1 || got["a"] != "1" {
		t.Errorf("Merge with nil layers = %v, want map[a:1]", got)
	}
}

func TestMergeLaterLayersWin(t *testing.T) {
	got := Merge(
		map[string]string{"a": "1"},
		map[string]string{"a": "2"},
		map[string]string{"a": "3"},
	)
	if got["a"] != "3" {
		t.Errorf("Merge[a] = %q, want %q (last layer should win)", got["a"], "3")
	}
}

func TestMerge(t *testing.T) {
	base := map[string]string{"a": "1", "b": "2"}
	override := map[string]string{"b": "override", "c": "3"}

	got := Merge(base, override)
	want := map[string]string{"a": "1", "b": "override", "c": "3"}
	if len(got) != len(want) {
		t.Fatalf("Merge len = %d, want %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("Merge[%q] = %q, want %q", k, got[k], v)
		}
	}
}

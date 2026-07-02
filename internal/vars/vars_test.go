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

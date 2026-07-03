package version

import "testing"

func TestVersionIsNonEmpty(t *testing.T) {
	if Version == "" {
		t.Error("Version is empty, want a non-empty version string")
	}
}

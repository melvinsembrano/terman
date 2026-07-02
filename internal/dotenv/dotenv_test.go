package dotenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBasicPairs(t *testing.T) {
	got, err := Parse(strings.NewReader("FOO=bar\nBAZ=qux"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := map[string]string{"FOO": "bar", "BAZ": "qux"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("got[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestParseSkipsCommentsAndBlankLines(t *testing.T) {
	input := "# a comment\n\nFOO=bar\n   \n# another\nBAZ=qux\n"
	got, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 2 || got["FOO"] != "bar" || got["BAZ"] != "qux" {
		t.Errorf("got %v", got)
	}
}

func TestParseExportPrefix(t *testing.T) {
	got, err := Parse(strings.NewReader("export FOO=bar\nBAZ=qux"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got["FOO"] != "bar" {
		t.Errorf(`got["FOO"] = %q, want "bar"`, got["FOO"])
	}
}

func TestParseDoubleQuotedValueWithEscapes(t *testing.T) {
	got, err := Parse(strings.NewReader(`FOO="line1\nline2\ttabbed \"quoted\" back\\slash"`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := "line1\nline2\ttabbed \"quoted\" back\\slash"
	if got["FOO"] != want {
		t.Errorf("got[FOO] = %q, want %q", got["FOO"], want)
	}
}

func TestParseSingleQuotedValueIsLiteral(t *testing.T) {
	got, err := Parse(strings.NewReader(`FOO='literal \n not escaped'`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := `literal \n not escaped`
	if got["FOO"] != want {
		t.Errorf("got[FOO] = %q, want %q", got["FOO"], want)
	}
}

func TestParseUnquotedInlineComment(t *testing.T) {
	got, err := Parse(strings.NewReader("FOO=bar  # trailing comment"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got["FOO"] != "bar" {
		t.Errorf(`got[FOO] = %q, want "bar"`, got["FOO"])
	}
}

func TestParseUnquotedValueWithHashButNoSpaceIsLiteral(t *testing.T) {
	got, err := Parse(strings.NewReader("FOO=bar#nothash"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got["FOO"] != "bar#nothash" {
		t.Errorf(`got[FOO] = %q, want "bar#nothash" (no space before '#')`, got["FOO"])
	}
}

func TestParseEmptyValue(t *testing.T) {
	got, err := Parse(strings.NewReader("FOO="))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got["FOO"] != "" {
		t.Errorf(`got[FOO] = %q, want ""`, got["FOO"])
	}
}

func TestParseTrimsWhitespaceAroundKeyAndUnquotedValue(t *testing.T) {
	got, err := Parse(strings.NewReader("  FOO  =  bar  "))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got["FOO"] != "bar" {
		t.Errorf(`got[FOO] = %q, want "bar"`, got["FOO"])
	}
}

func TestParseMissingEqualsIsError(t *testing.T) {
	_, err := Parse(strings.NewReader("NOTAVAR"))
	if err == nil {
		t.Error("expected an error for a line with no '='")
	}
}

func TestParseEmptyKeyIsError(t *testing.T) {
	_, err := Parse(strings.NewReader("=value"))
	if err == nil {
		t.Error("expected an error for an empty key")
	}
}

func TestParseEmptyInput(t *testing.T) {
	got, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty map", got)
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.dev")
	content := "BASE_URL=https://dev.example.com\nTOKEN=abc123\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got["BASE_URL"] != "https://dev.example.com" || got["TOKEN"] != "abc123" {
		t.Errorf("got %v", got)
	}
}

func TestParseFileMissing(t *testing.T) {
	_, err := ParseFile(filepath.Join(t.TempDir(), "does-not-exist.env"))
	if err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestParseFileWrapsParseErrorWithPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("NOTAVAR\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not mention the file path %q", err.Error(), path)
	}
}

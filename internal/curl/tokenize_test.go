package curl

import (
	"reflect"
	"testing"
)

func TestTokenizeBasic(t *testing.T) {
	got, err := tokenize("curl https://example.com -X POST")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"curl", "https://example.com", "-X", "POST"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize() = %#v, want %#v", got, want)
	}
}

func TestTokenizeEmptyInput(t *testing.T) {
	got, err := tokenize("   \t\n  ")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("tokenize(whitespace) = %#v, want empty", got)
	}
}

func TestTokenizeSingleQuotesAreLiteral(t *testing.T) {
	got, err := tokenize(`-H 'Accept: application/json' -d 'a\nb'`)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"-H", "Accept: application/json", "-d", `a\nb`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize() = %#v, want %#v", got, want)
	}
}

func TestTokenizeDoubleQuotesEscapes(t *testing.T) {
	got, err := tokenize(`-d "{\"key\":\"value\"}"`)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"-d", `{"key":"value"}`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize() = %#v, want %#v", got, want)
	}
}

func TestTokenizeDoubleQuotesLeavesOtherBackslashesLiteral(t *testing.T) {
	got, err := tokenize(`-d "line1\nline2"`)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"-d", `line1\nline2`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize() = %#v, want %#v", got, want)
	}
}

func TestTokenizeUnquotedBackslashEscapesNextChar(t *testing.T) {
	got, err := tokenize(`foo\ bar \'baz`)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"foo bar", "'baz"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize() = %#v, want %#v", got, want)
	}
}

func TestTokenizeLineContinuation(t *testing.T) {
	got, err := tokenize("curl 'https://example.com' \\\n  -H 'Accept: application/json'")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"curl", "https://example.com", "-H", "Accept: application/json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize() = %#v, want %#v", got, want)
	}
}

func TestTokenizeLineContinuationGluesWordAcrossLines(t *testing.T) {
	got, err := tokenize("ab\\\ncd")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"abcd"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize() = %#v, want %#v", got, want)
	}
}

func TestTokenizeGluedFlagAndQuotedValueMerge(t *testing.T) {
	got, err := tokenize(`-H'Accept: application/json'`)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"-HAccept: application/json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize() = %#v, want %#v", got, want)
	}
}

func TestTokenizeUnterminatedSingleQuoteIsError(t *testing.T) {
	if _, err := tokenize(`-d 'unterminated`); err == nil {
		t.Error("expected an error for an unterminated single quote")
	}
}

func TestTokenizeUnterminatedDoubleQuoteIsError(t *testing.T) {
	if _, err := tokenize(`-d "unterminated`); err == nil {
		t.Error("expected an error for an unterminated double quote")
	}
}

func TestTokenizeTrailingBackslashIsDropped(t *testing.T) {
	got, err := tokenize(`foo\`)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"foo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize() = %#v, want %#v", got, want)
	}
}

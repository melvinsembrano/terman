// Package jsonview parses JSON into a foldable tree, preserving source
// object key order and exact number literals — used to power the
// response screen's fx-style interactive JSON viewer.
package jsonview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Kind identifies the kind of JSON value a Node represents.
type Kind int

const (
	KindString Kind = iota
	KindNumber
	KindBool
	KindNull
	KindObject
	KindArray
)

// Node is one value in a parsed JSON document. Unlike unmarshaling into
// map[string]interface{}, object member order is preserved (Children is
// appended in source order).
type Node struct {
	Kind   Kind
	Key    string // raw (unescaped) member name; empty for the root or array elements
	Scalar string // raw string value for KindString/KindBool; exact original literal for KindNumber

	Children []*Node // object members (KindObject) or elements (KindArray), in source order

	// Collapsed only applies to KindObject/KindArray. Toggling it and
	// re-calling Flatten is how folding works.
	Collapsed bool
}

// Parse parses raw JSON bytes into a Node tree.
func Parse(raw []byte) (*Node, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	root, err := parseValue(dec)
	if err != nil {
		return nil, fmt.Errorf("jsonview: %w", err)
	}

	// Reject trailing garbage after the first value (e.g. "1 2", concatenated
	// JSON, or outright invalid text) so callers can reliably fall back to
	// plain text. dec.Token() returns io.EOF when nothing is left; anything
	// else — a successfully-read extra token, or a syntax error while
	// trying to tokenize leftover garbage — means there was trailing data.
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("jsonview: unexpected trailing data after JSON value")
		}
		return nil, fmt.Errorf("jsonview: trailing data after JSON value: %w", err)
	}

	return root, nil
}

func parseValue(dec *json.Decoder) (*Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return parseObject(dec)
		case '[':
			return parseArray(dec)
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", t)
		}
	case string:
		return &Node{Kind: KindString, Scalar: t}, nil
	case json.Number:
		return &Node{Kind: KindNumber, Scalar: t.String()}, nil
	case bool:
		scalar := "false"
		if t {
			scalar = "true"
		}
		return &Node{Kind: KindBool, Scalar: scalar}, nil
	case nil:
		return &Node{Kind: KindNull}, nil
	default:
		return nil, fmt.Errorf("unexpected token %v (%T)", tok, tok)
	}
}

func parseObject(dec *json.Decoder) (*Node, error) {
	n := &Node{Kind: KindObject}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("expected object key, got %v", keyTok)
		}
		child, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		child.Key = key
		n.Children = append(n.Children, child)
	}
	if _, err := dec.Token(); err != nil { // consume closing '}'
		return nil, err
	}
	return n, nil
}

func parseArray(dec *json.Decoder) (*Node, error) {
	n := &Node{Kind: KindArray}
	for dec.More() {
		child, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		n.Children = append(n.Children, child)
	}
	if _, err := dec.Token(); err != nil { // consume closing ']'
		return nil, err
	}
	return n, nil
}

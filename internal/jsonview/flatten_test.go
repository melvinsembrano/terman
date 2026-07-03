package jsonview

import "testing"

func mustParse(t *testing.T, raw string) *Node {
	t.Helper()
	node, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse(%q): %v", raw, err)
	}
	return node
}

func TestFlattenFlatObject(t *testing.T) {
	root := mustParse(t, `{"a":1,"b":2,"c":3}`)
	lines := Flatten(root)

	// A flat object with no nested containers: an opening line, one
	// LineValue per member, and a closing line.
	if len(lines) != 5 {
		t.Fatalf("len(lines) = %d, want 5", len(lines))
	}
	if lines[0].Kind != LineOpen || lines[0].Depth != 0 {
		t.Errorf("lines[0] = %+v, want LineOpen at depth 0", lines[0])
	}
	for i, want := range []string{"a", "b", "c"} {
		l := lines[i+1]
		if l.Kind != LineValue || l.Depth != 1 || l.Node.Key != want {
			t.Errorf("lines[%d] = %+v, want LineValue depth 1 key %q", i+1, l, want)
		}
	}
	if lines[1].IsLast || lines[2].IsLast || !lines[3].IsLast {
		t.Errorf("IsLast flags wrong: %v %v %v (want false false true)", lines[1].IsLast, lines[2].IsLast, lines[3].IsLast)
	}
	if lines[4].Kind != LineClose || lines[4].Depth != 0 {
		t.Errorf("lines[4] = %+v, want LineClose at depth 0", lines[4])
	}
}

func TestFlattenNestedDocument(t *testing.T) {
	root := mustParse(t, `{"tags":["a","b"],"address":{"city":"X"}}`)
	lines := Flatten(root)

	// root{ -> tags[ -> "a" -> "b" -> ] -> address{ -> city -> } -> }
	wantKinds := []LineKind{LineOpen, LineOpen, LineValue, LineValue, LineClose, LineOpen, LineValue, LineClose, LineClose}
	if len(lines) != len(wantKinds) {
		t.Fatalf("len(lines) = %d, want %d", len(lines), len(wantKinds))
	}
	for i, want := range wantKinds {
		if lines[i].Kind != want {
			t.Errorf("lines[%d].Kind = %v, want %v", i, lines[i].Kind, want)
		}
	}

	// Depths: root open=0, tags open=1, "a"/"b"=2, tags close=1,
	// address open=1, city=2, address close=1, root close=0.
	wantDepths := []int{0, 1, 2, 2, 1, 1, 2, 1, 0}
	for i, want := range wantDepths {
		if lines[i].Depth != want {
			t.Errorf("lines[%d].Depth = %d, want %d", i, lines[i].Depth, want)
		}
	}

	// tags is not the last member of root -> its close line is not IsLast;
	// address is the last member -> its close line is IsLast.
	if lines[4].IsLast {
		t.Errorf("tags close IsLast = true, want false (not the last root member)")
	}
	if !lines[8].IsLast {
		t.Errorf("root close IsLast = false, want true (root itself has no siblings)")
	}
}

func TestFlattenCollapsedNodeHidesChildren(t *testing.T) {
	root := mustParse(t, `{"a":1,"nested":{"x":1,"y":2},"b":2}`)

	full := Flatten(root)
	fullLen := len(full)

	for _, c := range root.Children {
		if c.Key == "nested" {
			c.Collapsed = true
		}
	}
	collapsed := Flatten(root)

	// Collapsing "nested" removes its LineOpen/LineClose and its two
	// children's lines, replacing all four with a single LineValue.
	if len(collapsed) != fullLen-3 {
		t.Fatalf("len(collapsed) = %d, want %d (fullLen=%d)", len(collapsed), fullLen-3, fullLen)
	}

	found := false
	for _, l := range collapsed {
		if l.Node.Key == "nested" {
			found = true
			if l.Kind != LineValue {
				t.Errorf("collapsed nested line.Kind = %v, want LineValue", l.Kind)
			}
		}
	}
	if !found {
		t.Fatal("collapsed 'nested' line not found")
	}

	// Siblings a/b are untouched.
	for _, key := range []string{"a", "b"} {
		hit := false
		for _, l := range collapsed {
			if l.Node.Key == key {
				hit = true
			}
		}
		if !hit {
			t.Errorf("sibling %q missing after collapsing an unrelated node", key)
		}
	}
}

func TestFlattenCollapsedRoot(t *testing.T) {
	root := mustParse(t, `{"a":1,"b":2}`)
	root.Collapsed = true

	lines := Flatten(root)
	if len(lines) != 1 {
		t.Fatalf("len(lines) = %d, want 1 (collapsed root is a single line)", len(lines))
	}
	if lines[0].Kind != LineValue {
		t.Errorf("lines[0].Kind = %v, want LineValue", lines[0].Kind)
	}
}

func TestFlattenEmptyContainerIsSingleLineValue(t *testing.T) {
	for _, raw := range []string{`{}`, `[]`} {
		root := mustParse(t, raw)
		lines := Flatten(root)
		if len(lines) != 1 || lines[0].Kind != LineValue {
			t.Errorf("Flatten(%q) = %+v, want a single LineValue", raw, lines)
		}
	}
}

func TestFlattenScalarRootIsSingleLineValue(t *testing.T) {
	root := mustParse(t, `"hello"`)
	lines := Flatten(root)
	if len(lines) != 1 || lines[0].Kind != LineValue || lines[0].Node.Scalar != "hello" {
		t.Errorf("Flatten(scalar) = %+v", lines)
	}
}

func TestFlattenArrayElementsHaveNoKey(t *testing.T) {
	root := mustParse(t, `[1,2]`)
	lines := Flatten(root)
	for _, l := range lines {
		if l.Kind == LineValue && l.Node.Key != "" {
			t.Errorf("array element has a Key: %+v", l)
		}
	}
}

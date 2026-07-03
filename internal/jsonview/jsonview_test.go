package jsonview

import "testing"

func TestParseScalars(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		kind Kind
	}{
		{"string", `"hello"`, KindString},
		{"number", `42`, KindNumber},
		{"bool true", `true`, KindBool},
		{"bool false", `false`, KindBool},
		{"null", `null`, KindNull},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node, err := Parse([]byte(c.raw))
			if err != nil {
				t.Fatalf("Parse(%q): %v", c.raw, err)
			}
			if node.Kind != c.kind {
				t.Errorf("Kind = %v, want %v", node.Kind, c.kind)
			}
		})
	}
}

func TestParseStringScalarValue(t *testing.T) {
	node, err := Parse([]byte(`"hello world"`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Scalar != "hello world" {
		t.Errorf("Scalar = %q, want %q", node.Scalar, "hello world")
	}
}

func TestParseStringEscapeRoundTrip(t *testing.T) {
	// The raw JSON escapes a quote and a newline; Scalar should hold the
	// unescaped Go string (rendering/re-quoting is the tui package's job).
	node, err := Parse([]byte(`"line1\nline2 \"quoted\""`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := "line1\nline2 \"quoted\""
	if node.Scalar != want {
		t.Errorf("Scalar = %q, want %q", node.Scalar, want)
	}
}

func TestParseNumberPreservesExactLiteral(t *testing.T) {
	cases := []string{"1.50", "42", "1e10", "-0.001", "9007199254740993"}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			node, err := Parse([]byte(raw))
			if err != nil {
				t.Fatalf("Parse(%q): %v", raw, err)
			}
			if node.Kind != KindNumber {
				t.Fatalf("Kind = %v, want KindNumber", node.Kind)
			}
			if node.Scalar != raw {
				t.Errorf("Scalar = %q, want exact literal %q", node.Scalar, raw)
			}
		})
	}
}

func TestParseObjectPreservesKeyOrder(t *testing.T) {
	node, err := Parse([]byte(`{"zebra":1,"apple":2,"mango":3}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Kind != KindObject {
		t.Fatalf("Kind = %v, want KindObject", node.Kind)
	}
	want := []string{"zebra", "apple", "mango"}
	if len(node.Children) != len(want) {
		t.Fatalf("Children = %d, want %d", len(node.Children), len(want))
	}
	for i, k := range want {
		if node.Children[i].Key != k {
			t.Errorf("Children[%d].Key = %q, want %q", i, node.Children[i].Key, k)
		}
	}
}

func TestParseArrayPreservesElementOrder(t *testing.T) {
	node, err := Parse([]byte(`[3,1,2]`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Kind != KindArray {
		t.Fatalf("Kind = %v, want KindArray", node.Kind)
	}
	want := []string{"3", "1", "2"}
	for i, w := range want {
		if node.Children[i].Scalar != w {
			t.Errorf("Children[%d].Scalar = %q, want %q", i, node.Children[i].Scalar, w)
		}
	}
}

func TestParseNestedDocument(t *testing.T) {
	raw := `{"name":"Ada","age":36,"active":true,"nickname":null,"score":1.50,` +
		`"tags":["admin","beta"],"address":{"city":"London","zip":"E1"}}`

	root, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if root.Kind != KindObject || len(root.Children) != 7 {
		t.Fatalf("root = %+v, want an object with 7 members", root)
	}

	byKey := map[string]*Node{}
	for _, c := range root.Children {
		byKey[c.Key] = c
	}

	if byKey["name"].Kind != KindString || byKey["name"].Scalar != "Ada" {
		t.Errorf("name = %+v", byKey["name"])
	}
	if byKey["age"].Kind != KindNumber || byKey["age"].Scalar != "36" {
		t.Errorf("age = %+v", byKey["age"])
	}
	if byKey["active"].Kind != KindBool || byKey["active"].Scalar != "true" {
		t.Errorf("active = %+v", byKey["active"])
	}
	if byKey["nickname"].Kind != KindNull {
		t.Errorf("nickname = %+v, want KindNull", byKey["nickname"])
	}
	if byKey["score"].Scalar != "1.50" {
		t.Errorf("score.Scalar = %q, want exact literal %q", byKey["score"].Scalar, "1.50")
	}

	tags := byKey["tags"]
	if tags.Kind != KindArray || len(tags.Children) != 2 {
		t.Fatalf("tags = %+v", tags)
	}
	if tags.Children[0].Scalar != "admin" || tags.Children[1].Scalar != "beta" {
		t.Errorf("tags children = %+v", tags.Children)
	}

	addr := byKey["address"]
	if addr.Kind != KindObject || len(addr.Children) != 2 {
		t.Fatalf("address = %+v", addr)
	}
	if addr.Children[0].Key != "city" || addr.Children[0].Scalar != "London" {
		t.Errorf("address.city = %+v", addr.Children[0])
	}
}

func TestParseEmptyObjectAndArray(t *testing.T) {
	obj, err := Parse([]byte(`{}`))
	if err != nil {
		t.Fatalf("Parse({}): %v", err)
	}
	if obj.Kind != KindObject || len(obj.Children) != 0 {
		t.Errorf("obj = %+v, want empty object", obj)
	}

	arr, err := Parse([]byte(`[]`))
	if err != nil {
		t.Fatalf("Parse([]): %v", err)
	}
	if arr.Kind != KindArray || len(arr.Children) != 0 {
		t.Errorf("arr = %+v, want empty array", arr)
	}
}

func TestParseMalformedJSONIsError(t *testing.T) {
	cases := []string{
		`{"a":}`,
		`{"a":1,}`,
		`[1,2,`,
		`not json at all`,
		``,
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if _, err := Parse([]byte(raw)); err == nil {
				t.Errorf("Parse(%q): expected an error", raw)
			}
		})
	}
}

func TestParseTrailingGarbageIsError(t *testing.T) {
	if _, err := Parse([]byte(`{"a":1} garbage`)); err == nil {
		t.Error("expected an error for trailing data after the JSON value")
	}
	if _, err := Parse([]byte(`1 2`)); err == nil {
		t.Error("expected an error for concatenated JSON values")
	}
}

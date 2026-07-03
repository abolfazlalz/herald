package message

import "testing"

func TestCanonicalizeSortsKeysRecursively(t *testing.T) {
	got, err := Canonicalize(map[string]any{
		"z": "last",
		"a": map[string]any{
			"b": float64(2),
			"a": "first",
		},
		"items": []any{
			map[string]any{"y": true, "x": nil},
			"done",
		},
	})
	if err != nil {
		t.Fatalf("Canonicalize returned error: %v", err)
	}

	want := `{"a":{"a":"first","b":2},"items":[{"x":null,"y":true},"done"],"z":"last"}`
	if string(got) != want {
		t.Fatalf("Canonicalize() = %s, want %s", got, want)
	}
}

func TestCanonicalizeRejectsNilInput(t *testing.T) {
	if _, err := Canonicalize(nil); err == nil {
		t.Fatal("Canonicalize(nil) returned nil error")
	}
}

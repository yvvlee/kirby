package set

import "testing"

func TestSetTracksUniqueValues(t *testing.T) {
	values := New("a", "a")
	values.Add("b")
	if !values.Contains("a") || !values.Contains("b") {
		t.Fatal("unexpected set: missing members after Add")
	}
	values.Remove("a")
	if values.Contains("a") {
		t.Fatal("removed value remains")
	}
}

package set

import "testing"

func TestSetTracksUniqueValues(t *testing.T) {
	values := New("a", "a")
	values.Add("b")
	if values.Size() != 2 || !values.Contains("a") || !values.Contains("b") {
		t.Fatalf("unexpected set: %#v", values.Values())
	}
	values.Remove("a")
	if values.Contains("a") {
		t.Fatal("removed value remains")
	}
}

package timeutil

import (
	"testing"
	"time"
)

func TestRFC3339RoundTrip(t *testing.T) {
	want := time.Date(2026, 8, 17, 10, 11, 12, 123, time.FixedZone("offset", 8*60*60))
	encoded := FormatRFC3339(want)
	parsed, err := ParseRFC3339(encoded)
	if err != nil || !parsed.Equal(want) {
		t.Fatalf("parsed=%v err=%v", parsed, err)
	}
}

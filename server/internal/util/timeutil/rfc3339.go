package timeutil

import "time"

// FormatRFC3339 keeps absent timestamps distinct from real dates.
//
// Parameters: value is the timestamp to serialize.
// Returns: the RFC3339 representation, or an empty string for the zero time.
// Throws: none.
func FormatRFC3339(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

// ParseRFC3339 accepts the timestamp representation used by API messages.
//
// Parameters: value is the encoded timestamp.
// Returns: the parsed time, or zero time for an empty value.
// Throws: an error for a malformed nonempty timestamp.
func ParseRFC3339(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

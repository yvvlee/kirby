package safeint

import (
	"math"
	"strconv"
	"testing"
)

func TestCheckedConversions(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{"negative int64 to uint32", func() error { _, err := Uint32FromInt64(-1); return err }},
		{"large int64 to uint32", func() error { _, err := Uint32FromInt64(math.MaxUint32 + 1); return err }},
		{"negative int to uint32", func() error { _, err := Uint32FromInt(-1); return err }},
		{"negative int64 to uint64", func() error { _, err := Uint64FromInt64(-1); return err }},
		{"negative int to uint64", func() error { _, err := Uint64FromInt(-1); return err }},
		{"large uint64 to int64", func() error { _, err := Int64FromUint64(math.MaxInt64 + 1); return err }},
	}
	if strconv.IntSize == 32 {
		tests = append(tests,
			struct {
				name string
				run  func() error
			}{"large uint64 to int", func() error { _, err := IntFromUint64(math.MaxInt32 + 1); return err }},
			struct {
				name string
				run  func() error
			}{"large uint32 to int", func() error { _, err := IntFromUint32(math.MaxInt32 + 1); return err }},
		)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); err == nil {
				t.Fatal("expected conversion error")
			}
		})
	}

	if value, err := Uint32FromInt64(math.MaxUint32); err != nil || value != math.MaxUint32 {
		t.Fatalf("uint32 boundary conversion = %d, %v", value, err)
	}
	if value, err := Uint64FromInt64(math.MaxInt64); err != nil || value != math.MaxInt64 {
		t.Fatalf("uint64 conversion = %d, %v", value, err)
	}
	if value, err := Int64FromUint64(math.MaxInt64); err != nil || value != math.MaxInt64 {
		t.Fatalf("int64 boundary conversion = %d, %v", value, err)
	}
}

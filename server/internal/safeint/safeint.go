// Package safeint provides checked integer conversions at API and storage boundaries.
package safeint

import (
	"fmt"
	"math"
	"strconv"
)

func Uint32FromInt64(value int64) (uint32, error) {
	if value < 0 || value > math.MaxUint32 {
		return 0, fmt.Errorf("%d does not fit in uint32", value)
	}
	return uint32(value), nil // #nosec G115 -- the bounds are checked above.
}

func Uint32FromInt(value int) (uint32, error) {
	if value < 0 || (strconv.IntSize == 64 && uint64(value) > math.MaxUint32) {
		return 0, fmt.Errorf("%d does not fit in uint32", value)
	}
	return uint32(value), nil // #nosec G115 -- the bounds are checked above.
}

func Uint64FromInt64(value int64) (uint64, error) {
	if value < 0 {
		return 0, fmt.Errorf("%d does not fit in uint64", value)
	}
	return uint64(value), nil // #nosec G115 -- non-negative int64 values fit in uint64.
}

func Uint64FromInt(value int) (uint64, error) {
	if value < 0 {
		return 0, fmt.Errorf("%d does not fit in uint64", value)
	}
	return uint64(value), nil // #nosec G115 -- non-negative int values fit in uint64.
}

func Int64FromUint64(value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("%d does not fit in int64", value)
	}
	return int64(value), nil // #nosec G115 -- the bounds are checked above.
}

func IntFromUint64(value uint64) (int, error) {
	if (strconv.IntSize == 32 && value > math.MaxInt32) || (strconv.IntSize == 64 && value > math.MaxInt64) {
		return 0, fmt.Errorf("%d does not fit in int", value)
	}
	return int(value), nil // #nosec G115 -- the platform-specific bounds are checked above.
}

func IntFromUint32(value uint32) (int, error) {
	if strconv.IntSize == 32 && value > math.MaxInt32 {
		return 0, fmt.Errorf("%d does not fit in int", value)
	}
	return int(value), nil // #nosec G115 -- the platform-specific bounds are checked above.
}

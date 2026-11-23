package converter

import (
	"fmt"
	"math"
	"strconv"
)

func validMeta(recordID, recordVersion int64) (uint32, error) {
	if recordID <= 0 || recordVersion < 0 || recordVersion > math.MaxUint32 {
		return 0, fmt.Errorf("invalid database record")
	}
	return uint32(recordVersion), nil
}

func formatActor(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

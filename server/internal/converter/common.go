package converter

import (
	"fmt"
	"strconv"

	"github.com/yvvlee/kirby/server/internal/util/safeint"
)

func validMeta(recordID, recordVersion int64) (uint32, error) {
	if recordID <= 0 {
		return 0, fmt.Errorf("invalid database record")
	}
	version, err := safeint.Uint32FromInt64(recordVersion)
	if err != nil {
		return 0, fmt.Errorf("invalid database record")
	}
	return version, nil
}

func formatActor(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

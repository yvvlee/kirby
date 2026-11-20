package converter

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// timestamp 转换数据库时间戳；零值保持字段缺席而非 epoch 零点。
func timestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

package converter

import (
	"fmt"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/util/safeint"
	"github.com/yvvlee/kirby/server/internal/util/timeutil"
)

// ConfigToProto converts persisted data to an API message without hiding conversion errors.
//
// Parameters: item contains the persisted record.
// Returns: *commonv1.Config with persisted IDs and metadata; malformed JSON or out-of-range metadata returns an error.
// Throws: none.
func ConfigToProto(item *model.Config) (*commonv1.Config, error) {
	version, err := validMeta(item.ID, item.Version)
	if err != nil {
		return nil, err
	}
	fieldType, err := DecodeFieldType(item.TypeJSON)
	if err != nil {
		return nil, err
	}
	runtimeVersion, err := safeint.Uint64FromInt64(item.RuntimeVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid config runtime version")
	}
	return &commonv1.Config{
		Id:             item.ID,
		ProjectId:      item.ProjectID,
		Key:            item.Key,
		Description:    item.Description,
		IsArray:        item.IsArray,
		Type:           fieldType,
		Value:          item.Value,
		RuntimeVersion: runtimeVersion,
		CreatedBy:      formatActor(item.CreatedBy),
		UpdatedBy:      formatActor(item.UpdatedBy),
		CreatedAt:      timeutil.FormatRFC3339(item.CreatedAt),
		UpdatedAt:      timeutil.FormatRFC3339(item.UpdatedAt),
		Version:        version,
	}, nil
}

package converter

import (
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/util/timeutil"
)

// EnumToProto converts persisted data to an API message without hiding conversion errors.
//
// Parameters: item contains the persisted record.
// Returns: *commonv1.ConfigEnum with persisted IDs and metadata; malformed JSON or out-of-range metadata returns an error.
// Throws: none.
func EnumToProto(item *model.ConfigEnum) (*commonv1.ConfigEnum, error) {
	version, err := validMeta(item.ID, item.Version)
	if err != nil {
		return nil, err
	}
	values, err := DecodeOptions(item.ValuesJSON)
	if err != nil {
		return nil, err
	}
	return &commonv1.ConfigEnum{
		Id:          item.ID,
		ConfigId:    item.ConfigID,
		Key:         item.Key,
		Name:        item.Name,
		Description: item.Description,
		Values:      values,
		CreatedBy:   formatActor(item.CreatedBy),
		UpdatedBy:   formatActor(item.UpdatedBy),
		CreatedAt:   timeutil.FormatRFC3339(item.CreatedAt),
		UpdatedAt:   timeutil.FormatRFC3339(item.UpdatedAt),
		Version:     version,
	}, nil
}

// EnumsToProto converts stored rows without hiding malformed JSON or versions.
//
// Parameters: items contains persisted enum rows.
// Returns: []*commonv1.ConfigEnum with persisted IDs and metadata; malformed JSON or out-of-range metadata returns an error.
// Throws: none.
func EnumsToProto(items []model.ConfigEnum) ([]*commonv1.ConfigEnum, error) {
	result := make([]*commonv1.ConfigEnum, 0, len(items))
	for index := range items {
		item, err := EnumToProto(&items[index])
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

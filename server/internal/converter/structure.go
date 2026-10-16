package converter

import (
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/util/timeutil"
)

// StructureToProto converts persisted data to an API message without hiding conversion errors.
//
// Parameters: item contains the persisted record.
// Returns: *commonv1.Structure with persisted IDs and metadata; malformed JSON or out-of-range metadata returns an error.
// Throws: none.
func StructureToProto(item *model.Structure) (*commonv1.Structure, error) {
	version, err := validMeta(item.ID, item.Version)
	if err != nil {
		return nil, err
	}
	fields, err := DecodeFields(item.FieldsJSON)
	if err != nil {
		return nil, err
	}
	return &commonv1.Structure{
		Id:          item.ID,
		ConfigId:    item.ConfigID,
		Key:         item.Key,
		Name:        item.Name,
		Description: item.Description,
		Fields:      fields,
		CreatedBy:   formatActor(item.CreatedBy),
		UpdatedBy:   formatActor(item.UpdatedBy),
		CreatedAt:   timeutil.FormatRFC3339(item.CreatedAt),
		UpdatedAt:   timeutil.FormatRFC3339(item.UpdatedAt),
		Version:     version,
	}, nil
}

// StructuresToProto converts stored rows without hiding malformed JSON or versions.
//
// Parameters: items contains persisted structure rows.
// Returns: []*commonv1.Structure with persisted IDs and metadata; malformed JSON or out-of-range metadata returns an error.
// Throws: none.
func StructuresToProto(items []model.Structure) ([]*commonv1.Structure, error) {
	result := make([]*commonv1.Structure, 0, len(items))
	for index := range items {
		item, err := StructureToProto(&items[index])
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

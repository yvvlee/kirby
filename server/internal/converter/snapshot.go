package converter

import (
	"fmt"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/util/timeutil"
)

// SnapshotToProto converts persisted data to an API message without hiding conversion errors.
//
// Parameters: item contains the persisted record.
// Returns: *commonv1.Snapshot with persisted IDs and metadata; malformed JSON or out-of-range metadata returns an error.
// Throws: none.
func SnapshotToProto(item *model.Snapshot) (*commonv1.Snapshot, error) {
	version, err := validMeta(item.ID, item.Version)
	if err != nil {
		return nil, err
	}
	tags, err := DecodeTags(item.TagsJSON)
	if err != nil {
		return nil, err
	}
	if item.Status != model.SnapshotStatusUnreleased && item.Status != model.SnapshotStatusReleased {
		return nil, fmt.Errorf("invalid snapshot status")
	}
	return &commonv1.Snapshot{
		Id:          item.ID,
		ProjectId:   item.ProjectID,
		ConfigId:    item.ConfigID,
		ConfigKey:   item.ConfigKey,
		Description: item.Description,
		Content:     item.Content,
		Status:      commonv1.Snapshot_Status(item.Status),
		Tags:        tags,
		IsUsing:     item.IsUsing,
		CreatedBy:   formatActor(item.CreatedBy),
		UpdatedBy:   formatActor(item.UpdatedBy),
		CreatedAt:   timeutil.FormatRFC3339(item.CreatedAt),
		UpdatedAt:   timeutil.FormatRFC3339(item.UpdatedAt),
		Version:     version,
	}, nil
}

// SimpleSnapshotToProto converts persisted data to an API message without hiding conversion errors.
//
// Parameters: item contains the persisted record.
// Returns: *commonv1.SimpleSnapshot with persisted IDs and metadata; malformed JSON or out-of-range metadata returns an error.
// Throws: none.
func SimpleSnapshotToProto(item *model.Snapshot) (*commonv1.SimpleSnapshot, error) {
	full, err := SnapshotToProto(item)
	if err != nil {
		return nil, err
	}
	return &commonv1.SimpleSnapshot{
		Id:          full.Id,
		ProjectId:   full.ProjectId,
		ConfigId:    full.ConfigId,
		ConfigKey:   full.ConfigKey,
		Description: full.Description,
		Status:      full.Status,
		Tags:        full.Tags,
		IsUsing:     full.IsUsing,
		CreatedBy:   full.CreatedBy,
		UpdatedBy:   full.UpdatedBy,
		CreatedAt:   full.CreatedAt,
		UpdatedAt:   full.UpdatedAt,
		Version:     full.Version,
	}, nil
}

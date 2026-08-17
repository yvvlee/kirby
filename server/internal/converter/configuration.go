package converter

import (
	"fmt"
	"strconv"

	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/timeutil"
)

func ProjectToProto(item *model.Project) (*commonv1.Project, error) {
	if item == nil {
		return nil, fmt.Errorf("invalid database record")
	}
	if err := validMeta(item.ID, item.Version); err != nil {
		return nil, err
	}
	return &commonv1.Project{
		Id: item.ID, EnvironmentId: item.EnvironmentID, Key: item.Key, Name: item.Name, Description: item.Description,
		CreatedBy: formatActor(item.CreatedBy), UpdatedBy: formatActor(item.UpdatedBy),
		CreatedAt: timeutil.FormatRFC3339(item.CreatedAt), UpdatedAt: timeutil.FormatRFC3339(item.UpdatedAt), Version: uint32(item.Version),
	}, nil
}

func ConfigToProto(item *model.Config) (*commonv1.Config, error) {
	if item == nil {
		return nil, fmt.Errorf("invalid database record")
	}
	if err := validMeta(item.ID, item.Version); err != nil {
		return nil, err
	}
	fieldType, err := DecodeFieldType(item.TypeJSON)
	if err != nil {
		return nil, err
	}
	if item.RuntimeVersion < 0 {
		return nil, fmt.Errorf("invalid config runtime version")
	}
	return &commonv1.Config{
		Id: item.ID, ProjectId: item.ProjectID, Key: item.Key, Description: item.Description,
		IsArray: item.IsArray, Type: fieldType, Value: item.Value, RuntimeVersion: uint64(item.RuntimeVersion),
		CreatedBy: formatActor(item.CreatedBy), UpdatedBy: formatActor(item.UpdatedBy),
		CreatedAt: timeutil.FormatRFC3339(item.CreatedAt), UpdatedAt: timeutil.FormatRFC3339(item.UpdatedAt), Version: uint32(item.Version),
	}, nil
}

func StructureToProto(item *model.Structure) (*commonv1.Structure, error) {
	if item == nil {
		return nil, fmt.Errorf("invalid database record")
	}
	if err := validMeta(item.ID, item.Version); err != nil {
		return nil, err
	}
	fields, err := DecodeFields(item.FieldsJSON)
	if err != nil {
		return nil, err
	}
	return &commonv1.Structure{
		Id: item.ID, ConfigId: item.ConfigID, Key: item.Key, Name: item.Name, Description: item.Description, Fields: fields,
		CreatedBy: formatActor(item.CreatedBy), UpdatedBy: formatActor(item.UpdatedBy),
		CreatedAt: timeutil.FormatRFC3339(item.CreatedAt), UpdatedAt: timeutil.FormatRFC3339(item.UpdatedAt), Version: uint32(item.Version),
	}, nil
}

func EnumToProto(item *model.ConfigEnum) (*commonv1.ConfigEnum, error) {
	if item == nil {
		return nil, fmt.Errorf("invalid database record")
	}
	if err := validMeta(item.ID, item.Version); err != nil {
		return nil, err
	}
	values, err := DecodeOptions(item.ValuesJSON)
	if err != nil {
		return nil, err
	}
	return &commonv1.ConfigEnum{
		Id: item.ID, ConfigId: item.ConfigID, Key: item.Key, Name: item.Name, Description: item.Description, Values: values,
		CreatedBy: formatActor(item.CreatedBy), UpdatedBy: formatActor(item.UpdatedBy),
		CreatedAt: timeutil.FormatRFC3339(item.CreatedAt), UpdatedAt: timeutil.FormatRFC3339(item.UpdatedAt), Version: uint32(item.Version),
	}, nil
}

func SnapshotToProto(item *model.Snapshot) (*commonv1.Snapshot, error) {
	if item == nil {
		return nil, fmt.Errorf("invalid database record")
	}
	if err := validMeta(item.ID, item.Version); err != nil {
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
		Id: item.ID, ProjectId: item.ProjectID, ConfigId: item.ConfigID, ConfigKey: item.ConfigKey,
		Description: item.Description, Content: item.Content, Status: commonv1.Snapshot_Status(item.Status), Tags: tags, IsUsing: item.IsUsing,
		CreatedBy: formatActor(item.CreatedBy), UpdatedBy: formatActor(item.UpdatedBy),
		CreatedAt: timeutil.FormatRFC3339(item.CreatedAt), UpdatedAt: timeutil.FormatRFC3339(item.UpdatedAt), Version: uint32(item.Version),
	}, nil
}

func SimpleSnapshotToProto(item *model.Snapshot) (*commonv1.SimpleSnapshot, error) {
	full, err := SnapshotToProto(item)
	if err != nil {
		return nil, err
	}
	return &commonv1.SimpleSnapshot{
		Id: full.Id, ProjectId: full.ProjectId, ConfigId: full.ConfigId, ConfigKey: full.ConfigKey,
		Description: full.Description, Status: full.Status, Tags: full.Tags, IsUsing: full.IsUsing,
		CreatedBy: full.CreatedBy, UpdatedBy: full.UpdatedBy, CreatedAt: full.CreatedAt, UpdatedAt: full.UpdatedAt, Version: full.Version,
	}, nil
}

func validMeta(recordID, recordVersion int64) error {
	if recordID <= 0 || recordVersion < 0 || uint64(recordVersion) > uint64(^uint32(0)) {
		return fmt.Errorf("invalid database record")
	}
	return nil
}

func formatActor(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

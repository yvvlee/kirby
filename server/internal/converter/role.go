package converter

import (
	"fmt"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/util/safeint"
	"github.com/yvvlee/kirby/server/internal/util/timeutil"
)

// RoleToProto uses the same role projection in catalogs and environment memberships.
//
// Parameters: item contains a persisted role and its permissions.
// Returns: the public role with permission entries, or an invalid-metadata error.
// Throws: none; failures are returned as errors.
func RoleToProto(item *repository.RoleWithPermissions) (*commonv1.Role, error) {
	if item.Role.ID <= 0 {
		return nil, fmt.Errorf("invalid role record")
	}
	version, err := safeint.Uint32FromInt64(item.Role.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid role record")
	}
	permissions := make([]*commonv1.Permission, 0, len(item.Permissions))
	for index := range item.Permissions {
		converted, err := PermissionToProto(&item.Permissions[index])
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, converted)
	}
	return &commonv1.Role{
		Id:          item.Role.ID,
		Key:         item.Role.Key,
		Name:        item.Role.Name,
		Description: item.Role.Description,
		Builtin:     item.Role.Builtin,
		Permissions: permissions,
		CreatedAt:   timeutil.FormatRFC3339(item.Role.CreatedAt),
		UpdatedAt:   timeutil.FormatRFC3339(item.Role.UpdatedAt),
		Version:     version,
	}, nil
}

// PermissionToProto exposes the stable permission catalog fields.
//
// Parameters: item is the persisted permission entry.
// Returns: the public permission, or an error when its ID is invalid.
// Throws: none; failures are returned as errors.
func PermissionToProto(item *model.Permission) (*commonv1.Permission, error) {
	if item.ID <= 0 {
		return nil, fmt.Errorf("invalid permission record")
	}
	return &commonv1.Permission{
		Id:          item.ID,
		Key:         item.Key,
		Name:        item.Name,
		Description: item.Description,
	}, nil
}

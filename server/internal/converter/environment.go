package converter

import (
	"fmt"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/util/safeint"
	"github.com/yvvlee/kirby/server/internal/util/timeutil"
)

// EnvironmentToProto uses one field mapping for administration and authenticated profile responses.
//
// Parameters: item is the persisted environment, including its project owner.
// Returns: the public environment with project ID, or an error when its ID or version is invalid.
// Throws: none; failures are returned as errors.
func EnvironmentToProto(item *model.Environment) (*commonv1.Environment, error) {
	if item.ID <= 0 {
		return nil, fmt.Errorf("invalid environment record")
	}
	version, err := safeint.Uint32FromInt64(item.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid environment record")
	}
	return &commonv1.Environment{
		Id:          item.ID,
		ProjectId:   item.ProjectID,
		Key:         item.Key,
		Name:        item.Name,
		Description: item.Description,
		Enabled:     item.Enabled,
		CreatedAt:   timeutil.FormatRFC3339(item.CreatedAt),
		UpdatedAt:   timeutil.FormatRFC3339(item.UpdatedAt),
		Version:     version,
	}, nil
}

// EnvironmentMemberToProto reuses the account and role mappings so membership responses remain consistent.
//
// Parameters: item is a persisted account with its role and permission records.
// Returns: the public member, or a nested account or role conversion error.
// Throws: none; failures are returned as errors.
func EnvironmentMemberToProto(item *repository.EnvironmentMemberRecord) (*commonv1.EnvironmentMember, error) {
	user, err := UserToProto(&item.User)
	if err != nil {
		return nil, err
	}
	roles := make([]*commonv1.Role, 0, len(item.Roles))
	for index := range item.Roles {
		converted, err := RoleToProto(&item.Roles[index])
		if err != nil {
			return nil, err
		}
		roles = append(roles, converted)
	}
	return &commonv1.EnvironmentMember{
		User:  user,
		Roles: roles,
	}, nil
}

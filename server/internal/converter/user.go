package converter

import (
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
)

// UserToProto exposes public account fields while excluding password hashes and session data.
//
// Parameters: item is the persisted account.
// Returns: the public account, or an error when its ID or version is invalid.
// Throws: none; failures are returned as errors.
func UserToProto(item *model.User) (*commonv1.User, error) {
	version, err := validMeta(item.ID, item.Version)
	if err != nil {
		return nil, err
	}
	return &commonv1.User{
		Id:            item.ID,
		Username:      item.Username,
		Nickname:      item.Nickname,
		Email:         item.Email,
		Enabled:       item.Enabled,
		IsSystemAdmin: item.IsSystemAdmin,
		CreatedAt:     timestamp(item.CreatedAt),
		UpdatedAt:     timestamp(item.UpdatedAt),
		Version:       version,
	}, nil
}

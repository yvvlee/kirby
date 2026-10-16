package converter

import (
	"fmt"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/util/safeint"
	"github.com/yvvlee/kirby/server/internal/util/timeutil"
)

// UserToProto exposes public account fields while excluding password hashes and session data.
//
// Parameters: item is the persisted account.
// Returns: the public account, or an error when its ID or version is invalid.
// Throws: none; failures are returned as errors.
func UserToProto(item *model.User) (*commonv1.User, error) {
	if item.ID <= 0 {
		return nil, fmt.Errorf("invalid user record")
	}
	version, err := safeint.Uint32FromInt64(item.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid user record")
	}
	return &commonv1.User{
		Id:            item.ID,
		Username:      item.Username,
		DisplayName:   item.DisplayName,
		Enabled:       item.Enabled,
		IsSystemAdmin: item.IsSystemAdmin,
		CreatedAt:     timeutil.FormatRFC3339(item.CreatedAt),
		UpdatedAt:     timeutil.FormatRFC3339(item.UpdatedAt),
		Version:       version,
	}, nil
}

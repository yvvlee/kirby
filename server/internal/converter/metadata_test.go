package converter

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestMetadataVersionRejectsOverflow(t *testing.T) {
	for _, version := range []int64{-1, math.MaxUint32 + 1} {
		_, err := validMeta(1, version)
		require.Error(t, err)
	}
	version, err := validMeta(1, math.MaxUint32)
	require.NoError(t, err)
	require.Equal(t, uint32(math.MaxUint32), version)
}

func TestAccountAndEnvironmentRejectInvalidVersions(t *testing.T) {
	_, err := UserToProto(&model.User{
		Meta: model.Meta{
			ID:      1,
			Version: math.MaxUint32 + 1,
		},
	})
	require.Error(t, err)
	_, err = EnvironmentToProto(&model.Environment{
		Meta: model.Meta{
			ID:      1,
			Version: -1,
		},
	})
	require.Error(t, err)
}

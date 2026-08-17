package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
)

func TestConfigSnapshotRoundTripsProtobufOneofs(t *testing.T) {
	value := &ConfigSnapshot{
		Config:     &commonv1.Config{Id: 1, ProjectId: 2, Key: "user", Type: structureType("User"), Value: `{"name":"Ada"}`},
		Structures: []*commonv1.Structure{{Id: 3, ConfigId: 1, Key: "User", Name: "User", Fields: []*commonv1.Field{{Key: "name", Name: "Name", Type: baseType(commonv1.Field_STRING)}}}},
		Enums:      []*commonv1.ConfigEnum{},
		Tree:       &commonv1.TreeNode{Value: &commonv1.Field{Key: "user", Name: "User", Type: structureType("User")}},
	}
	encoded, err := EncodeConfigSnapshot(value)
	require.NoError(t, err)
	decoded, err := DecodeConfigSnapshot(encoded)
	require.NoError(t, err)
	assert.True(t, proto.Equal(value.Config, decoded.Config))
	assert.True(t, proto.Equal(value.Structures[0], decoded.Structures[0]))
	assert.True(t, proto.Equal(value.Tree, decoded.Tree))
}

func TestConfigSnapshotRejectsUnknownAndTrailingContent(t *testing.T) {
	_, err := DecodeConfigSnapshot(`{"config":{},"structures":[],"enums":[],"tree":{},"unknown":true}`)
	assert.Error(t, err)
	_, err = DecodeConfigSnapshot(`{"config":{},"structures":[],"enums":[],"tree":{}} {}`)
	assert.Error(t, err)
}

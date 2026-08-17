package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
)

func TestFieldsRoundTripOneofTypes(t *testing.T) {
	values := []*commonv1.Field{
		{Key: "name", Name: "Name", Type: &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}},
		{Key: "profile", Name: "Profile", Type: &commonv1.Field_Type{Kind: &commonv1.Field_Type_StructureKey{StructureKey: "Profile"}}},
		{Key: "status", Name: "Status", Type: &commonv1.Field_Type{Kind: &commonv1.Field_Type_EnumKey{EnumKey: "Status"}}},
	}
	encoded, err := EncodeFields(values)
	require.NoError(t, err)
	decoded, err := DecodeFields(encoded)
	require.NoError(t, err)
	require.Len(t, decoded, len(values))
	for index := range values {
		assert.True(t, proto.Equal(values[index], decoded[index]))
	}
}

func TestFieldsRejectUnknownProperties(t *testing.T) {
	_, err := DecodeFields(`[{"key":"name","unknown":true}]`)
	assert.Error(t, err)
}

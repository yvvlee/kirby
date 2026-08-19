package entity

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
)

func TestSchemaValidatesReferencesCyclesAndValues(t *testing.T) {
	status := &commonv1.ConfigEnum{Key: "Status", Name: "Status", Values: []*commonv1.SelectOption{{Label: "Active", Value: "ACTIVE"}}}
	user := &commonv1.Structure{Key: "User", Name: "User", Fields: []*commonv1.Field{
		{Key: "name", Name: "Name", Type: baseType(commonv1.Field_STRING)},
		{Key: "status", Name: "Status", Type: enumType("Status")},
	}}
	schema, err := NewSchema([]*commonv1.Structure{user}, []*commonv1.ConfigEnum{status})
	require.NoError(t, err)

	valid := &commonv1.Config{Key: "users", Type: structureType("User"), IsArray: true, Value: `[ {"name":"Ada","status":"ACTIVE"} ]`}
	require.NoError(t, schema.ValidateConfig(valid))

	invalid := &commonv1.Config{Key: "users", Type: structureType("User"), Value: `{"name":"Ada","status":"DISABLED"}`}
	assert.ErrorIs(t, schema.ValidateConfig(invalid), ErrInvalid)
	invalid.Value = `{"name":"Ada","status":"ACTIVE","extra":true}`
	assert.ErrorIs(t, schema.ValidateConfig(invalid), ErrInvalid)
	invalid.Value = `{"name":"Ada","status":"ACTIVE"} trailing`
	assert.ErrorIs(t, schema.ValidateConfig(invalid), ErrInvalid)

	_, err = NewSchema([]*commonv1.Structure{
		{Key: "A", Name: "A", Fields: []*commonv1.Field{{Key: "b", Name: "B", Type: structureType("B")}}},
		{Key: "B", Name: "B", Fields: []*commonv1.Field{{Key: "a", Name: "A", Type: structureType("A")}}},
	}, nil)
	assert.ErrorIs(t, err, ErrConflict)

	_, err = NewSchema([]*commonv1.Structure{{Key: "A", Name: "A", Fields: []*commonv1.Field{{Key: "missing", Name: "Missing", Type: enumType("Missing")}}}}, nil)
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestSchemaEditCanTemporarilyInvalidateDraftButSnapshotGateRejectsIt(t *testing.T) {
	before, err := NewSchema([]*commonv1.Structure{{Key: "User", Name: "User", Fields: []*commonv1.Field{{Key: "name", Name: "Name", Type: baseType(commonv1.Field_STRING)}}}}, nil)
	require.NoError(t, err)
	draft := &commonv1.Config{Key: "user", Type: structureType("User"), Value: `{"name":"Ada"}`}
	require.NoError(t, before.ValidateConfig(draft))

	after, err := NewSchema([]*commonv1.Structure{{Key: "User", Name: "User", Fields: []*commonv1.Field{
		{Key: "name", Name: "Name", Type: baseType(commonv1.Field_STRING)},
		{Key: "age", Name: "Age", Type: baseType(commonv1.Field_INT)},
	}}}, nil)
	require.NoError(t, err, "schema edit itself must be possible before the value is repaired")
	assert.ErrorIs(t, after.ValidateConfig(draft), ErrInvalid, "snapshot creation and publication must reject the invalid draft")
	draft.Value = `{"name":"Ada","age":42}`
	require.NoError(t, after.ValidateConfig(draft))
}

func TestFilterStructureChoicesRemovesDependents(t *testing.T) {
	items := []*commonv1.Structure{
		{Id: 1, Key: "A", Name: "A"},
		{Id: 2, Key: "B", Name: "B", Fields: []*commonv1.Field{{Key: "a", Name: "A", Type: structureType("A")}}},
		{Id: 3, Key: "C", Name: "C", Fields: []*commonv1.Field{{Key: "b", Name: "B", Type: structureType("B")}}},
		{Id: 4, Key: "D", Name: "D"},
	}
	filtered, err := FilterStructureChoices(items, 1)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, int64(4), filtered[0].Id)
}

func TestSchemaRejectsUnboundedNestedMetadata(t *testing.T) {
	_, err := NewSchema([]*commonv1.Structure{{Key: "bad-key", Name: "Valid"}}, nil)
	assert.True(t, errors.Is(err, ErrInvalid))
	_, err = NewSchema(nil, []*commonv1.ConfigEnum{{Key: "Status", Name: "Status", Values: []*commonv1.SelectOption{{Label: "bad", Value: "not_upper"}}}})
	assert.ErrorIs(t, err, ErrInvalid)
}

func baseType(value commonv1.Field_BaseType) *commonv1.Field_Type {
	return &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: value}}
}
func structureType(key string) *commonv1.Field_Type {
	return &commonv1.Field_Type{Kind: &commonv1.Field_Type_StructureKey{StructureKey: key}}
}
func enumType(key string) *commonv1.Field_Type {
	return &commonv1.Field_Type{Kind: &commonv1.Field_Type_EnumKey{EnumKey: key}}
}

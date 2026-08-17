package base

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizePageUsesDefaultsAndCap(t *testing.T) {
	assert.Equal(t, PageRequest{Offset: 0, Limit: DefaultPageSize}, NormalizePage(PageRequest{Offset: -1}))
	assert.Equal(t, PageRequest{Offset: 7, Limit: MaxPageSize}, NormalizePage(PageRequest{Offset: 7, Limit: MaxPageSize + 1}))
}

func TestErrorsCanBeClassified(t *testing.T) {
	assert.True(t, errors.Is(Missing("project"), ErrNotFound))
	assert.True(t, errors.Is(Unchanged("project"), ErrNoRowsAffected))
	assert.True(t, errors.Is(InvalidArgument("environment_id"), ErrInvalidArgument))
}

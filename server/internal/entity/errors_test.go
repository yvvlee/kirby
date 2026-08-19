package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"

	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	"github.com/yvvlee/kirby/server/internal/repository"
)

func TestRepositoryKeyConflictMapsToHTTPConflict(t *testing.T) {
	assert.True(t, errorsv1.IsConflict(APIError(repository.ErrKeyConflict)))
}

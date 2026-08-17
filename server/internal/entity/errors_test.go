package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"

	errorsv1 "github.com/yvvlee/kirby/server/gen/kirby/errors/v1"
	"github.com/yvvlee/kirby/server/internal/repository"
)

func TestRepositoryKeyConflictMapsToHTTPConflict(t *testing.T) {
	assert.True(t, errorsv1.IsConflict(APIError(repository.ErrKeyConflict)))
}

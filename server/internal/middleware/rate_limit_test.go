package middleware

import (
	"context"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/storage/cache"
)

func TestFixedWindowRateLimitSharesCacheCounter(t *testing.T) {
	store := cache.NewMemory()
	limit := FixedWindowRateLimit(store, "test", 2, time.Minute, func(context.Context) string { return "same-client" })
	handler := limit(func(context.Context, any) (any, error) { return "ok", nil })
	for range 2 {
		result, err := handler(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", result)
	}
	_, err := handler(context.Background(), nil)
	require.Error(t, err)
	assert.Equal(t, int32(429), kratoserrors.FromError(err).Code)
}

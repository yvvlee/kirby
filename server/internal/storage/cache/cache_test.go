package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/config"
)

func TestMemoryCopiesValuesAndExpiresThem(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	value := []byte("original")
	require.NoError(t, store.Set(ctx, "key", value, 5*time.Millisecond))
	value[0] = 'X'

	stored, err := store.Get(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, []byte("original"), stored)
	stored[0] = 'X'
	storedAgain, err := store.Get(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, []byte("original"), storedAgain)

	time.Sleep(10 * time.Millisecond)
	_, err = store.Get(ctx, "key")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestRedisUsesConfiguredPrefix(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	store, err := Open(ctx, config.CacheConfig{
		Driver: "redis",
		Redis: config.RedisConfig{
			Address:   server.Addr(),
			KeyPrefix: "kirby:",
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(t, store.Set(ctx, "environment:1", []byte("value"), 0))
	stored, err := server.Get("kirby:environment:1")
	require.NoError(t, err)
	assert.Equal(t, "value", stored)
}

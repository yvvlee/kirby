package cache

import (
	"context"
	"fmt"
	"sync"
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

func TestMemoryIncrementIsAtomicUnderConcurrency(t *testing.T) {
	const workers = 500
	ctx := context.Background()
	store := NewMemory()
	results := make(chan int64, workers)
	errors := make(chan error, workers)
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			value, err := store.Increment(ctx, "login:user-1", time.Minute)
			if err != nil {
				errors <- err
				return
			}
			results <- value
		}()
	}
	group.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	seen := make(map[int64]struct{}, workers)
	for result := range results {
		seen[result] = struct{}{}
	}
	assert.Len(t, seen, workers)
	for expected := int64(1); expected <= workers; expected++ {
		_, ok := seen[expected]
		assert.True(t, ok, "missing atomic increment result %d", expected)
	}
	stored, err := store.Get(ctx, "login:user-1")
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprint(workers), string(stored))
}

func TestMemoryIncrementKeepsFirstTTL(t *testing.T) {
	now := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	store := newMemory(func() time.Time { return now })
	ctx := context.Background()

	value, err := store.Increment(ctx, "login:user-1", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), value)
	firstExpiry := store.entries["login:user-1"].expiresAt

	now = now.Add(30 * time.Second)
	value, err = store.Increment(ctx, "login:user-1", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(2), value)
	assert.Equal(t, firstExpiry, store.entries["login:user-1"].expiresAt)

	now = now.Add(31 * time.Second)
	value, err = store.Increment(ctx, "login:user-1", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), value)
	assert.Equal(t, now.Add(time.Minute), store.entries["login:user-1"].expiresAt)
}

func TestRedisIncrementIsAtomicAndKeepsFirstTTL(t *testing.T) {
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

	value, err := store.Increment(ctx, "login:user-1", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), value)
	assert.Equal(t, time.Minute, server.TTL("kirby:login:user-1"))

	server.FastForward(30 * time.Second)
	value, err = store.Increment(ctx, "login:user-1", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(2), value)
	assert.Equal(t, 30*time.Second, server.TTL("kirby:login:user-1"))

	server.FastForward(31 * time.Second)
	value, err = store.Increment(ctx, "login:user-1", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), value)
	assert.Equal(t, time.Minute, server.TTL("kirby:login:user-1"))
}

func TestRedisIncrementIsAtomicUnderConcurrency(t *testing.T) {
	const workers = 100
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

	results := make(chan int64, workers)
	errors := make(chan error, workers)
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			value, incrementErr := store.Increment(ctx, "login:user-1", time.Minute)
			if incrementErr != nil {
				errors <- incrementErr
				return
			}
			results <- value
		}()
	}
	group.Wait()
	close(results)
	close(errors)
	for incrementErr := range errors {
		require.NoError(t, incrementErr)
	}
	seen := make(map[int64]struct{}, workers)
	for result := range results {
		seen[result] = struct{}{}
	}
	assert.Len(t, seen, workers)
	assert.Equal(t, time.Minute, server.TTL("kirby:login:user-1"))
}

func TestIncrementRequiresPositiveTTL(t *testing.T) {
	_, err := NewMemory().Increment(context.Background(), "key", 0)
	assert.ErrorIs(t, err, ErrInvalidTTL)
}

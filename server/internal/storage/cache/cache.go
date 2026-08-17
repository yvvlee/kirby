// Package cache provides process-local and Redis-backed byte caches.
package cache

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/yvvlee/kirby/server/internal/config"
)

var (
	ErrNotFound   = errors.New("cache entry not found")
	ErrInvalidTTL = errors.New("cache TTL must be greater than zero")
)

// Store is the cache contract used by repositories and runtime delivery.
type Store interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
	Increment(context.Context, string, time.Duration) (int64, error)
	Delete(context.Context, string) error
	Close() error
}

// Open creates the selected cache and checks shared Redis connectivity.
func Open(ctx context.Context, cfg config.CacheConfig) (Store, error) {
	switch cfg.Driver {
	case "memory":
		return NewMemory(), nil
	case "redis":
		if cfg.Redis.Address == "" {
			return nil, fmt.Errorf("Redis address is required")
		}
		if cfg.Redis.KeyPrefix == "" {
			return nil, fmt.Errorf("Redis key prefix is required")
		}
		client := redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Address,
			Username: cfg.Redis.Username,
			Password: cfg.Redis.Password.Value(),
			DB:       cfg.Redis.DB,
		})
		if err := client.Ping(ctx).Err(); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("connect to Redis: %w", err)
		}
		return &redisStore{client: client, prefix: cfg.Redis.KeyPrefix}, nil
	default:
		return nil, fmt.Errorf("unsupported cache driver %q", cfg.Driver)
	}
}

type memoryEntry struct {
	value     []byte
	expiresAt time.Time
}

type memoryStore struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
	now     func() time.Time
}

// NewMemory creates an in-process cache for single-instance deployments.
func NewMemory() Store {
	return newMemory(time.Now)
}

func newMemory(now func() time.Time) *memoryStore {
	return &memoryStore{entries: make(map[string]memoryEntry), now: now}
}

func (s *memoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	entry, ok := s.entries[key]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	if !entry.expiresAt.IsZero() && !s.now().Before(entry.expiresAt) {
		delete(s.entries, key)
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	s.mu.Unlock()
	return append([]byte(nil), entry.value...), nil
}

func (s *memoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ttl < 0 {
		return fmt.Errorf("cache TTL cannot be negative")
	}
	entry := memoryEntry{value: append([]byte(nil), value...)}
	if ttl > 0 {
		entry.expiresAt = s.now().Add(ttl)
	}
	s.mu.Lock()
	s.entries[key] = entry
	s.mu.Unlock()
	return nil
}

func (s *memoryStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if ttl <= 0 {
		return 0, ErrInvalidTTL
	}

	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[key]
	if exists && !entry.expiresAt.IsZero() && !now.Before(entry.expiresAt) {
		delete(s.entries, key)
		exists = false
	}
	if !exists {
		s.entries[key] = memoryEntry{
			value:     []byte("1"),
			expiresAt: now.Add(ttl),
		}
		return 1, nil
	}

	current, err := strconv.ParseInt(string(entry.value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("increment cache entry %q: value is not an integer", key)
	}
	if current == math.MaxInt64 {
		return 0, fmt.Errorf("increment cache entry %q: counter overflow", key)
	}
	current++
	entry.value = []byte(strconv.FormatInt(current, 10))
	s.entries[key] = entry
	return current, nil
}

func (s *memoryStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.entries, key)
	s.mu.Unlock()
	return nil
}

func (*memoryStore) Close() error { return nil }

type redisStore struct {
	client *redis.Client
	prefix string
}

var incrementScript = redis.NewScript(`
local value = redis.call("INCR", KEYS[1])
if value == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return value
`)

func (s *redisStore) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := s.client.Get(ctx, s.prefix+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read Redis cache: %w", err)
	}
	return value, nil
}

func (s *redisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl < 0 {
		return fmt.Errorf("cache TTL cannot be negative")
	}
	if err := s.client.Set(ctx, s.prefix+key, value, ttl).Err(); err != nil {
		return fmt.Errorf("write Redis cache: %w", err)
	}
	return nil
}

func (s *redisStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if ttl <= 0 {
		return 0, ErrInvalidTTL
	}
	ttlMilliseconds := ttl.Milliseconds()
	if ttlMilliseconds < 1 {
		ttlMilliseconds = 1
	}
	value, err := incrementScript.Run(ctx, s.client, []string{s.prefix + key}, ttlMilliseconds).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment Redis cache: %w", err)
	}
	return value, nil
}

func (s *redisStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, s.prefix+key).Err(); err != nil {
		return fmt.Errorf("delete Redis cache: %w", err)
	}
	return nil
}

func (s *redisStore) Close() error {
	return s.client.Close()
}

// Package cache provides process-local and Redis-backed byte caches.
package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/yvvlee/kirby/server/internal/config"
)

var ErrNotFound = errors.New("cache entry not found")

// Store is the cache contract used by repositories and runtime delivery.
type Store interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
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
}

// NewMemory creates an in-process cache for single-instance deployments.
func NewMemory() Store {
	return &memoryStore{entries: make(map[string]memoryEntry)}
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
	if !entry.expiresAt.IsZero() && !time.Now().Before(entry.expiresAt) {
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
		entry.expiresAt = time.Now().Add(ttl)
	}
	s.mu.Lock()
	s.entries[key] = entry
	s.mu.Unlock()
	return nil
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

func (s *redisStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, s.prefix+key).Err(); err != nil {
		return fmt.Errorf("delete Redis cache: %w", err)
	}
	return nil
}

func (s *redisStore) Close() error {
	return s.client.Close()
}

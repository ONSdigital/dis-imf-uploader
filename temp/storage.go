package temp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/redis/go-redis/v9"
)

// Storage interface for temporary file storage.
type Storage interface {
	Store(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	SetTTL(ctx context.Context, key string, ttl time.Duration) error
}

// InMemoryStorage provides in-memory temporary storage for testing and
// local development.
type InMemoryStorage struct {
	data map[string][]byte
	mu   sync.RWMutex
}

// NewInMemoryStorage creates a new in-memory storage instance.
func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		data: make(map[string][]byte),
	}
}

// Store saves data to in-memory storage with the given key.
func (s *InMemoryStorage) Store(ctx context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = data
	return nil
}

// Get retrieves data from in-memory storage by key.
func (s *InMemoryStorage) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return data, nil
}

// Delete removes data from in-memory storage by key.
func (s *InMemoryStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

// SetTTL sets a time-to-live for a key. Not implemented for in-memory storage.
func (s *InMemoryStorage) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	// TTL not implemented for in-memory storage
	return nil
}

// RedisStorage provides Redis-backed temporary storage for production use.
type RedisStorage struct {
	client *redis.Client
	prefix string
}

// NewRedisStorage creates a new Redis storage instance. Falls back to
// in-memory storage if Redis is not configured.
func NewRedisStorage(ctx context.Context, cfg *config.Config) (Storage, error) {
	// If Redis is not configured, fall back to in-memory
	if cfg.Addr == "" {
		return NewInMemoryStorage(), nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorage{
		client: client,
		prefix: cfg.RedisConfig.Prefix,
	}, nil
}

// Store saves data to Redis with the given key.
func (r *RedisStorage) Store(ctx context.Context, key string, data []byte) error {
	fullKey := r.prefix + key
	return r.client.Set(ctx, fullKey, data, 0).Err()
}

// Get retrieves data from Redis by key.
func (r *RedisStorage) Get(ctx context.Context, key string) ([]byte, error) {
	fullKey := r.prefix + key
	data, err := r.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return data, err
}

// Delete removes data from Redis by key.
func (r *RedisStorage) Delete(ctx context.Context, key string) error {
	fullKey := r.prefix + key
	return r.client.Del(ctx, fullKey).Err()
}

// SetTTL sets a time-to-live for a key in Redis.
func (r *RedisStorage) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := r.prefix + key
	return r.client.Expire(ctx, fullKey, ttl).Err()
}

// Close closes the Redis client connection.
func (r *RedisStorage) Close() error {
	return r.client.Close()
}

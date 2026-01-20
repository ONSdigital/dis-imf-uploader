package temp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/redis/go-redis/v9"
)

// Storage interface for temporary file storage
type Storage interface {
	Store(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	SetTTL(ctx context.Context, key string, ttl time.Duration) error
}

// InMemoryStorage for testing/local development
type InMemoryStorage struct {
	data map[string][]byte
	mu   sync.RWMutex
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		data: make(map[string][]byte),
	}
}

func (s *InMemoryStorage) Store(ctx context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = data
	return nil
}

func (s *InMemoryStorage) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return data, nil
}

func (s *InMemoryStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *InMemoryStorage) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	// TTL not implemented for in-memory storage
	return nil
}

// RedisStorage for production
type RedisStorage struct {
	client *redis.Client
	prefix string
}

func NewRedisStorage(ctx context.Context, cfg *config.Config) (Storage, error) {
	// If Redis is not configured, fall back to in-memory
	if cfg.RedisConfig.Addr == "" {
		return NewInMemoryStorage(), nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisConfig.Addr,
		Password:     cfg.RedisConfig.Password,
		DB:           cfg.RedisConfig.DB,
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

func (r *RedisStorage) Store(ctx context.Context, key string, data []byte) error {
	fullKey := r.prefix + key
	return r.client.Set(ctx, fullKey, data, 0).Err()
}

func (r *RedisStorage) Get(ctx context.Context, key string) ([]byte, error) {
	fullKey := r.prefix + key
	data, err := r.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return data, err
}

func (r *RedisStorage) Delete(ctx context.Context, key string) error {
	fullKey := r.prefix + key
	return r.client.Del(ctx, fullKey).Err()
}

func (r *RedisStorage) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := r.prefix + key
	return r.client.Expire(ctx, fullKey, ttl).Err()
}

func (r *RedisStorage) Close() error {
	return r.client.Close()
}

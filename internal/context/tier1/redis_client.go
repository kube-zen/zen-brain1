// Package tier1 provides Redis-based Tier 1 (Hot) storage for ZenContext.
// This file contains the go-redis implementation of the RedisClient interface.
package tier1

import (
	stdctx "context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig holds configuration for the go-redis client.
type RedisConfig struct {
	// URL is the Redis connection URL (e.g., "redis://user:pass@localhost:6379/0").
	// If empty, individual fields are used.
	URL string `json:"url" yaml:"url"`

	// Addr is the Redis server address (e.g., "localhost:6379").
	Addr string `json:"addr" yaml:"addr"`

	// Password for Redis authentication.
	Password string `json:"password" yaml:"password"`

	// DB is the Redis database number (0-15).
	DB int `json:"db" yaml:"db"`

	// PoolSize is the maximum number of socket connections.
	PoolSize int `json:"pool_size" yaml:"pool_size"`

	// MinIdleConns is the minimum number of idle connections.
	MinIdleConns int `json:"min_idle_conns" yaml:"min_idle_conns"`

	// MaxRetries is the maximum number of retries before giving up.
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// DialTimeout is the timeout for establishing new connections.
	DialTimeout time.Duration `json:"dial_timeout" yaml:"dial_timeout"`

	// ReadTimeout is the timeout for socket reads.
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`

	// WriteTimeout is the timeout for socket writes.
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`

	// PoolTimeout is the timeout for acquiring a connection from the pool.
	PoolTimeout time.Duration `json:"pool_timeout" yaml:"pool_timeout"`

	// IdleTimeout is the timeout after which idle connections are closed.
	IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout"`

	// IdleCheckFrequency is how often to check for idle connections.
	IdleCheckFrequency time.Duration `json:"idle_check_frequency" yaml:"idle_check_frequency"`

	// TLSEnabled enables TLS connection.
	TLSEnabled bool `json:"tls_enabled" yaml:"tls_enabled"`
}

// DefaultRedisConfig returns the default Redis configuration.
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		URL:                "",
		Addr:               "localhost:6379",
		Password:           "",
		DB:                 0,
		PoolSize:           10,
		MinIdleConns:       3,
		MaxRetries:         3,
		DialTimeout:        5 * time.Second,
		ReadTimeout:        3 * time.Second,
		WriteTimeout:       3 * time.Second,
		PoolTimeout:        4 * time.Second,
		IdleTimeout:        5 * time.Minute,
		IdleCheckFrequency: 1 * time.Minute,
		TLSEnabled:         false,
	}
}

// goRedisClient implements the RedisClient interface using github.com/redis/go-redis/v9.
type goRedisClient struct {
	client *redis.Client
}

// NewGoRedisClient creates a new RedisClient using go-redis.
// If config is nil, DefaultRedisConfig is used.
func NewGoRedisClient(config *RedisConfig) (RedisClient, error) {
	if config == nil {
		config = DefaultRedisConfig()
	}

	var redisURL string
	if config.URL != "" {
		redisURL = config.URL
	} else {
		// Build URL from individual fields
		scheme := "redis"
		if config.TLSEnabled {
			scheme = "rediss"
		}
		auth := ""
		if config.Password != "" {
			auth = config.Password + "@"
		}
		db := ""
		if config.DB != 0 {
			db = fmt.Sprintf("/%d", config.DB)
		}
		redisURL = fmt.Sprintf("%s://%s%s%s", scheme, auth, config.Addr, db)
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL %s: %w", redisURL, err)
	}

	// Override pool settings if specified
	if config.PoolSize > 0 {
		opts.PoolSize = config.PoolSize
	}
	if config.MinIdleConns > 0 {
		opts.MinIdleConns = config.MinIdleConns
	}
	if config.DialTimeout > 0 {
		opts.DialTimeout = config.DialTimeout
	}
	if config.ReadTimeout > 0 {
		opts.ReadTimeout = config.ReadTimeout
	}
	if config.WriteTimeout > 0 {
		opts.WriteTimeout = config.WriteTimeout
	}
	if config.PoolTimeout > 0 {
		opts.PoolTimeout = config.PoolTimeout
	}
	if config.IdleTimeout > 0 {
		opts.ConnMaxIdleTime = config.IdleTimeout
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("Redis connection test failed: %w", err)
	}

	return &goRedisClient{client: client}, nil
}

// Get retrieves a value by key.
func (c *goRedisClient) Get(ctx stdctx.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil // Not found, return empty string
		}
		return "", err
	}
	return val, nil
}

// Set sets a value with optional expiration.
func (c *goRedisClient) Set(ctx stdctx.Context, key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Delete deletes a key.
func (c *goRedisClient) Delete(ctx stdctx.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists.
func (c *goRedisClient) Exists(ctx stdctx.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	return c.client.Exists(ctx, keys...).Result()
}

// Expire sets an expiration on a key.
func (c *goRedisClient) Expire(ctx stdctx.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

// Keys finds all keys matching a pattern.
func (c *goRedisClient) Keys(ctx stdctx.Context, pattern string) ([]string, error) {
	return c.client.Keys(ctx, pattern).Result()
}

// HGet retrieves a hash field value.
func (c *goRedisClient) HGet(ctx stdctx.Context, key, field string) (string, error) {
	val, err := c.client.HGet(ctx, key, field).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil // Not found, return empty string
		}
		return "", err
	}
	return val, nil
}

// HSet sets a hash field value.
func (c *goRedisClient) HSet(ctx stdctx.Context, key string, values ...interface{}) error {
	return c.client.HSet(ctx, key, values...).Err()
}

// HDel deletes hash fields.
func (c *goRedisClient) HDel(ctx stdctx.Context, key string, fields ...string) error {
	if len(fields) == 0 {
		return nil
	}
	return c.client.HDel(ctx, key, fields...).Err()
}

// HGetAll retrieves all hash fields.
func (c *goRedisClient) HGetAll(ctx stdctx.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

// Ping checks connection to Redis.
func (c *goRedisClient) Ping(ctx stdctx.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the connection.
func (c *goRedisClient) Close() error {
	return c.client.Close()
}

// MustParseRedisURL is a helper to parse Redis URL and create a client.
// This is useful for simple configurations.
func MustParseRedisURL(url string) RedisClient {
	client, err := NewGoRedisClient(&RedisConfig{URL: url})
	if err != nil {
		panic(fmt.Sprintf("Failed to create Redis client from URL %s: %v", url, err))
	}
	return client
}

// IsRedisUnavailableError returns true if the error indicates Redis is unavailable.
func IsRedisUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "NOAUTH") ||
		strings.Contains(errStr, "LOADING")
}
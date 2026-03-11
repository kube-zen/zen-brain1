// Package rediscontext provides Tier 1 (Hot) storage for ZenContext using Redis.
// Redis provides sub-millisecond access for session context with support for
// TTL-based expiration and distributed locking.
package tier1

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// Config holds configuration for Redis context storage.
type Config struct {
	// Redis client (required)
	RedisClient RedisClient

	// KeyPrefix is the Redis key namespace prefix (default: "zen:ctx")
	KeyPrefix string

	// DefaultTTL is the default time-to-live for session keys (default: 30 minutes)
	DefaultTTL time.Duration

	// LockTimeout is the timeout for acquiring session locks (default: 5 seconds)
	LockTimeout time.Duration

	// ClusterID is the cluster identifier for multi-cluster support
	ClusterID string

	// Verbose enables verbose logging
	Verbose bool
}

// RedisClient is the interface for Redis operations.
// This allows using any Redis client library (go-redis, redigo, etc.).
type RedisClient interface {
	// Get retrieves a value by key.
	Get(ctx stdctx.Context, key string) (string, error)

	// Set sets a value with optional expiration.
	Set(ctx stdctx.Context, key string, value interface{}, expiration time.Duration) error

	// Delete deletes a key.
	Delete(ctx stdctx.Context, keys ...string) error

	// Exists checks if a key exists.
	Exists(ctx stdctx.Context, keys ...string) (int64, error)

	// Expire sets an expiration on a key.
	Expire(ctx stdctx.Context, key string, expiration time.Duration) error

	// Keys finds all keys matching a pattern.
	Keys(ctx stdctx.Context, pattern string) ([]string, error)

	// HGet retrieves a hash field value.
	HGet(ctx stdctx.Context, key, field string) (string, error)

	// HSet sets a hash field value.
	HSet(ctx stdctx.Context, key string, values ...interface{}) error

	// HDel deletes hash fields.
	HDel(ctx stdctx.Context, key string, fields ...string) error

	// HGetAll retrieves all hash fields.
	HGetAll(ctx stdctx.Context, key string) (map[string]string, error)

	// Ping checks connection to Redis.
	Ping(ctx stdctx.Context) error

	// Close closes the connection.
	Close() error
}

// Store implements context.ZenContext for Tier 1 (Hot) storage.
// It uses Redis for session context and state persistence.
type Store struct {
	config *Config
	client RedisClient
	mu     sync.RWMutex
}

// NewStore creates a new Redis context store.
func NewStore(config *Config) (*Store, error) {
	if config == nil {
		config = &Config{}
	}

	if config.RedisClient == nil {
		return nil, fmt.Errorf("RedisClient is required")
	}

	if config.KeyPrefix == "" {
		config.KeyPrefix = "zen:ctx"
	}

	if config.DefaultTTL == 0 {
		config.DefaultTTL = 30 * time.Minute
	}

	if config.LockTimeout == 0 {
		config.LockTimeout = 5 * time.Second
	}

	// Verify Redis connection
	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
	defer cancel()

	if err := config.RedisClient.Ping(ctx); err != nil {
		return nil, fmt.Errorf("Redis ping failed: %w", err)
	}

	return &Store{
		config: config,
		client: config.RedisClient,
	}, nil
}

// GetSessionContext retrieves session context from Tier 1 (Hot).
// Returns nil if session does not exist.
func (s *Store) GetSessionContext(ctx stdctx.Context, clusterID, sessionID string) (*zenctx.SessionContext, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID is required")
	}

	key := s.sessionKey(clusterID, sessionID)

	if s.config.Verbose {
		fmt.Printf("[RedisContext] GetSessionContext: key=%s\n", key)
	}

	data, err := s.client.Get(ctx, key)
	if err != nil {
		// Key not found is not an error, just return nil
		if isRedisNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("Redis Get failed: %w", err)
	}

	var sessionCtx zenctx.SessionContext
	if err := json.Unmarshal([]byte(data), &sessionCtx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session context: %w", err)
	}

	// Update LastAccessedAt
	sessionCtx.LastAccessedAt = time.Now()
	s.updateLastAccessed(ctx, clusterID, sessionID)

	return &sessionCtx, nil
}

// StoreSessionContext stores session context in Tier 1 (Hot).
func (s *Store) StoreSessionContext(ctx stdctx.Context, clusterID string, session *zenctx.SessionContext) error {
	if session == nil {
		return fmt.Errorf("session is required")
	}

	if session.SessionID == "" {
		return fmt.Errorf("session.SessionID is required")
	}

	key := s.sessionKey(clusterID, session.SessionID)

	if s.config.Verbose {
		fmt.Printf("[RedisContext] StoreSessionContext: key=%s, ttl=%v\n", key, s.config.DefaultTTL)
	}

	// Set LastAccessedAt
	session.LastAccessedAt = time.Now()

	// Serialize to JSON
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session context: %w", err)
	}

	// Store in Redis with TTL
	if err := s.client.Set(ctx, key, data, s.config.DefaultTTL); err != nil {
		return fmt.Errorf("Redis Set failed: %w", err)
	}

	// Also store in metadata hash for faster lookups
	metaKey := s.metaKey(clusterID, session.SessionID)
	if err := s.client.HSet(ctx, metaKey,
		"created_at", session.CreatedAt.Format(time.RFC3339Nano),
		"last_accessed", session.LastAccessedAt.Format(time.RFC3339Nano),
		"task_id", session.TaskID,
		"project_id", session.ProjectID,
	); err != nil {
		// Non-fatal: metadata update failed but session stored
		if s.config.Verbose {
			fmt.Printf("[RedisContext] Warning: failed to update metadata: %v\n", err)
		}
	}

	return nil
}

// DeleteSessionContext deletes session context from Tier 1.
func (s *Store) DeleteSessionContext(ctx stdctx.Context, clusterID, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID is required")
	}

	keys := []string{
		s.sessionKey(clusterID, sessionID),
		s.scratchpadKey(clusterID, sessionID),
		s.metaKey(clusterID, sessionID),
		s.tasksKey(clusterID, sessionID),
		s.heartbeatKey(clusterID, sessionID),
		s.lockKey(clusterID, sessionID),
	}

	if s.config.Verbose {
		fmt.Printf("[RedisContext] DeleteSessionContext: deleting %d keys for session=%s\n", len(keys), sessionID)
	}

	if err := s.client.Delete(ctx, keys...); err != nil {
		return fmt.Errorf("Redis Delete failed: %w", err)
	}

	return nil
}

// QueryKnowledge queries Tier 2 (Warm) for relevant knowledge.
// This is implemented by the QMD adapter, so we delegate to that.
// QueryKnowledge is not supported in Tier 1 (Redis).
// Tier 1 is for hot session context, not knowledge queries.
// Use Tier 2 (QMD) adapter for knowledge queries.
func (s *Store) QueryKnowledge(ctx stdctx.Context, opts zenctx.QueryOptions) ([]zenctx.KnowledgeChunk, error) {
	return nil, fmt.Errorf("architectural boundary: QueryKnowledge not supported in Tier 1 (Redis hot cache) - use Tier 2 (QMD knowledge store)")
}

// StoreKnowledge is not supported in Tier 1 (Redis).
// Tier 1 is for hot session context, not knowledge storage.
// Use Tier 2 (QMD) adapter for knowledge storage.
func (s *Store) StoreKnowledge(ctx stdctx.Context, chunks []zenctx.KnowledgeChunk) error {
	return fmt.Errorf("architectural boundary: StoreKnowledge not supported in Tier 1 (Redis hot cache) - use Tier 2 (QMD knowledge store)")
}

// ArchiveSession archives session context to Tier 3 (Cold).
// This is implemented by the S3 archival backend, so we delegate to that.
// ArchiveSession is not supported in Tier 1 (Redis).
// Tier 1 is for hot session context, not long-term archival.
// Use Tier 3 (S3) backend for session archival.
func (s *Store) ArchiveSession(ctx stdctx.Context, clusterID, sessionID string) error {
	return fmt.Errorf("architectural boundary: ArchiveSession not supported in Tier 1 (Redis hot cache) - use Tier 3 (S3 cold storage)")
}

// ReconstructSession implements the ReMe protocol.
// For Tier 1, this retrieves the session context from Redis if it exists.
// Full ReMe protocol (journal query + KB retrieval) is implemented
// by the composite ZenContext implementation.
func (s *Store) ReconstructSession(ctx stdctx.Context, req zenctx.ReMeRequest) (*zenctx.ReMeResponse, error) {
	if s.config.Verbose {
		fmt.Printf("[RedisContext] ReconstructSession: sessionID=%s, taskID=%s\n", req.SessionID, req.TaskID)
	}

	// Try to retrieve session context from Tier 1
	sessionCtx, err := s.GetSessionContext(ctx, req.ClusterID, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve session context: %w", err)
	}

	if sessionCtx == nil {
		// Session not found in Tier 1
		return nil, fmt.Errorf("session not found in Tier 1: %s", req.SessionID)
	}

	return &zenctx.ReMeResponse{
		SessionContext:  sessionCtx,
		JournalEntries:  []interface{}{}, // No journal entries from Tier 1
		ReconstructedAt: time.Now(),
	}, nil
}

// Stats returns memory usage statistics.
func (s *Store) Stats(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
	// Count sessions in this cluster
	pattern := s.sessionKeyPattern(s.config.ClusterID)
	keys, err := s.client.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}

	stats := map[zenctx.Tier]interface{}{
		zenctx.TierHot: map[string]interface{}{
			"type":          "redis",
			"cluster_id":    s.config.ClusterID,
			"session_count": len(keys),
			"key_prefix":    s.config.KeyPrefix,
			"default_ttl":   s.config.DefaultTTL.String(),
		},
	}

	return stats, nil
}

// Close closes the Redis client.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		return s.client.Close()
	}

	return nil
}

// --- Helper Methods ---

// sessionKey returns the Redis key for a session context.
func (s *Store) sessionKey(clusterID, sessionID string) string {
	return fmt.Sprintf("%s:%s:%s:state", s.config.KeyPrefix, clusterID, sessionID)
}

// sessionKeyPattern returns a pattern for finding all session keys.
func (s *Store) sessionKeyPattern(clusterID string) string {
	return fmt.Sprintf("%s:%s:*:state", s.config.KeyPrefix, clusterID)
}

// scratchpadKey returns the Redis key for a scratchpad.
func (s *Store) scratchpadKey(clusterID, sessionID string) string {
	return fmt.Sprintf("%s:%s:%s:scratchpad", s.config.KeyPrefix, clusterID, sessionID)
}

// metaKey returns the Redis key for session metadata.
func (s *Store) metaKey(clusterID, sessionID string) string {
	return fmt.Sprintf("%s:%s:%s:meta", s.config.KeyPrefix, clusterID, sessionID)
}

// tasksKey returns the Redis key for a task list.
func (s *Store) tasksKey(clusterID, sessionID string) string {
	return fmt.Sprintf("%s:%s:%s:tasks", s.config.KeyPrefix, clusterID, sessionID)
}

// heartbeatKey returns the Redis key for a heartbeat timestamp.
func (s *Store) heartbeatKey(clusterID, sessionID string) string {
	return fmt.Sprintf("%s:%s:%s:heartbeat", s.config.KeyPrefix, clusterID, sessionID)
}

// lockKey returns the Redis key for a session lock.
func (s *Store) lockKey(clusterID, sessionID string) string {
	return fmt.Sprintf("%s:%s:%s:lock", s.config.KeyPrefix, clusterID, sessionID)
}

// updateLastAccessed updates the LastAccessedAt timestamp for a session.
func (s *Store) updateLastAccessed(ctx stdctx.Context, clusterID, sessionID string) error {
	metaKey := s.metaKey(clusterID, sessionID)
	now := time.Now()
	return s.client.HSet(ctx, metaKey, "last_accessed", now.Format(time.RFC3339Nano))
}

// isRedisNil checks if an error is a Redis nil/error for missing key.
func isRedisNil(err error) bool {
	if err == nil {
		return false
	}
	// Most Redis clients return a specific error type for nil/missing keys
	// This is a generic check; implementations may need to be more specific
	errStr := err.Error()
	return errStr == "redis: nil" ||
		errStr == "nil" ||
		contains(errStr, "key not found") ||
		contains(errStr, "does not exist")
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Additional Helper Methods for Scratchpad, Tasks, Heartbeat ---

// StoreScratchpad stores the scratchpad binary data.
func (s *Store) StoreScratchpad(ctx stdctx.Context, clusterID, sessionID string, data []byte) error {
	key := s.scratchpadKey(clusterID, sessionID)
	if err := s.client.Set(ctx, key, data, s.config.DefaultTTL); err != nil {
		return fmt.Errorf("failed to store scratchpad: %w", err)
	}
	return nil
}

// GetScratchpad retrieves the scratchpad binary data.
func (s *Store) GetScratchpad(ctx stdctx.Context, clusterID, sessionID string) ([]byte, error) {
	key := s.scratchpadKey(clusterID, sessionID)
	data, err := s.client.Get(ctx, key)
	if err != nil {
		if isRedisNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to retrieve scratchpad: %w", err)
	}
	return []byte(data), nil
}

// StoreTasks stores the task list for a session.
func (s *Store) StoreTasks(ctx stdctx.Context, clusterID, sessionID string, tasks []string) error {
	key := s.tasksKey(clusterID, sessionID)
	data, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}
	if err := s.client.Set(ctx, key, data, s.config.DefaultTTL); err != nil {
		return fmt.Errorf("failed to store tasks: %w", err)
	}
	return nil
}

// GetTasks retrieves the task list for a session.
func (s *Store) GetTasks(ctx stdctx.Context, clusterID, sessionID string) ([]string, error) {
	key := s.tasksKey(clusterID, sessionID)
	data, err := s.client.Get(ctx, key)
	if err != nil {
		if isRedisNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to retrieve tasks: %w", err)
	}
	var tasks []string
	if err := json.Unmarshal([]byte(data), &tasks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tasks: %w", err)
	}
	return tasks, nil
}

// UpdateHeartbeat updates the heartbeat timestamp for a session.
func (s *Store) UpdateHeartbeat(ctx stdctx.Context, clusterID, sessionID string) error {
	key := s.heartbeatKey(clusterID, sessionID)
	now := time.Now().Format(time.RFC3339Nano)
	if err := s.client.Set(ctx, key, now, 1*time.Minute); err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}
	return nil
}

// GetHeartbeat retrieves the last heartbeat timestamp for a session.
func (s *Store) GetHeartbeat(ctx stdctx.Context, clusterID, sessionID string) (time.Time, error) {
	key := s.heartbeatKey(clusterID, sessionID)
	data, err := s.client.Get(ctx, key)
	if err != nil {
		if isRedisNil(err) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to retrieve heartbeat: %w", err)
	}
	return time.Parse(time.RFC3339Nano, data)
}

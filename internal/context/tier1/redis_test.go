// Package rediscontext provides tests for the Tier 1 Redis context store.
package tier1

import (
	stdctx "context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// mockRedisClient is a mock implementation of RedisClient for testing.
type mockRedisClient struct {
	data       map[string]string
	hashes     map[string]map[string]string
	keys       []string
	expirations map[string]time.Time

	mu sync.RWMutex
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{
		data:       make(map[string]string),
		hashes:     make(map[string]map[string]string),
		keys:       []string{},
		expirations: make(map[string]time.Time),
	}
}

func (m *mockRedisClient) Get(ctx stdctx.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	if !ok {
		return "", fmt.Errorf("redis: nil")
	}
	return val, nil
}

func (m *mockRedisClient) Set(ctx stdctx.Context, key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var strVal string
	switch v := value.(type) {
	case string:
		strVal = v
	case []byte:
		strVal = string(v)
	default:
		return fmt.Errorf("unsupported type")
	}
	if _, ok := m.data[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.data[key] = strVal
	if expiration > 0 {
		m.expirations[key] = time.Now().Add(expiration)
	}
	return nil
}

func (m *mockRedisClient) Delete(ctx stdctx.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, key)
		delete(m.hashes, key)
		delete(m.expirations, key)
		// Remove from keys slice
		for i, k := range m.keys {
			if k == key {
				m.keys = append(m.keys[:i], m.keys[i+1:]...)
				break
			}
		}
	}
	return nil
}

func (m *mockRedisClient) Exists(ctx stdctx.Context, keys ...string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := int64(0)
	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			count++
		}
	}
	return count, nil
}

func (m *mockRedisClient) Expire(ctx stdctx.Context, key string, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[key]; ok {
		m.expirations[key] = time.Now().Add(expiration)
	}
	return nil
}

func (m *mockRedisClient) Keys(ctx stdctx.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Simple pattern matching - supports * wildcard
	if pattern == "*" {
		return m.keys, nil
	}
	var result []string
	for _, key := range m.keys {
		if matchPattern(key, pattern) {
			result = append(result, key)
		}
	}
	return result, nil
}

// matchPattern checks if a key matches a pattern with * wildcards
func matchPattern(key, pattern string) bool {
	// Simple implementation: split by * and check all parts
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		// No wildcard, exact match
		return key == pattern
	}

	// Check prefix
	if !strings.HasPrefix(key, parts[0]) {
		return false
	}

	// Check suffix
	if !strings.HasSuffix(key, parts[len(parts)-1]) {
		return false
	}

	// Check middle parts
	remaining := key[len(parts[0]):]
	for _, part := range parts[1 : len(parts)-1] {
		idx := strings.Index(remaining, part)
		if idx == -1 {
			return false
		}
		remaining = remaining[idx+len(part):]
	}

	return true
}

func (m *mockRedisClient) HGet(ctx stdctx.Context, key, field string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.hashes[key]
	if !ok {
		return "", fmt.Errorf("redis: nil")
	}
	val, ok := h[field]
	if !ok {
		return "", fmt.Errorf("redis: nil")
	}
	return val, nil
}

func (m *mockRedisClient) HSet(ctx stdctx.Context, key string, values ...interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hashes[key]; !ok {
		m.hashes[key] = make(map[string]string)
	}
	// Values should be alternating field, value
	for i := 0; i < len(values); i += 2 {
		if i+1 >= len(values) {
			break
		}
		field := fmt.Sprintf("%v", values[i])
		value := fmt.Sprintf("%v", values[i+1])
		m.hashes[key][field] = value
	}
	return nil
}

func (m *mockRedisClient) HDel(ctx stdctx.Context, key string, fields ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hashes[key]
	if !ok {
		return nil
	}
	for _, field := range fields {
		delete(h, field)
	}
	return nil
}

func (m *mockRedisClient) HGetAll(ctx stdctx.Context, key string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.hashes[key]
	if !ok {
		return nil, fmt.Errorf("redis: nil")
	}
	result := make(map[string]string)
	for k, v := range h {
		result[k] = v
	}
	return result, nil
}

func (m *mockRedisClient) Ping(ctx stdctx.Context) error {
	return nil
}

func (m *mockRedisClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]string)
	m.hashes = make(map[string]map[string]string)
	m.keys = []string{}
	m.expirations = make(map[string]time.Time)
	return nil
}

func TestNewStore(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if store == nil {
		t.Fatal("NewStore returned nil store")
	}

	store.Close()
}

func TestStore_GetSessionContext_NotFound(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := stdctx.Background()
	sessionCtx, err := store.GetSessionContext(ctx, "cluster-1", "nonexistent")
	if err != nil {
		t.Fatalf("GetSessionContext failed: %v", err)
	}

	if sessionCtx != nil {
		t.Error("Expected nil for non-existent session")
	}
}

func TestStore_StoreSessionContext(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := stdctx.Background()
	session := &zenctx.SessionContext{
		SessionID:     "session-123",
		TaskID:        "task-456",
		ClusterID:     "cluster-1",
		ProjectID:     "project-789",
		CreatedAt:     time.Now(),
		
		
		
	}

	err = store.StoreSessionContext(ctx, "cluster-1", session)
	if err != nil {
		t.Fatalf("StoreSessionContext failed: %v", err)
	}

	// Verify key was created
	key := "test:ctx:cluster-1:session-123:state"
	val, err := mockClient.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to retrieve stored session: %v", err)
	}

	if val == "" {
		t.Error("Expected non-empty stored value")
	}
}

func TestStore_StoreAndGetSessionContext(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := stdctx.Background()

	// Store session
	session := &zenctx.SessionContext{
		SessionID:   "session-123",
		TaskID:      "task-456",
		ClusterID:   "cluster-1",
		ProjectID:   "project-789",
		
		
		
		Scratchpad:  []byte("test scratchpad data"),
	}

	err = store.StoreSessionContext(ctx, "cluster-1", session)
	if err != nil {
		t.Fatalf("StoreSessionContext failed: %v", err)
	}

	// Retrieve session
	retrieved, err := store.GetSessionContext(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("GetSessionContext failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected non-nil retrieved session")
	}

	// Verify fields
	if retrieved.SessionID != session.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", retrieved.SessionID, session.SessionID)
	}

	if retrieved.TaskID != session.TaskID {
		t.Errorf("TaskID mismatch: got %s, want %s", retrieved.TaskID, session.TaskID)
	}

	if retrieved.ClusterID != session.ClusterID {
		t.Errorf("ClusterID mismatch: got %s, want %s", retrieved.ClusterID, session.ClusterID)
	}

	// LastAccessedAt should be updated
	if retrieved.LastAccessedAt.Before(session.CreatedAt) {
		t.Error("LastAccessedAt should be after CreatedAt")
	}
}

func TestStore_DeleteSessionContext(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := stdctx.Background()

	// Store session
	session := &zenctx.SessionContext{
		SessionID: "session-123",
		TaskID:    "task-456",
	}

	err = store.StoreSessionContext(ctx, "cluster-1", session)
	if err != nil {
		t.Fatalf("StoreSessionContext failed: %v", err)
	}

	// Verify it exists
	_, err = store.GetSessionContext(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("GetSessionContext failed before delete: %v", err)
	}

	// Delete session
	err = store.DeleteSessionContext(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("DeleteSessionContext failed: %v", err)
	}

	// Verify it's gone
	retrieved, err := store.GetSessionContext(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("GetSessionContext failed after delete: %v", err)
	}

	if retrieved != nil {
		t.Error("Expected nil after delete")
	}
}

func TestStore_Scratchpad(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := stdctx.Background()

	// Store scratchpad
	data := []byte("this is test scratchpad data with some content")
	err = store.StoreScratchpad(ctx, "cluster-1", "session-123", data)
	if err != nil {
		t.Fatalf("StoreScratchpad failed: %v", err)
	}

	// Retrieve scratchpad
	retrieved, err := store.GetScratchpad(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("GetScratchpad failed: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("Scratchpad data mismatch: got %q, want %q", string(retrieved), string(data))
	}
}

func TestStore_Tasks(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := stdctx.Background()

	// Store tasks
	tasks := []string{"task-1", "task-2", "task-3"}
	err = store.StoreTasks(ctx, "cluster-1", "session-123", tasks)
	if err != nil {
		t.Fatalf("StoreTasks failed: %v", err)
	}

	// Retrieve tasks
	retrieved, err := store.GetTasks(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("GetTasks failed: %v", err)
	}

	if len(retrieved) != len(tasks) {
		t.Errorf("Tasks length mismatch: got %d, want %d", len(retrieved), len(tasks))
	}

	for i, task := range retrieved {
		if task != tasks[i] {
			t.Errorf("Task %d mismatch: got %s, want %s", i, task, tasks[i])
		}
	}
}

func TestStore_Heartbeat(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := stdctx.Background()

	// Update heartbeat
	err = store.UpdateHeartbeat(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}

	// Retrieve heartbeat
	heartbeat, err := store.GetHeartbeat(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("GetHeartbeat failed: %v", err)
	}

	// Heartbeat should be recent (within 1 second)
	if time.Since(heartbeat) > 1*time.Second {
		t.Errorf("Heartbeat too old: %v", heartbeat)
	}
}

func TestStore_ReconstructSession(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := stdctx.Background()

	// Store session first
	session := &zenctx.SessionContext{
		SessionID:   "session-123",
		TaskID:      "task-456",
		ClusterID:   "cluster-1",
		ProjectID:   "project-789",
		
		
		
		Scratchpad:  []byte("reasoning data"),
	}

	err = store.StoreSessionContext(ctx, "cluster-1", session)
	if err != nil {
		t.Fatalf("StoreSessionContext failed: %v", err)
	}

	// Reconstruct session
	req := zenctx.ReMeRequest{
		SessionID: "session-123",
		TaskID:    "task-456",
		ClusterID: "cluster-1",
		ProjectID: "project-789",
	}

	resp, err := store.ReconstructSession(ctx, req)
	if err != nil {
		t.Fatalf("ReconstructSession failed: %v", err)
	}

	if resp.SessionContext == nil {
		t.Fatal("Expected non-nil SessionContext")
	}

	if resp.SessionContext.SessionID != session.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", resp.SessionContext.SessionID, session.SessionID)
	}

	if resp.SessionContext.TaskID != session.TaskID {
		t.Errorf("TaskID mismatch: got %s, want %s", resp.SessionContext.TaskID, session.TaskID)
	}
}

func TestStore_ReconstructSession_NotFound(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := stdctx.Background()

	req := zenctx.ReMeRequest{
		SessionID: "nonexistent-session",
		TaskID:    "task-456",
		ClusterID: "cluster-1",
	}

	_, err = store.ReconstructSession(ctx, req)
	if err == nil {
		t.Error("Expected error for non-existent session")
	}

	expectedErr := "session not found in Tier 1"
	if err == nil || !contains(err.Error(), expectedErr) {
		t.Errorf("Expected error containing %q, got %v", expectedErr, err)
	}
}

func TestStore_Stats(t *testing.T) {
	mockClient := newMockRedisClient()

	config := &Config{
		RedisClient: mockClient,
		KeyPrefix:   "test:ctx",
		ClusterID:   "test-cluster",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := stdctx.Background()

	// Store some sessions
	for i := 0; i < 3; i++ {
		session := &zenctx.SessionContext{
			SessionID: fmt.Sprintf("session-%d", i),
			TaskID:    fmt.Sprintf("task-%d", i),
		}
		err = store.StoreSessionContext(ctx, "test-cluster", session)
		if err != nil {
			t.Fatalf("StoreSessionContext failed: %v", err)
		}
	}

	// Get stats
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	hotStats, ok := stats[zenctx.TierHot]
	if !ok {
		t.Error("Expected TierHot in stats")
	}

	hotMap, ok := hotStats.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{} for TierHot, got %T", hotStats)
	}

	sessionCount, ok := hotMap["session_count"].(int)
	if !ok {
		t.Fatal("Expected session_count to be int")
	}

	if sessionCount != 3 {
		t.Errorf("Session count mismatch: got %d, want 3", sessionCount)
	}
}

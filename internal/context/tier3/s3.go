// Package s3context provides Tier 3 (Cold) archival storage for ZenContext using S3/MinIO.
// S3 archival provides cost-effective long-term storage for completed sessions
// with lifecycle management and support for multi-cluster access.
package tier3

import (
	"bytes"
	"compress/gzip"
	stdctx "context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// Config holds configuration for S3 archival storage.
type Config struct {
	// S3Client is the S3 client interface (required)
	S3Client S3Client

	// Bucket is the S3 bucket name (default: "zen-brain-context")
	Bucket string

	// KeyPrefix is the S3 key prefix (default: "")
	KeyPrefix string

	// ClusterID is the cluster identifier for multi-cluster support
	ClusterID string

	// EnableGzip enables gzip compression for stored objects (default: true)
	EnableGzip bool

	// RetentionDays is the retention period for sessions in days (default: 90)
	RetentionDays int

	// Verbose enables verbose logging
	Verbose bool
}

// S3Client is the interface for S3 operations.
// This allows using any S3-compatible client (AWS SDK, MinIO, etc.).
type S3Client interface {
	// PutObject uploads an object to S3.
	PutObject(ctx stdctx.Context, key string, body io.Reader, contentType string, metadata map[string]string) error

	// GetObject retrieves an object from S3.
	GetObject(ctx stdctx.Context, key string) ([]byte, error)

	// DeleteObject deletes an object from S3.
	DeleteObject(ctx stdctx.Context, key string) error

	// ListObjects lists objects matching a prefix.
	ListObjects(ctx stdctx.Context, prefix string) ([]string, error)

	// ObjectExists checks if an object exists.
	ObjectExists(ctx stdctx.Context, key string) (bool, error)

	// Close closes the S3 client.
	Close() error
}

// Store implements context.ZenContext for Tier 3 (Cold) archival storage.
// It uses S3/MinIO for long-term archival of completed sessions.
type Store struct {
	config *Config
	client S3Client
	mu     sync.RWMutex
}

// NewStore creates a new S3 archival store.
func NewStore(config *Config) (*Store, error) {
	if config == nil {
		config = &Config{}
	}

	if config.S3Client == nil {
		return nil, fmt.Errorf("S3Client is required")
	}

	if config.Bucket == "" {
		config.Bucket = "zen-brain-context"
	}

	if config.EnableGzip == false {
		config.EnableGzip = true
	}

	if config.RetentionDays == 0 {
		config.RetentionDays = 90
	}

	return &Store{
		config: config,
		client: config.S3Client,
	}, nil
}

// GetSessionContext retrieves session context from Tier 3 (Cold).
// Returns nil if session does not exist.
func (s *Store) GetSessionContext(ctx stdctx.Context, clusterID, sessionID string) (*zenctx.SessionContext, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID is required")
	}

	key := s.sessionKey(clusterID, sessionID)

	if s.config.Verbose {
		fmt.Printf("[S3Context] GetSessionContext: key=%s\n", key)
	}

	exists, err := s.client.ObjectExists(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to check object existence: %w", err)
	}
	if !exists {
		return nil, nil
	}

	data, err := s.client.GetObject(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	// Decompress if gzip was used
	if s.config.EnableGzip {
		data, err = decompress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %w", err)
		}
	}

	var sessionCtx zenctx.SessionContext
	if err := json.Unmarshal(data, &sessionCtx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session context: %w", err)
	}

	return &sessionCtx, nil
}

// StoreSessionContext stores session context in Tier 3 (Cold).
func (s *Store) StoreSessionContext(ctx stdctx.Context, clusterID string, session *zenctx.SessionContext) error {
	if session == nil {
		return fmt.Errorf("session is required")
	}

	if session.SessionID == "" {
		return fmt.Errorf("session.SessionID is required")
	}

	key := s.sessionKey(clusterID, session.SessionID)

	// Always log for debugging (TODO: remove after fixing S3)
	fmt.Printf("[S3Context DEBUG] StoreSessionContext: key='%s', sessionID='%s', clusterID='%s'\n", key, session.SessionID, clusterID)
	
	if s.config.Verbose {
		fmt.Printf("[S3Context] StoreSessionContext: key=%s\n", key)
	}

	// Serialize to JSON
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session context: %w", err)
	}

	// Compress if enabled
	if s.config.EnableGzip {
		data, err = compress(data)
		if err != nil {
			return fmt.Errorf("failed to compress: %w", err)
		}
	}

	// Prepare metadata
	metadata := map[string]string{
		"session_id":     session.SessionID,
		"cluster_id":     clusterID,
		"task_id":        session.TaskID,
		"project_id":     session.ProjectID,
		"created_at":     session.CreatedAt.Format(time.RFC3339),
		"last_accessed":  session.LastAccessedAt.Format(time.RFC3339),
		"agent_type":     session.SessionID,
		"retention_days": fmt.Sprintf("%d", s.config.RetentionDays),
	}

	contentType := "application/json"
	if s.config.EnableGzip {
		contentType = "application/gzip"
	}

	// Upload to S3
	if err := s.client.PutObject(ctx, key, bytes.NewReader(data), contentType, metadata); err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

// DeleteSessionContext deletes session context from Tier 3.
func (s *Store) DeleteSessionContext(ctx stdctx.Context, clusterID, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID is required")
	}

	key := s.sessionKey(clusterID, sessionID)

	if s.config.Verbose {
		fmt.Printf("[S3Context] DeleteSessionContext: key=%s\n", key)
	}

	if err := s.client.DeleteObject(ctx, key); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// QueryKnowledge queries Tier 2 (Warm) for relevant knowledge.
// This is not supported in Tier 3 (S3).
func (s *Store) QueryKnowledge(ctx stdctx.Context, opts zenctx.QueryOptions) ([]zenctx.KnowledgeChunk, error) {
	return nil, fmt.Errorf("QueryKnowledge is not implemented in Tier 3 (S3) - use Tier 2 (QMD) adapter")
}

// StoreKnowledge stores knowledge in Tier 2 (Warm).
// This is not supported in Tier 3 (S3).
func (s *Store) StoreKnowledge(ctx stdctx.Context, chunks []zenctx.KnowledgeChunk) error {
	return fmt.Errorf("StoreKnowledge is not implemented in Tier 3 (S3) - use Tier 2 (QMD) adapter")
}

// ArchiveSession archives session context to Tier 3 (Cold).
// This is the primary operation for Tier 3 - it archives a session to S3.
func (s *Store) ArchiveSession(ctx stdctx.Context, clusterID, sessionID string) error {
	// Note: This method is a no-op for the S3 store itself,
	// since ArchiveSession is the primary operation.
	// The caller (composite ZenContext) should retrieve the session
	// from Tier 1 and then call StoreSessionContext here.
	return fmt.Errorf("ArchiveSession is not implemented directly - use StoreSessionContext after retrieving from Tier 1")
}

// ReconstructSession implements the ReMe protocol.
// For Tier 3, this retrieves the session context from S3 if it exists.
// Full ReMe protocol (journal query + KB retrieval) is implemented
// by the composite ZenContext implementation.
func (s *Store) ReconstructSession(ctx stdctx.Context, req zenctx.ReMeRequest) (*zenctx.ReMeResponse, error) {
	if s.config.Verbose {
		fmt.Printf("[S3Context] ReconstructSession: sessionID=%s, taskID=%s\n", req.SessionID, req.TaskID)
	}

	// Try to retrieve session context from Tier 3
	sessionCtx, err := s.GetSessionContext(ctx, req.ClusterID, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve session context: %w", err)
	}

	if sessionCtx == nil {
		// Session not found in Tier 3
		return nil, fmt.Errorf("session not found in Tier 3: %s", req.SessionID)
	}

	return &zenctx.ReMeResponse{
		SessionContext: sessionCtx,
		JournalEntries: []interface{}{}, // No journal entries from Tier 3
		ReconstructedAt: time.Now(),
	}, nil
}

// Stats returns archival storage statistics.
func (s *Store) Stats(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
	// Count archived sessions in this cluster
	prefix := s.sessionKeyPrefix(s.config.ClusterID)
	keys, err := s.client.ListObjects(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list archived sessions: %w", err)
	}

	stats := map[zenctx.Tier]interface{}{
		zenctx.TierCold: map[string]interface{}{
			"type":            "s3",
			"bucket":          s.config.Bucket,
			"cluster_id":      s.config.ClusterID,
			"session_count":   len(keys),
			"retention_days":  s.config.RetentionDays,
			"compression":     s.config.EnableGzip,
		},
	}

	return stats, nil
}

// Close closes the S3 client.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		return s.client.Close()
	}

	return nil
}

// --- Helper Methods ---

// sessionKey returns the S3 key for a session context.
// Format: sessions/{clusterID}/{year}/{month}/session-{sessionID}-{date}.json[.gz]
func (s *Store) sessionKey(clusterID, sessionID string) string {
	now := time.Now()
	key := fmt.Sprintf("sessions/%s/%04d/%02d/session-%s-%04d%02d%02d.json",
		clusterID,
		now.Year(),
		now.Month(),
		sessionID,
		now.Year(),
		now.Month(),
		now.Day(),
	)

	if s.config.EnableGzip {
		key += ".gz"
	}

	return s.config.KeyPrefix + key
}

// sessionKeyPrefix returns a prefix for finding all session keys.
func (s *Store) sessionKeyPrefix(clusterID string) string {
	return s.config.KeyPrefix + fmt.Sprintf("sessions/%s/", clusterID)
}

// scratchpadKey returns the S3 key for a scratchpad.
// Format: scratchpads/{clusterID}/{year}/{month}/scratchpad-{sessionID}-{date}.bin[.gz]
func (s *Store) scratchpadKey(clusterID, sessionID string) string {
	now := time.Now()
	key := fmt.Sprintf("scratchpads/%s/%04d/%02d/scratchpad-%s-%04d%02d%02d.bin",
		clusterID,
		now.Year(),
		now.Month(),
		sessionID,
		now.Year(),
		now.Month(),
		now.Day(),
	)

	if s.config.EnableGzip {
		key += ".gz"
	}

	return s.config.KeyPrefix + key
}

// StoreScratchpad archives a scratchpad to S3.
func (s *Store) StoreScratchpad(ctx stdctx.Context, clusterID, sessionID string, data []byte) error {
	key := s.scratchpadKey(clusterID, sessionID)

	if s.config.Verbose {
		fmt.Printf("[S3Context] StoreScratchpad: key=%s, size=%d bytes\n", key, len(data))
	}

	// Compress if enabled
	if s.config.EnableGzip {
		compressed, err := compress(data)
		if err != nil {
			return fmt.Errorf("failed to compress: %w", err)
		}
		data = compressed
	}

	metadata := map[string]string{
		"session_id": sessionID,
		"cluster_id": clusterID,
		"size":       fmt.Sprintf("%d", len(data)),
		"date":       time.Now().Format(time.RFC3339),
	}

	contentType := "application/octet-stream"
	if s.config.EnableGzip {
		contentType = "application/gzip"
	}

	if err := s.client.PutObject(ctx, key, bytes.NewReader(data), contentType, metadata); err != nil {
		return fmt.Errorf("failed to upload scratchpad to S3: %w", err)
	}

	return nil
}

// GetScratchpad retrieves an archived scratchpad from S3.
func (s *Store) GetScratchpad(ctx stdctx.Context, clusterID, sessionID string) ([]byte, error) {
	key := s.scratchpadKey(clusterID, sessionID)

	if s.config.Verbose {
		fmt.Printf("[S3Context] GetScratchpad: key=%s\n", key)
	}

	exists, err := s.client.ObjectExists(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to check object existence: %w", err)
	}
	if !exists {
		return nil, nil
	}

	data, err := s.client.GetObject(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	// Decompress if gzip was used
	if s.config.EnableGzip {
		data, err = decompress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %w", err)
		}
	}

	return data, nil
}

// compress compresses data using gzip.
func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decompress decompresses gzip data.
func decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}

// --- Global Index Management ---

// indexKey returns the S3 key for the global session index.
func (s *Store) indexKey() string {
	return s.config.KeyPrefix + "metadata/index.json"
}

// UpdateGlobalIndex updates the global session index with a session location.
func (s *Store) UpdateGlobalIndex(ctx stdctx.Context, clusterID, sessionID string, location string) error {
	if s.config.Verbose {
		fmt.Printf("[S3Context] UpdateGlobalIndex: sessionID=%s, location=%s\n", sessionID, location)
	}

	// Read existing index
	indexData, err := s.client.GetObject(ctx, s.indexKey())
	if err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
		return fmt.Errorf("failed to read index: %w", err)
	}

	var index GlobalIndex
	if len(indexData) > 0 {
		if err := json.Unmarshal(indexData, &index); err != nil {
			return fmt.Errorf("failed to unmarshal index: %w", err)
		}
	}

	// Update index entry
	if index.Sessions == nil {
		index.Sessions = make(map[string]SessionLocation)
	}

	now := time.Now()
	index.Sessions[sessionID] = SessionLocation{
		ClusterID:     clusterID,
		LastHeartbeat: now.Format(time.RFC3339Nano),
		Location:      location,
		LastUpdated:   now.Format(time.RFC3339Nano),
	}

	// Serialize and upload
	indexData, err = json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	metadata := map[string]string{
		"updated_at": time.Now().Format(time.RFC3339),
	}

	if err := s.client.PutObject(ctx, s.indexKey(), bytes.NewReader(indexData), "application/json", metadata); err != nil {
		return fmt.Errorf("failed to upload index: %w", err)
	}

	return nil
}

// GetGlobalIndex retrieves the global session index.
func (s *Store) GetGlobalIndex(ctx stdctx.Context) (*GlobalIndex, error) {
	data, err := s.client.GetObject(ctx, s.indexKey())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			return &GlobalIndex{Sessions: make(map[string]SessionLocation)}, nil
		}
		return nil, fmt.Errorf("failed to get index: %w", err)
	}

	var index GlobalIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index: %w", err)
	}

	return &index, nil
}

// GlobalIndex tracks session locations across clusters.
type GlobalIndex struct {
	Version  string                      `json:"version"`
	Sessions map[string]SessionLocation `json:"sessions"`
}

// SessionLocation describes where a session is located.
type SessionLocation struct {
	ClusterID     string `json:"cluster_id"`
	LastHeartbeat string `json:"last_heartbeat"`
	Location      string `json:"location"` // e.g., "redis://cluster-1"
	LastUpdated   string `json:"last_updated"`
}

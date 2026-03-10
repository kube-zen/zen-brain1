// Package context provides factory functions for creating production-ready ZenContext.
package context

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/context/tier1"
	"github.com/kube-zen/zen-brain1/internal/context/tier2"
	"github.com/kube-zen/zen-brain1/internal/context/tier3"
	internalkb "github.com/kube-zen/zen-brain1/internal/qmd"
	"github.com/kube-zen/zen-brain1/internal/journal/receiptlog"
	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	qmdpkg "github.com/kube-zen/zen-brain1/pkg/qmd"
)

// ZenContextConfig holds configuration for creating a production ZenContext.
type ZenContextConfig struct {
	// Tier1 (Hot) Redis configuration.
	Tier1Redis *tier1.RedisConfig `json:"tier1_redis" yaml:"tier1_redis"`

	// Tier2 (Warm) QMD configuration.
	Tier2QMD *QMDConfig `json:"tier2_qmd" yaml:"tier2_qmd"`

	// Tier3 (Cold) S3 configuration.
	Tier3S3 *tier3.S3Config `json:"tier3_s3" yaml:"tier3_s3"`

	// Journal configuration (optional, for ReMe protocol).
	Journal *JournalConfig `json:"journal" yaml:"journal"`

	// Verbose enables verbose logging.
	Verbose bool `json:"verbose" yaml:"verbose"`

	// ClusterID is the cluster identifier for multi-cluster support.
	ClusterID string `json:"cluster_id" yaml:"cluster_id"`
}

// QMDConfig holds configuration for Tier 2 QMD knowledge store.
type QMDConfig struct {
	// RepoPath is the path to the zen-docs repository.
	RepoPath string `json:"repo_path" yaml:"repo_path"`

	// QMDBinaryPath is the path to the qmd binary (optional, uses PATH if empty).
	QMDBinaryPath string `json:"qmd_binary_path" yaml:"qmd_binary_path"`

	// Verbose enables verbose logging for QMD.
	Verbose bool `json:"verbose" yaml:"verbose"`
}

// JournalConfig holds configuration for ZenJournal integration.
type JournalConfig struct {
	// JournalPath is the path to the journal database (e.g., "journal.db").
	JournalPath string `json:"journal_path" yaml:"journal_path"`

	// EnableQueryIndex enables query indexing for faster searches.
	EnableQueryIndex bool `json:"enable_query_index" yaml:"enable_query_index"`
}

// DefaultZenContextConfig returns the default ZenContext configuration.
// Note: Paths are fallbacks; production should use config/env for explicit values.
func DefaultZenContextConfig() *ZenContextConfig {
	return &ZenContextConfig{
		Tier1Redis: tier1.DefaultRedisConfig(),
		Tier2QMD: &QMDConfig{
			RepoPath:      "./zen-docs",
			QMDBinaryPath: "",
			Verbose:       false,
		},
		Tier3S3: tier3.DefaultS3Config(),
		Journal: &JournalConfig{
			JournalPath:      "./journal",
			EnableQueryIndex: true,
		},
		Verbose:   false,
		ClusterID: "default",
	}
}

// NewZenContext creates a production-ready ZenContext with real Redis and S3 clients.
// This is the main factory function for production deployments.
func NewZenContext(config *ZenContextConfig) (zenctx.ZenContext, error) {
	if config == nil {
		config = DefaultZenContextConfig()
	}

	if config.Verbose {
		log.Printf("[ZenContextFactory] Creating ZenContext with cluster=%s", config.ClusterID)
	}

	// Build Tier 1 (Hot) Redis store
	hotStore, err := createTier1Store(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tier 1 store: %w", err)
	}

	// Build Tier 2 (Warm) QMD store (optional)
	var warmStore Store
	if config.Tier2QMD != nil && config.Tier2QMD.RepoPath != "" {
		warmStore, err = createTier2Store(config)
		if err != nil {
			if config.Verbose {
				log.Printf("[ZenContextFactory] Warning: Tier 2 store creation failed: %v", err)
			}
			// Continue without Tier 2 (knowledge queries will fail)
			warmStore = nil
		}
	}

	// Build Tier 3 (Cold) S3 store (optional)
	var coldStore Store
	if config.Tier3S3 != nil && config.Tier3S3.Bucket != "" {
		coldStore, err = createTier3Store(config)
		if err != nil {
			if config.Verbose {
				log.Printf("[ZenContextFactory] Warning: Tier 3 store creation failed: %v", err)
			}
			// Continue without Tier 3 (archival will fail)
			coldStore = nil
		}
	}

	// Build Journal adapter (optional)
	var journalAdapter Journal
	if config.Journal != nil && config.Journal.JournalPath != "" {
		journalAdapter, err = createJournalAdapter(config)
		if err != nil && config.Verbose {
			log.Printf("[ZenContextFactory] Warning: Journal adapter creation failed: %v", err)
		}
	}

	// Build composite ZenContext
	compositeConfig := &Config{
		Hot:     hotStore,
		Warm:    warmStore,
		Cold:    coldStore,
		Journal: journalAdapter,
		Verbose: config.Verbose,
	}

	composite, err := NewComposite(compositeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create composite ZenContext: %w", err)
	}

	if config.Verbose {
		log.Printf("[ZenContextFactory] ZenContext created successfully")
		log.Printf("[ZenContextFactory] Tier 1: %v", hotStore != nil)
		log.Printf("[ZenContextFactory] Tier 2: %v", warmStore != nil)
		log.Printf("[ZenContextFactory] Tier 3: %v", coldStore != nil)
		log.Printf("[ZenContextFactory] Journal: %v", journalAdapter != nil)
	}

	return composite, nil
}

// NewMinimalZenContext creates a ZenContext with only Tier 1 (Redis). Used when full tier stack is not needed (e.g. Foreman with ReMe).
func NewMinimalZenContext(redisURL, clusterID string) (zenctx.ZenContext, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis URL is required")
	}
	if clusterID == "" {
		clusterID = "default"
	}
	config := &ZenContextConfig{
		Tier1Redis: &tier1.RedisConfig{URL: redisURL},
		ClusterID:  clusterID,
	}
	hotStore, err := createTier1Store(config)
	if err != nil {
		return nil, err
	}
	return NewComposite(&Config{Hot: hotStore})
}

// createTier1Store creates the Redis-based Tier 1 store.
func createTier1Store(config *ZenContextConfig) (Store, error) {
	if config.Tier1Redis == nil {
		return nil, fmt.Errorf("Tier 1 Redis configuration is required")
	}

	if config.Verbose {
		log.Printf("[ZenContextFactory] Creating Tier 1 Redis store: addr=%s, db=%d",
			config.Tier1Redis.Addr, config.Tier1Redis.DB)
	}

	// Create Redis client
	redisClient, err := tier1.NewGoRedisClient(config.Tier1Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client: %w", err)
	}

	// Create Redis store
	redisStoreConfig := &tier1.Config{
		RedisClient: redisClient,
		KeyPrefix:   "zen:ctx",
		DefaultTTL:  30 * time.Minute,
		LockTimeout: 5 * time.Second,
		ClusterID:   config.ClusterID,
		Verbose:     config.Verbose,
	}

	store, err := tier1.NewStore(redisStoreConfig)
	if err != nil {
		redisClient.Close()
		return nil, fmt.Errorf("failed to create Redis store: %w", err)
	}

	return store, nil
}

// createTier2Store creates the QMD-based Tier 2 store.
func createTier2Store(config *ZenContextConfig) (Store, error) {
	if config.Tier2QMD == nil {
		return nil, fmt.Errorf("Tier 2 QMD configuration is required")
	}

	if config.Verbose {
		log.Printf("[ZenContextFactory] Creating Tier 2 QMD store: repo=%s",
			config.Tier2QMD.RepoPath)
	}

	// Create qmd client
	var qmdClient qmdpkg.Client
	qmdConfig := &internalkb.Config{
		QMDPath:               config.Tier2QMD.QMDBinaryPath,
		Timeout:               30 * time.Second,
		Verbose:               config.Tier2QMD.Verbose,
		FallbackToMock:        false, // Require real qmd client (npx @tobilu/qmd)
		SkipAvailabilityCheck: false,
	}
	client, err := internalkb.NewClient(qmdConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create qmd client: %w", err)
	}
	qmdClient = client

	// Create KB store
	kbStoreConfig := &internalkb.KBStoreConfig{
		QMDClient: qmdClient,
		RepoPath:  config.Tier2QMD.RepoPath,
		Verbose:   config.Tier2QMD.Verbose,
	}

	kbStore, err := internalkb.NewKBStore(kbStoreConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create KB store: %w", err)
	}

	// Create QMD store
	qmdStoreConfig := &tier2.Config{
		KBStore: kbStore,
		Verbose: config.Tier2QMD.Verbose,
	}

	store, err := tier2.NewQMDStore(qmdStoreConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create QMD store: %w", err)
	}

	return store, nil
}

// createTier3Store creates the S3-based Tier 3 store.
func createTier3Store(config *ZenContextConfig) (Store, error) {
	if config.Tier3S3 == nil {
		return nil, fmt.Errorf("Tier 3 S3 configuration is required")
	}

	if config.Verbose {
		log.Printf("[ZenContextFactory] Creating Tier 3 S3 store: bucket=%s, region=%s",
			config.Tier3S3.Bucket, config.Tier3S3.Region)
	}

	// Create S3 client
	s3Client, err := tier3.NewS3Client(config.Tier3S3)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Create S3 store
	s3StoreConfig := &tier3.Config{
		S3Client:      s3Client,
		Bucket:        config.Tier3S3.Bucket,
		KeyPrefix:     "",
		ClusterID:     config.ClusterID,
		EnableGzip:    true,
		RetentionDays: 90,
		Verbose:       config.Tier3S3.Verbose,
	}

	store, err := tier3.NewStore(s3StoreConfig)
	if err != nil {
		s3Client.Close()
		return nil, fmt.Errorf("failed to create S3 store: %w", err)
	}

	return store, nil
}

// createJournalAdapter creates the Journal adapter for ReMe protocol.
func createJournalAdapter(config *ZenContextConfig) (Journal, error) {
	if config.Journal == nil {
		return nil, fmt.Errorf("journal configuration is required")
	}

	if config.Verbose {
		log.Printf("[ZenContextFactory] Creating Journal adapter: path=%s",
			config.Journal.JournalPath)
	}

	// Ensure spool directory exists
	spoolDir := config.Journal.JournalPath
	if err := os.MkdirAll(spoolDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create journal spool directory %s: %w", spoolDir, err)
	}

	// Create receiptlog journal configuration
	receiptlogConfig := &receiptlog.Config{
		SpoolDir:      spoolDir,
		SpoolSize:     100 * 1024 * 1024, // 100MB
		RetentionDays: 7,
		// S3 archival optional - can be added later via config
	}

	// Create the ZenJournal implementation
	zenJournal, err := receiptlog.New(receiptlogConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create receiptlog journal: %w", err)
	}

	// Create adapter that conforms to composite.Journal interface
	adapter := NewJournalAdapter(zenJournal, config.Verbose)

	if config.Verbose {
		log.Printf("[ZenContextFactory] Journal adapter created successfully")
	}

	return adapter, nil
}

// MustCreateZenContext creates a ZenContext or panics on error.
// Use only in tests or where panic is acceptable.
func MustCreateZenContext(config *ZenContextConfig) zenctx.ZenContext {
	zc, err := NewZenContext(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to create ZenContext: %v", err))
	}
	return zc
}

// CreateMockZenContext creates a mock ZenContext for testing.
// This uses in-memory stores instead of real Redis/S3.
func CreateMockZenContext() zenctx.ZenContext {
	// Use the existing mock from tests (simplified)
	// For now, create a minimal composite with nil stores
	// In practice, you'd use the mock implementations from tests
	config := &Config{
		Hot:     nil, // Would need mock
		Warm:    nil,
		Cold:    nil,
		Journal: nil,
		Verbose: false,
	}
	composite, err := NewComposite(config)
	if err != nil {
		panic(err)
	}
	return composite
}

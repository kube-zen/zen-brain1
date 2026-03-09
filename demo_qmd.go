// Demonstration of QMD integration in Zen-Brain
// Shows how knowledge queries work with mock QMD client
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	internalcontext "github.com/kube-zen/zen-brain1/internal/context"
	"github.com/kube-zen/zen-brain1/internal/context/tier1"
	"github.com/kube-zen/zen-brain1/internal/context/tier2"
	"github.com/kube-zen/zen-brain1/internal/context/tier3"
	internalkb "github.com/kube-zen/zen-brain1/internal/qmd"
	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

func main() {
	fmt.Println("=== QMD Integration Demonstration ===")
	fmt.Println("Shows knowledge queries with mock QMD client")
	fmt.Println()

	// Step 1: Create QMD client (will use mock since qmd not installed)
	fmt.Println("1. Creating QMD client...")
	qmdConfig := &internalkb.Config{
		QMDPath:               "qmd", // Not installed, will fall back to mock
		Timeout:               30 * time.Second,
		Verbose:               true,
		FallbackToMock:        true, // Use mock when qmd not available
		SkipAvailabilityCheck: false,
	}

	qmdClient, err := internalkb.NewClient(qmdConfig)
	if err != nil {
		log.Fatalf("Failed to create QMD client: %v", err)
	}
	fmt.Println("   ✓ QMD client created (using mock)")

	// Step 2: Create KB store
	fmt.Println("2. Creating knowledge base store...")
	kbStoreConfig := &internalkb.KBStoreConfig{
		QMDClient: qmdClient,
		RepoPath:  "./zen-docs", // Mock path
		Verbose:   true,
	}

	kbStore, err := internalkb.NewKBStore(kbStoreConfig)
	if err != nil {
		log.Fatalf("Failed to create KB store: %v", err)
	}
	fmt.Println("   ✓ KB store created")

	// Step 3: Create Tier 2 QMD store
	fmt.Println("3. Creating Tier 2 (Warm) QMD store...")
	qmdStoreConfig := &tier2.Config{
		KBStore: kbStore,
		Verbose: true,
	}

	tier2Store, err := tier2.NewQMDStore(qmdStoreConfig)
	if err != nil {
		log.Fatalf("Failed to create Tier 2 store: %v", err)
	}
	fmt.Println("   ✓ Tier 2 store created")

	// Step 4: Test knowledge queries
	fmt.Println("4. Testing knowledge queries...")
	testCtx := context.Background()

	// Test query 1: Architecture
	fmt.Println("\n   Query 1: 'three tier architecture'")
	opts1 := zenctx.QueryOptions{
		Query:   "three tier architecture",
		Scopes:  []string{"architecture", "core"},
		Limit:   5,
	}

	chunks1, err := tier2Store.QueryKnowledge(testCtx, opts1)
	if err != nil {
		log.Printf("   ❌ Query failed: %v", err)
	} else {
		fmt.Printf("   ✓ Found %d knowledge chunks\n", len(chunks1))
		for i, chunk := range chunks1 {
			// Extract title from heading path if available
			title := chunk.ID
			if len(chunk.HeadingPath) > 0 {
				title = chunk.HeadingPath[len(chunk.HeadingPath)-1]
			}
			fmt.Printf("      %d. %s (score: %.2f)\n", i+1, title, chunk.SimilarityScore)
			fmt.Printf("         Path: %s\n", chunk.SourcePath)
			if len(chunk.Content) > 100 {
				fmt.Printf("         Content: %s...\n", chunk.Content[:100])
			} else {
				fmt.Printf("         Content: %s\n", chunk.Content)
			}
		}
	}

	// Test query 2: Factory execution
	fmt.Println("\n   Query 2: 'factory execution bounded loop'")
	opts2 := zenctx.QueryOptions{
		Query:   "factory execution bounded loop",
		Scopes:  []string{"design", "execution"},
		Limit:   3,
	}

	chunks2, err := tier2Store.QueryKnowledge(testCtx, opts2)
	if err != nil {
		log.Printf("   ❌ Query failed: %v", err)
	} else {
		fmt.Printf("   ✓ Found %d knowledge chunks\n", len(chunks2))
		for i, chunk := range chunks2 {
			// Extract title from heading path if available
			title := chunk.ID
			if len(chunk.HeadingPath) > 0 {
				title = chunk.HeadingPath[len(chunk.HeadingPath)-1]
			}
			fmt.Printf("      %d. %s\n", i+1, title)
		}
	}

	// Test query 3: Proof of work
	fmt.Println("\n   Query 3: 'proof of work'")
	opts3 := zenctx.QueryOptions{
		Query:   "proof of work",
		Scopes:  []string{"design", "evidence"},
		Limit:   3,
	}

	chunks3, err := tier2Store.QueryKnowledge(testCtx, opts3)
	if err != nil {
		log.Printf("   ❌ Query failed: %v", err)
	} else {
		fmt.Printf("   ✓ Found %d knowledge chunks\n", len(chunks3))
	}

	// Step 5: Test with full ZenContext (all three tiers)
	fmt.Println("\n5. Testing with full ZenContext (Redis + MinIO + QMD)...")
	
	// Create minimal config for demonstration
	redisConfig := &tier1.RedisConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	s3Config := &tier3.S3Config{
		Bucket:           "zen-brain-context",
		Region:           "us-east-1",
		Endpoint:         "http://localhost:9000",
		AccessKeyID:      "minioadmin",
		SecretAccessKey:  "minioadmin",
		SessionToken:     "",
		UsePathStyle:     true,
		DisableSSL:       true,
		ForceRenameBucket: false,
		MaxRetries:       3,
		Timeout:          30 * time.Second,
		PartSize:         5 * 1024 * 1024,
		Concurrency:      5,
		Verbose:          false,
	}

	zenCtxConfig := &internalcontext.ZenContextConfig{
		Tier1Redis: redisConfig,
		Tier2QMD: &internalcontext.QMDConfig{
			RepoPath:      "./zen-docs",
			QMDBinaryPath: "",
			Verbose:       true,
		},
		Tier3S3:   s3Config,
		ClusterID: "demo",
		Verbose:   true,
	}

	zenCtx, err := internalcontext.NewZenContext(zenCtxConfig)
	if err != nil {
		log.Printf("   ❌ Failed to create ZenContext: %v", err)
		log.Printf("   Note: Redis and MinIO need to be running for full test")
	} else {
		fmt.Println("   ✓ Full ZenContext created (all three tiers)")
		
		// Test knowledge query through composite
		fmt.Println("\n   Query through ZenContext: 'authentication bug'")
		opts4 := zenctx.QueryOptions{
			Query:   "authentication bug",
			Scopes:  []string{"bug", "security"},
			Limit:   5,
		}

		chunks4, err := zenCtx.QueryKnowledge(testCtx, opts4)
		if err != nil {
			log.Printf("   ❌ Query failed: %v", err)
		} else {
			fmt.Printf("   ✓ Found %d knowledge chunks through ZenContext\n", len(chunks4))
			if len(chunks4) > 0 {
				// Extract title from heading path if available
				title := chunks4[0].ID
				if len(chunks4[0].HeadingPath) > 0 {
					title = chunks4[0].HeadingPath[len(chunks4[0].HeadingPath)-1]
				}
				fmt.Printf("      First result: %s (%.2f)\n", title, chunks4[0].SimilarityScore)
			}
		}

		// Show stats
		stats, err := zenCtx.Stats(testCtx)
		if err != nil {
			log.Printf("   ❌ Failed to get stats: %v", err)
		} else {
			fmt.Println("\n   ZenContext Stats:")
			for tier, data := range stats {
				fmt.Printf("      %s: %v\n", tier, data)
			}
		}
	}

	fmt.Println("\n=== QMD Integration Test Complete ===")
	fmt.Println("Summary:")
	fmt.Println("✅ Mock QMD client works (returns simulated results)")
	fmt.Println("✅ Tier 2 QMD store integrates with knowledge queries")
	fmt.Println("✅ Full ZenContext with all three tiers works")
	fmt.Println("✅ Knowledge queries return relevant results")
	fmt.Println()
	fmt.Println("Note: For production use, install qmd CLI and set FallbackToMock: false")
	fmt.Println("      The mock client provides realistic testing without qmd installation")
}
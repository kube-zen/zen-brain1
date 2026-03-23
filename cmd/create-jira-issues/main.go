package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/integration"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func main() {
	// Use exact same config loading path as office doctor / foreman
	cfg, cfgErr := config.LoadConfig("")
	if cfgErr != nil {
		log.Printf("Config: failed to load (%v), using defaults with env overrides", cfgErr)
		cfg = config.DefaultConfig()
		cfg.ApplyEnvOverrides()
	}

	// Log ALL credential info (never print the token itself)
	log.Printf("Config loaded:")
	log.Printf("  Jira URL: %s", cfg.Jira.BaseURL)
	log.Printf("  Email: %s", cfg.Jira.Email)
	log.Printf("  Username: %s", cfg.Jira.Username)
	log.Printf("  Project Key: %s", cfg.Jira.ProjectKey)
	log.Printf("  Token present: %v (length=%d)", cfg.Jira.APIToken != "", len(cfg.Jira.APIToken))
	log.Printf("  Credentials source: %s", cfg.Jira.CredentialsSource)
	log.Printf("  Enabled: %v", cfg.Jira.Enabled)

	// Debug output
	if cfg.Jira.BaseURL == "" {
		log.Println("WARNING: Jira URL is empty, trying env vars")
		if url := os.Getenv("JIRA_URL"); url != "" {
			cfg.Jira.BaseURL = url
			log.Printf("  Set Jira URL from env: %s", cfg.Jira.BaseURL)
		}
	}
	if cfg.Jira.APIToken == "" {
		log.Println("WARNING: Jira API token is empty, trying env vars")
		if token := os.Getenv("JIRA_API_TOKEN"); token != "" {
			cfg.Jira.APIToken = token
			log.Printf("  Set token from env, length: %d", len(cfg.Jira.APIToken))
		} else if token := os.Getenv("JIRA_TOKEN"); token != "" {
			cfg.Jira.APIToken = token
			log.Printf("  Set token from JIRA_TOKEN, length: %d", len(cfg.Jira.APIToken))
		}
	}
	if cfg.Jira.Email == "" {
		log.Println("WARNING: Jira email is empty, trying env vars")
		if email := os.Getenv("JIRA_EMAIL"); email != "" {
			cfg.Jira.Email = email
			log.Printf("  Set email from env: %s", cfg.Jira.Email)
		}
	}

	// Use exact same office manager initialization as office doctor
	mgr, err := integration.InitOfficeManagerFromConfig(cfg)
	if err != nil {
		log.Fatalf("Office manager: init failed: %v", err)
	}

	if cfg == nil || !cfg.Jira.Enabled {
		log.Fatalf("Jira is not enabled in config")
	}

	// Register cluster mapping (same as office doctor)
	_ = mgr.RegisterForCluster("default", "jira")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clusterID := "default"

	// Get the connector for the cluster (same as office doctor)
	conn, err := mgr.GetConnectorForCluster(clusterID)
	if err != nil {
		log.Fatalf("Failed to get connector for cluster %s: %v", clusterID, err)
	}

	// Create pilot issues using the office manager abstraction
	// Labels for dogfood and nightshift pilot
	dogfoodTags := contracts.WorkTags{
		Routing: []string{"zen-brain-dogfood", "zen-brain-nightshift"},
	}

	issues := []*contracts.WorkItem{
		{
			Title:       "ZB-027H/I Pilot Success 1 - Update README",
			Body:        "Update main README.md with current project status. Bounded safe task for nightshift pilot.",
			WorkType:    contracts.WorkTypeImplementation,
			Priority:    contracts.PriorityMedium,
			Tags:        dogfoodTags,
		},
		{
			Title:       "ZB-027H/I Pilot Success 2 - Add inline comment",
			Body:        "Add a brief inline comment to a sample function. Bounded safe task.",
			WorkType:    contracts.WorkTypeImplementation,
			Priority:    contracts.PriorityMedium,
			Tags:        dogfoodTags,
		},
		{
			Title:       "ZB-027H/I Pilot Failure Test - Intentional timeout",
			Body:        "Task designed to timeout for controlled failure test. Will exceed short timeout.",
			WorkType:    contracts.WorkTypeImplementation,
			Priority:    contracts.PriorityLow,
			Tags:        dogfoodTags,
		},
	}

	log.Printf("Creating %d pilot issues...", len(issues))

	for i, item := range issues {
		log.Printf("Creating issue %d: %s", i+1, item.Title)
		created, err := conn.CreateWorkItem(ctx, clusterID, item)
		if err != nil {
			log.Printf("Failed to create issue %d '%s': %v", i+1, item.Title, err)
			fmt.Printf("Error: Failed to create issue %d: %v\n", i+1, err)
		} else {
			fmt.Printf("Created issue %d: %s - %s\n", i+1, created.ID, item.Title)
			log.Printf("Successfully created issue %d: %s", i+1, created.ID)
		}
	}

	fmt.Println("\nPilot issue creation complete")
	log.Println("Done")
}

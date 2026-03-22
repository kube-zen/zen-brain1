package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	
	"github.com/kube-zen/zen-brain1/internal/office/jira"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func main() {
	secretDir := "/zen-lock/secrets"
	if _, err := os.Stat(secretDir); os.IsNotExist(err) {
		secretDir = os.ExpandEnv("$HOME/.zen-brain/secrets")
	}
	
	apiToken := strings.TrimSpace(string(mustReadFile(secretDir + "/JIRA_API_TOKEN")))
	email := strings.TrimSpace(string(mustReadFile(secretDir + "/JIRA_EMAIL")))
	url := strings.TrimSpace(string(mustReadFile(secretDir + "/JIRA_URL")))
	projectKey := strings.TrimSpace(string(mustReadFile(secretDir + "/JIRA_PROJECT_KEY")))
	if projectKey == "" {
		projectKey = "ZB"
	}
	
	config := &jira.Config{
		BaseURL:    url,
		Email:      email,
		APIToken:   apiToken,
		ProjectKey: projectKey,
	}
	
	jiraConn, err := jira.New("zen-brain-cli", "default", config)
	if err != nil {
		log.Fatalf("Failed to create Jira connector: %v", err)
	}
	
	ctx := context.Background()
	clusterID := "default"
	
	success1 := &contracts.WorkItem{
		Title:       "ZB-027H Success Test 1 - Create README documentation",
		Body:        "Create a simple README.md file. Bounded safe task.",
		WorkType:    contracts.WorkTypeImplementation,
		Priority:    contracts.PriorityMedium,
	}
	
	if created, err := jiraConn.CreateWorkItem(ctx, clusterID, success1); err != nil {
		log.Printf("Failed to create success task 1: %v", err)
	} else {
		fmt.Printf("Created success task 1: %s\n", created.ID)
	}
	
	success2 := &contracts.WorkItem{
		Title:       "ZB-027H Success Test 2 - Add inline code comment",
		Body:        "Add a brief inline comment. Bounded safe task.",
		WorkType:    contracts.WorkTypeImplementation,
		Priority:    contracts.PriorityMedium,
	}
	
	if created, err := jiraConn.CreateWorkItem(ctx, clusterID, success2); err != nil {
		log.Printf("Failed to create success task 2: %v", err)
	} else {
		fmt.Printf("Created success task 2: %s\n", created.ID)
	}
	
	failure := &contracts.WorkItem{
		Title:       "ZB-027H Failure Test - Intentionally impossible task",
		Body:        "Attempt to delete production database without authorization. Designed to fail.",
		WorkType:    contracts.WorkTypeImplementation,
		Priority:    contracts.PriorityLow,
	}
	
	if created, err := jiraConn.CreateWorkItem(ctx, clusterID, failure); err != nil {
		log.Printf("Failed to create failure task: %v", err)
	} else {
		fmt.Printf("Created failure task: %s\n", created.ID)
	}
	
	fmt.Println("\nJira issue creation complete")
}

func mustReadFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", path, err)
	}
	return data
}

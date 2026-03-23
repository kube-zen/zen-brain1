// Package jira provides Jira issue creation for zen-brain office integration.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// CreateWorkItem creates a new Jira issue and returns it as a WorkItem.
func (j *JiraOffice) CreateWorkItem(ctx context.Context, clusterID string, item *contracts.WorkItem) (*contracts.WorkItem, error) {
	// Validate required fields
	if item.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	if j.config.ProjectKey == "" {
		return nil, fmt.Errorf("project_key is required")
	}

	// Map priority to Jira priority name
	priorityName := "Medium" // default
	switch item.Priority {
	case contracts.PriorityCritical:
		priorityName = "Highest"
	case contracts.PriorityHigh:
		priorityName = "High"
	case contracts.PriorityMedium:
		priorityName = "Medium"
	case contracts.PriorityLow:
		priorityName = "Low"
	case contracts.PriorityBackground:
		priorityName = "Lowest"
	}

	// Build Jira create payload
	jiraIssue := JiraIssueCreate{
		Fields: IssueFields{
			Project: Project{Key: j.config.ProjectKey},
			Summary: item.Title,
			Description: IssueDescription{
				Type:  "doc",
				Version: 1,
				Content: []Content{
					{
						Type: "paragraph",
						Content: []Content{
							{
								Type: "text",
								Text: item.Body,
							},
						},
					},
				},
			},
			IssueType: IssueType{
				Name: "Task", // Always use "Task" issue type for creation
			},
			Priority: Priority{
				Name: priorityName,
			},
		},
	}

	// Map tags to labels (use Routing tags as Jira labels)
	if len(item.Tags.Routing) > 0 {
		jiraIssue.Fields.Labels = make([]string, len(item.Tags.Routing))
		copy(jiraIssue.Fields.Labels, item.Tags.Routing)
	}

	// Serialize to JSON
	bodyBytes, err := json.Marshal(jiraIssue)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize Jira issue: %w", err)
	}

	// Issue creation endpoint
	path := "/rest/api/3/issue"

	resp, err := j.jiraRequest(ctx, "POST", path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira issue: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create Jira issue (status %d): %s", resp.StatusCode, string(responseBody))
	}

	// Decode created issue
	var created JiraIssue
	if err := json.Unmarshal(responseBody, &created); err != nil {
		return nil, fmt.Errorf("failed to decode created issue: %w", err)
	}

	// Extract custom fields (empty for new issue)
	customFields := make(map[string]interface{})

	// Convert to WorkItem
	workItem := j.convertToWorkItem(&created, customFields)

	// Add metadata for ingestion
	workItem.ClusterID = clusterID
	workItem.ProjectID = j.config.ProjectKey

	return workItem, nil
}

// JiraIssueCreate represents the payload for creating a Jira issue via REST API.
type JiraIssueCreate struct {
	Fields IssueFields `json:"fields"`
}

// IssueFields represents the fields section of a Jira issue.
type IssueFields struct {
	Project   Project           `json:"project"`
	Summary   string            `json:"summary"`
	Description IssueDescription `json:"description"`
	IssueType IssueType         `json:"issuetype"`
	Priority   Priority           `json:"priority"`
	Labels     []string           `json:"labels,omitempty"`
}

// Project represents a Jira project.
type Project struct {
	Key string `json:"key"`
}

// IssueType represents a Jira issue type.
type IssueType struct {
	Name string `json:"name"`
}

// Priority represents a Jira priority.
type Priority struct {
	Name string `json:"name"`
}

// IssueDescription represents the ADF-formatted description.
type IssueDescription struct {
	Type    string    `json:"type"`
	Version int       `json:"version"`
	Content []Content `json:"content"`
}

// Content represents ADF content.
type Content struct {
	Type    string    `json:"type"`
	Text    string    `json:"text,omitempty"`
	Content []Content `json:"content,omitempty"`
}

// Package jira provides a Jira connector that implements the ZenOffice interface.
// This connector integrates with Atlassian Jira, mapping Jira issues to canonical
// WorkItem types and injecting AI attribution headers as required by V6.
package jira

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	pkgoffice "github.com/kube-zen/zen-brain1/pkg/office"
)

// Config holds Jira connection configuration.
type Config struct {
	BaseURL            string            `yaml:"base_url" json:"base_url"`
	Email              string            `yaml:"email" json:"email"`
	APIToken           string            `yaml:"api_token" json:"api_token"`
	ProjectKey         string            `yaml:"project_key" json:"project_key"`
	FieldMappings      map[string]string `yaml:"field_mappings" json:"field_mappings"`
	StatusMapping      map[string]string `yaml:"status_mapping" json:"status_mapping"`
	WorkTypeMapping    map[string]string `yaml:"worktype_mapping" json:"worktype_mapping"`
	PriorityMapping    map[string]string `yaml:"priority_mapping" json:"priority_mapping"`
	CustomFieldMapping map[string]string `yaml:"custom_field_mapping" json:"custom_field_mapping"`
	WebhookURL         string            `yaml:"webhook_url" json:"webhook_url"`
	WebhookSecret      string            `yaml:"webhook_secret" json:"webhook_secret"`
	WebhookPort        int               `yaml:"webhook_port" json:"webhook_port"`
	WebhookPath        string            `yaml:"webhook_path" json:"webhook_path"`
}

// JiraOffice implements the ZenOffice interface for Atlassian Jira.
type JiraOffice struct {
	*office.BaseOffice
	config *Config
	client *http.Client

	// webhook server fields
	mu         sync.RWMutex
	server     *http.Server
	eventChan  chan pkgoffice.WorkItemEvent
	serverDone chan struct{}
}

// New creates a new JiraOffice connector.
func New(name, clusterID string, config *Config) (*JiraOffice, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base_url is required")
	}

	if config.APIToken == "" {
		return nil, fmt.Errorf("api_token is required")
	}

	// Normalize base URL
	config.BaseURL = strings.TrimSuffix(config.BaseURL, "/")

	// Set default email if not provided
	if config.Email == "" {
		config.Email = "zen-brain@automation.local"
	}
	// Set default status mapping if not provided
	if config.StatusMapping == nil {
		config.StatusMapping = map[string]string{
			"To Do":       "requested",
			"Backlog":     "requested",
			"Selected":    "requested",
			"In Progress": "running",
			"Done":        "completed",
			"Closed":      "completed",
			"Resolved":    "completed",
			"Reopened":    "blocked",
			"Blocked":     "blocked",
			"On Hold":     "blocked",
			"Paused":      "blocked",
		}
	}
	// Set default worktype mapping if not provided
	if config.WorkTypeMapping == nil {
		config.WorkTypeMapping = map[string]string{
			"Bug":           "debug",
			"Defect":        "debug",
			"Task":          "implementation",
			"Chore":         "implementation",
			"Story":         "design",
			"Feature":       "design",
			"Epic":          "research",
			"Initiative":    "research",
			"Spike":         "research",
			"Investigation": "analysis",
			"Documentation": "documentation",
			"Refactor":      "refactor",
			"Security":      "security",
			"Test":          "testing",
			"Operation":     "operations",
			"Ops":           "operations",
		}
	}
	// Set default priority mapping if not provided
	if config.PriorityMapping == nil {
		config.PriorityMapping = map[string]string{
			"Highest":  "critical",
			"Critical": "critical",
			"High":     "high",
			"Medium":   "medium",
			"Low":      "low",
			"Lowest":   "background",
		}
	}
	// Initialize custom field mapping if nil
	if config.CustomFieldMapping == nil {
		config.CustomFieldMapping = make(map[string]string)
	}
	// Set default webhook port if not provided
	if config.WebhookPort == 0 {
		config.WebhookPort = 8080
	}
	// Set default webhook path if not provided
	if config.WebhookPath == "" {
		config.WebhookPath = "/webhook"
	}

	base := office.NewBaseOffice(name, clusterID, nil) // No extra config needed

	return &JiraOffice{
		BaseOffice: base,
		config:     config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// NewFromEnv creates a JiraOffice from environment variables.
func NewFromEnv(name, clusterID string) (*JiraOffice, error) {
	config := &Config{
		BaseURL:    os.Getenv("JIRA_URL"),
		APIToken:   os.Getenv("JIRA_TOKEN"),
		Email:      os.Getenv("JIRA_EMAIL"),
		ProjectKey: os.Getenv("JIRA_PROJECT_KEY"),
	}

	return New(name, clusterID, config)
}

// jiraRequest makes an authenticated request to the Jira API.
func (j *JiraOffice) jiraRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := j.config.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(j.config.Email, j.config.APIToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return j.client.Do(req)
}

// formatAIAttribution formats an AI attribution header according to V6 spec.

// injectAIAttribution injects AI attribution into comment body.

// extractJiraKey extracts Jira issue key from sourceKey or workItemID.
func extractJiraKey(sourceKey string) string {
	// If it already looks like a Jira key (PROJ-123), return it
	if strings.Contains(sourceKey, "-") && !strings.Contains(sourceKey, "/") {
		return sourceKey
	}

	// Otherwise, assume it's a WorkItem ID and we need to map it
	// For now, return as-is - actual implementation would need a mapping
	return sourceKey
}

// extractCustomFields extracts custom fields from a Jira fields map.
func (j *JiraOffice) extractCustomFields(fields map[string]interface{}) map[string]interface{} {
	custom := make(map[string]interface{})
	for key, value := range fields {
		if strings.HasPrefix(key, "customfield_") {
			custom[key] = value
		}
	}
	return custom
}

// Fetch retrieves a work item by ID.
func (j *JiraOffice) Fetch(ctx context.Context, clusterID, workItemID string) (*contracts.WorkItem, error) {
	jiraKey := extractJiraKey(workItemID)
	return j.fetchJiraIssue(ctx, jiraKey)
}

// FetchBySourceKey retrieves a work item by its source system key (e.g., "PROJ-123").
func (j *JiraOffice) FetchBySourceKey(ctx context.Context, clusterID, sourceKey string) (*contracts.WorkItem, error) {
	return j.fetchJiraIssue(ctx, sourceKey)
}

// fetchJiraIssue fetches a Jira issue and converts it to a WorkItem.
func (j *JiraOffice) fetchJiraIssue(ctx context.Context, jiraKey string) (*contracts.WorkItem, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s", url.PathEscape(jiraKey))
	resp, err := j.jiraRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Jira issue: %w", err)
	}
	defer resp.Body.Close()

	// Read the entire response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Jira issue (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode into map to extract custom fields
	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode raw JSON: %w", err)
	}

	// Extract fields map
	fields, ok := raw["fields"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing fields in Jira response")
	}

	// Extract custom fields
	customFields := j.extractCustomFields(fields)

	// Decode into JiraIssue struct (for known fields)
	var issue JiraIssue
	if err := json.Unmarshal(bodyBytes, &issue); err != nil {
		return nil, fmt.Errorf("failed to decode Jira issue: %w", err)
	}

	return j.convertToWorkItem(&issue, customFields), nil
}

// UpdateStatus updates the status of a work item.
func (j *JiraOffice) UpdateStatus(ctx context.Context, clusterID, workItemID string, status contracts.WorkStatus) error {
	jiraKey := extractJiraKey(workItemID)

	// First, get available transitions for this issue
	transitions, err := j.getTransitions(jiraKey)
	if err != nil {
		return fmt.Errorf("failed to get transitions: %w", err)
	}

	// Map canonical status to Jira transition
	transitionID, err := j.findTransition(transitions, status)
	if err != nil {
		return fmt.Errorf("no suitable transition found for status %s: %w", status, err)
	}

	// Execute the transition
	return j.executeTransition(jiraKey, transitionID)
}

// getTransitions fetches available transitions for a Jira issue.
func (j *JiraOffice) getTransitions(jiraKey string) ([]JiraTransition, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s/transitions", url.PathEscape(jiraKey))
	resp, err := j.jiraRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get transitions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get transitions (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Transitions []JiraTransition `json:"transitions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode transitions: %w", err)
	}

	return result.Transitions, nil
}

// findTransition finds a Jira transition that matches the target canonical status.
func (j *JiraOffice) findTransition(transitions []JiraTransition, targetStatus contracts.WorkStatus) (string, error) {
	// Simple mapping - in production, this should be configurable
	statusToTransition := map[contracts.WorkStatus][]string{
		contracts.StatusRunning:   {"Start Progress", "In Progress", "Reopen"},
		contracts.StatusCompleted: {"Done", "Close", "Resolve"},
		contracts.StatusBlocked:   {"Block", "Hold"},
		contracts.StatusFailed:    {"Fail"},
		contracts.StatusCanceled:  {"Cancel"},
	}

	targetNames, ok := statusToTransition[targetStatus]
	if !ok {
		return "", fmt.Errorf("no transition mapping for status %s", targetStatus)
	}

	for _, transition := range transitions {
		for _, targetName := range targetNames {
			if strings.EqualFold(transition.Name, targetName) {
				return transition.ID, nil
			}
		}
	}

	return "", fmt.Errorf("no matching transition found for status %s", targetStatus)
}

// executeTransition executes a Jira transition.
func (j *JiraOffice) executeTransition(jiraKey, transitionID string) error {
	path := fmt.Sprintf("/rest/api/3/issue/%s/transitions", url.PathEscape(jiraKey))

	payload := map[string]interface{}{
		"transition": map[string]interface{}{
			"id": transitionID,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal transition payload: %w", err)
	}

	resp, err := j.jiraRequest("POST", path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to execute transition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to execute transition (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// AddComment adds a comment to a work item with AI attribution.
func (j *JiraOffice) AddComment(ctx context.Context, clusterID, workItemID string, comment *contracts.Comment) error {
	jiraKey := extractJiraKey(workItemID)

	// Inject AI attribution if present
	body := comment.Body
	if comment.Attribution != nil {
		header := j.FormatAIAttributionHeader(comment.Attribution)
		body = header + "\n\n" + body
	}

	// Prepare Jira comment payload (Atlassian Document Format)
	payload := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []interface{}{
				map[string]interface{}{
					"type": "paragraph",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": body,
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal comment payload: %w", err)
	}

	path := fmt.Sprintf("/rest/api/3/issue/%s/comment", url.PathEscape(jiraKey))
	resp, err := j.jiraRequest("POST", path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add comment (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// AddAttachment attaches evidence to a work item.
func (j *JiraOffice) AddAttachment(ctx context.Context, clusterID, workItemID string, attachment *contracts.Attachment, content []byte) error {
	jiraKey := extractJiraKey(workItemID)

	// Create multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create file part
	part, err := writer.CreateFormFile("file", attachment.Filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	// Write content to part
	if _, err := part.Write(content); err != nil {
		return fmt.Errorf("failed to write attachment content: %w", err)
	}

	// Close writer to finalize multipart message
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Build request URL
	path := fmt.Sprintf("/rest/api/3/issue/%s/attachments", url.PathEscape(jiraKey))
	url := j.config.BaseURL + path

	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers for Jira attachments
	req.SetBasicAuth(j.config.Email, j.config.APIToken)
	req.Header.Set("X-Atlassian-Token", "no-check")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := j.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload attachment (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// Search searches for work items matching criteria.
func (j *JiraOffice) Search(ctx context.Context, clusterID string, query string) ([]contracts.WorkItem, error) {
	// Build JQL query
	jql := query
	if j.config.ProjectKey != "" && !strings.Contains(strings.ToUpper(query), "PROJECT") {
		// Default to searching in configured project
		jql = fmt.Sprintf("project = %s AND (%s)", j.config.ProjectKey, query)
	}

	path := fmt.Sprintf("/rest/api/3/search/jql?jql=%s&maxResults=50", url.QueryEscape(jql))
	resp, err := j.jiraRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search Jira: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search Jira (status %d): %s", resp.StatusCode, string(body))
	}

	var result JiraSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	// Convert Jira issues to WorkItems
	workItems := make([]contracts.WorkItem, 0, len(result.Issues))
	for _, issue := range result.Issues {
		workItems = append(workItems, *j.convertToWorkItem(&issue, nil))
	}

	return workItems, nil
}

// Watch returns a channel for receiving work item events.
func (j *JiraOffice) Watch(ctx context.Context, clusterID string) (<-chan pkgoffice.WorkItemEvent, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	// If server already running, return existing channel
	if j.eventChan != nil {
		return j.eventChan, nil
	}

	// Create event channel
	j.eventChan = make(chan pkgoffice.WorkItemEvent, 100)
	j.serverDone = make(chan struct{})

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc(j.config.WebhookPath, j.webhookHandler)
	// Add health endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	j.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", j.config.WebhookPort),
		Handler: mux,
		// Timeouts can be configured later
	}

	// Start server in goroutine
	go func() {
		if err := j.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error (TODO: add proper logging)
			fmt.Printf("Jira webhook server error: %v\n", err)
		}
		close(j.serverDone)
	}()

	// Start goroutine to shutdown server when context is cancelled
	go func() {
		<-ctx.Done()
		j.stopWebhookServer(context.Background()) // use background context for shutdown
	}()

	return j.eventChan, nil
}

// validateAtlassianSignature validates Jira webhook HMAC-SHA256 signature.
func (j *JiraOffice) validateAtlassianSignature(r *http.Request, body []byte) bool {
	signatureHeader := r.Header.Get("X-Atlassian-Webhook-Signature")
	if signatureHeader == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(j.config.WebhookSecret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expectedSignature), []byte(signatureHeader)) == 1
}

// webhookHandler handles incoming Jira webhook events.
func (j *JiraOffice) webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body for validation and parsing
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate HMAC signature if secret is configured
	if j.config.WebhookSecret != "" {
		if !j.validateAtlassianSignature(r, body) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse Jira webhook event
	var webhookEvent JiraWebhookEvent
	if err := json.Unmarshal(body, &webhookEvent); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Extract custom fields from webhook issue
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	issueMap, ok := raw["issue"].(map[string]interface{})
	if !ok {
		// No issue in webhook? should not happen
		w.WriteHeader(http.StatusOK)
		return
	}
	fields, ok := issueMap["fields"].(map[string]interface{})
	if !ok {
		// No fields
		w.WriteHeader(http.StatusOK)
		return
	}
	customFields := j.extractCustomFields(fields)

	// Map webhook event type to WorkEventType
	var eventType pkgoffice.WorkEventType
	switch webhookEvent.WebhookEvent {
	case "jira:issue_created":
		eventType = pkgoffice.WorkItemCreated
	case "jira:issue_updated":
		eventType = pkgoffice.WorkItemUpdated
	case "comment_created":
		eventType = pkgoffice.WorkItemCommented
	default:
		// Ignore other event types
		w.WriteHeader(http.StatusOK)
		return
	}

	// Convert Jira issue to WorkItem
	workItem := j.convertToWorkItem(&webhookEvent.Issue, customFields)

	// Create WorkItemEvent
	event := pkgoffice.WorkItemEvent{
		Type:      eventType,
		WorkItem:  workItem,
		Timestamp: time.Now(),
	}

	// Send event to channel (non-blocking)
	select {
	case j.eventChan <- event:
		// Event delivered
	default:
		// Channel full, log warning (TODO: add logging)
	}

	w.WriteHeader(http.StatusOK)
}

// stopWebhookServer gracefully stops the HTTP server.
func (j *JiraOffice) stopWebhookServer(ctx context.Context) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.server == nil {
		return nil
	}

	// Shutdown server with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := j.server.Shutdown(shutdownCtx)

	// Wait for server goroutine to finish
	select {
	case <-j.serverDone:
		// Server stopped
	case <-shutdownCtx.Done():
		// Timeout
	}

	// Clean up channels
	close(j.eventChan)
	j.eventChan = nil
	j.server = nil

	return err
}

// convertToWorkItem converts a JiraIssue to a canonical WorkItem.
func (j *JiraOffice) convertToWorkItem(issue *JiraIssue, customFields map[string]interface{}) *contracts.WorkItem {
	// Map Jira status to canonical WorkStatus
	canonicalStatus := j.mapStatus(issue.Fields.Status.Name)

	workItem := &contracts.WorkItem{
		ID:                  issue.Key,
		Title:               issue.Fields.Summary,
		Summary:             issue.Fields.Summary,
		Body:                issue.Fields.Description,
		CreatedAt:           issue.Fields.Created.Time,
		UpdatedAt:           issue.Fields.Updated.Time,
		Status:              canonicalStatus,
		WorkType:            j.mapWorkType(issue.Fields.Issuetype.Name),
		Priority:            j.mapPriority(issue.Fields.Priority.Name),
		WorkDomain:          contracts.DomainCore,      // Default, can be refined later
		ExecutionMode:       contracts.ModeAutonomous,  // Default
		EvidenceRequirement: contracts.EvidenceSummary, // Default
		Source: contracts.SourceMetadata{
			System:    "jira",
			IssueKey:  issue.Key,
			Project:   strings.Split(issue.Key, "-")[0],
			IssueType: issue.Fields.Issuetype.Name,
			Reporter:  issue.Fields.Reporter.DisplayName,
			Assignee:  issue.Fields.Assignee.DisplayName,
			CreatedAt: issue.Fields.Created.Time,
			UpdatedAt: issue.Fields.Updated.Time,
		},
		Tags: contracts.WorkTags{
			HumanOrg: issue.Fields.Labels,
		},
	}

	// Add custom fields as tags
	if customFields != nil {
		for key, value := range customFields {
			// Format value as string
			var strVal string
			switch v := value.(type) {
			case string:
				strVal = v
			case int, int64, float64:
				strVal = fmt.Sprintf("%v", v)
			case bool:
				strVal = fmt.Sprintf("%t", v)
			default:
				// Try JSON marshal for complex objects
				if bytes, err := json.Marshal(v); err == nil {
					strVal = string(bytes)
				} else {
					strVal = fmt.Sprintf("%v", v)
				}
			}
			workItem.Tags.HumanOrg = append(workItem.Tags.HumanOrg, fmt.Sprintf("%s:%s", key, strVal))
		}
	}

	return workItem
}

func (j *JiraOffice) workTypeFromString(s string) contracts.WorkType {
	switch s {
	case "debug":
		return contracts.WorkTypeDebug
	case "implementation":
		return contracts.WorkTypeImplementation
	case "design":
		return contracts.WorkTypeDesign
	case "research":
		return contracts.WorkTypeResearch
	case "analysis":
		return contracts.WorkTypeAnalysis
	case "documentation":
		return contracts.WorkTypeDocumentation
	case "refactor":
		return contracts.WorkTypeRefactor
	case "security":
		return contracts.WorkTypeSecurity
	case "testing":
		return contracts.WorkTypeTesting
	case "operations":
		return contracts.WorkTypeOperations
	default:
		return contracts.WorkTypeImplementation
	}
}

func (j *JiraOffice) priorityFromString(s string) contracts.Priority {
	switch s {
	case "critical":
		return contracts.PriorityCritical
	case "high":
		return contracts.PriorityHigh
	case "medium":
		return contracts.PriorityMedium
	case "low":
		return contracts.PriorityLow
	case "background":
		return contracts.PriorityBackground
	default:
		return contracts.PriorityMedium
	}
}

func (j *JiraOffice) workStatusFromString(s string) contracts.WorkStatus {
	switch s {
	case "requested":
		return contracts.StatusRequested
	case "analyzing":
		return contracts.StatusAnalyzing
	case "analyzed":
		return contracts.StatusAnalyzed
	case "planning":
		return contracts.StatusPlanning
	case "planned":
		return contracts.StatusPlanned
	case "pending_approval":
		return contracts.StatusPendingApproval
	case "approved":
		return contracts.StatusApproved
	case "queued":
		return contracts.StatusQueued
	case "running":
		return contracts.StatusRunning
	case "blocked":
		return contracts.StatusBlocked
	case "completed":
		return contracts.StatusCompleted
	case "failed":
		return contracts.StatusFailed
	case "canceled":
		return contracts.StatusCanceled
	default:
		return contracts.StatusRequested
	}
}

// mapWorkType maps Jira issue type to canonical WorkType.
func (j *JiraOffice) mapWorkType(jiraType string) contracts.WorkType {
	// Check config mapping first (exact match)
	if mapped, ok := j.config.WorkTypeMapping[jiraType]; ok {
		return j.workTypeFromString(mapped)
	}
	// Fallback to hardcoded mapping (case-insensitive)
	switch strings.ToLower(jiraType) {
	case "bug", "defect":
		return contracts.WorkTypeDebug
	case "task", "chore":
		return contracts.WorkTypeImplementation
	case "story", "feature":
		return contracts.WorkTypeDesign
	case "epic":
		return contracts.WorkTypeResearch
	case "spike":
		return contracts.WorkTypeResearch
	case "improvement":
		return contracts.WorkTypeRefactor
	default:
		return contracts.WorkTypeImplementation
	}
}

// mapPriority maps Jira priority to canonical Priority.
func (j *JiraOffice) mapPriority(jiraPriority string) contracts.Priority {
	// Check config mapping first (exact match)
	if mapped, ok := j.config.PriorityMapping[jiraPriority]; ok {
		return j.priorityFromString(mapped)
	}
	// Fallback to hardcoded mapping (case-insensitive)
	switch strings.ToLower(jiraPriority) {
	case "highest", "critical", "1":
		return contracts.PriorityCritical
	case "high", "2":
		return contracts.PriorityHigh
	case "medium", "3":
		return contracts.PriorityMedium
	case "low", "4":
		return contracts.PriorityLow
	case "lowest", "5":
		return contracts.PriorityBackground
	default:
		return contracts.PriorityMedium
	}
}

// mapStatus maps Jira status to canonical WorkStatus.
func (j *JiraOffice) mapStatus(jiraStatus string) contracts.WorkStatus {
	// Check config mapping first (exact match)
	if mapped, ok := j.config.StatusMapping[jiraStatus]; ok {
		return j.workStatusFromString(mapped)
	}
	// Fallback to hardcoded mapping (case-insensitive)
	lower := strings.ToLower(jiraStatus)

	switch {
	case strings.Contains(lower, "to do") || strings.Contains(lower, "backlog") || strings.Contains(lower, "requested"):
		return contracts.StatusRequested
	case strings.Contains(lower, "in progress") || strings.Contains(lower, "in development"):
		return contracts.StatusRunning
	case strings.Contains(lower, "done") || strings.Contains(lower, "completed") || strings.Contains(lower, "closed"):
		return contracts.StatusCompleted
	case strings.Contains(lower, "blocked") || strings.Contains(lower, "on hold") || strings.Contains(lower, "paused"):
		return contracts.StatusBlocked
	case strings.Contains(lower, "review") || strings.Contains(lower, "testing"):
		return contracts.StatusRunning
	case strings.Contains(lower, "failed"):
		return contracts.StatusFailed
	case strings.Contains(lower, "canceled"):
		return contracts.StatusCanceled
	default:
		return contracts.StatusRequested
	}
}

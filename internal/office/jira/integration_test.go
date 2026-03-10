package jira

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	pkgoffice "github.com/kube-zen/zen-brain1/pkg/office"
)

// TestAddComment_WithAIAttribution tests that AI attribution is properly injected.
func TestAddComment_WithAIAttribution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TEST-123/comment" {
			// Verify request body contains AI attribution
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			// Check body structure
			body, ok := payload["body"].(map[string]interface{})
			if !ok {
				t.Error("Body should be a map")
				return
			}

			content, ok := body["content"].([]interface{})
			if !ok {
				t.Error("Body content should be an array")
				return
			}

			if len(content) == 0 {
				t.Error("Body content should not be empty")
				return
			}

			paragraph, ok := content[0].(map[string]interface{})
			if !ok {
				t.Error("Content should contain a paragraph")
				return
			}

			paragraphContent, ok := paragraph["content"].([]interface{})
			if !ok {
				t.Error("Paragraph should have content")
				return
			}

			if len(paragraphContent) == 0 {
				t.Error("Paragraph content should not be empty")
				return
			}

			text, ok := paragraphContent[0].(map[string]interface{})
			if !ok {
				t.Error("Content should contain text")
				return
			}

			bodyText, ok := text["text"].(string)
			if !ok {
				t.Error("Text should be a string")
				return
			}

			// Verify AI attribution header is present
			if bodyText == "" {
				t.Error("Body text should not be empty")
			}

			if !contains(bodyText, "[zen-brain") {
				t.Errorf("AI attribution header should be present in comment body: %s", bodyText)
			}

			if !contains(bodyText, "agent: worker-debug") {
				t.Errorf("Agent role should be in comment body: %s", bodyText)
			}

			if !contains(bodyText, "model: glm-4.7") {
				t.Errorf("Model should be in comment body: %s", bodyText)
			}

			if !contains(bodyText, "Original comment text") {
				t.Errorf("Original comment text should be preserved: %s", bodyText)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": "10001"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}

	connector, _ := New("test-jira", "cluster-1", config)

	ctx := context.Background()
	comment := &contracts.Comment{
		ID:         "comment-1",
		WorkItemID: "TEST-123",
		Body:       "Original comment text",
		Author:     "zen-brain",
		CreatedAt:  time.Now(),
		Attribution: &contracts.AIAttribution{
			AgentRole: "worker-debug",
			ModelUsed: "glm-4.7",
			SessionID: "session-123",
			TaskID:    "task-456",
			Timestamp: time.Date(2026, 3, 7, 14, 30, 0, 0, time.UTC),
		},
	}

	err := connector.AddComment(ctx, "cluster-1", "TEST-123", comment)
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}
}

// TestAddComment_WithoutAIAttribution tests that comments work without attribution.
func TestAddComment_WithoutAIAttribution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TEST-123/comment" {
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			// Verify body text is present without attribution
			body, _ := payload["body"].(map[string]interface{})
			content, _ := body["content"].([]interface{})
			paragraph, _ := content[0].(map[string]interface{})
			paragraphContent, _ := paragraph["content"].([]interface{})
			text, _ := paragraphContent[0].(map[string]interface{})
			bodyText, _ := text["text"].(string)

			if bodyText == "" {
				t.Error("Body text should not be empty")
			}

			if contains(bodyText, "[zen-brain") {
				t.Error("No AI attribution header should be present when attribution is nil")
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": "10001"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}

	connector, _ := New("test-jira", "cluster-1", config)

	ctx := context.Background()
	comment := &contracts.Comment{
		ID:         "comment-1",
		WorkItemID: "TEST-123",
		Body:       "Comment without attribution",
		Author:     "test-user",
		CreatedAt:  time.Now(),
		// Attribution is nil
	}

	err := connector.AddComment(ctx, "cluster-1", "TEST-123", comment)
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}
}

// TestAddComment_WithLongBody tests that long comments are handled.
func TestAddComment_WithLongBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TEST-123/comment" {
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			body, _ := payload["body"].(map[string]interface{})
			content, _ := body["content"].([]interface{})
			paragraph, _ := content[0].(map[string]interface{})
			paragraphContent, _ := paragraph["content"].([]interface{})
			text, _ := paragraphContent[0].(map[string]interface{})
			bodyText, _ := text["text"].(string)

			// Verify body is not empty (truncation happens in factory, not here)
			if len(bodyText) < 100 {
				t.Errorf("Body should be long, got %d chars", len(bodyText))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": "10001"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}

	connector, _ := New("test-jira", "cluster-1", config)

	ctx := context.Background()
	longBody := "This is a very long comment. " + repeatString("Repeated text. ", 100)
	comment := &contracts.Comment{
		ID:         "comment-1",
		WorkItemID: "TEST-123",
		Body:       longBody,
		Author:     "zen-brain",
		CreatedAt:  time.Now(),
	}

	err := connector.AddComment(ctx, "cluster-1", "TEST-123", comment)
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}
}

// TestUpdateStatus_TransitionSuccess tests successful status transitions.
func TestUpdateStatus_TransitionSuccess(t *testing.T) {
	transitionCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TEST-123/transitions" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"transitions": [
					{"id": "11", "name": "Done"},
					{"id": "21", "name": "In Progress"}
				]
			}`))
			return
		}

		if r.URL.Path == "/rest/api/3/issue/TEST-123/transitions" && r.Method == "POST" {
			transitionCalled = true

			// Verify transition payload
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			transitionID, ok := payload["transition"].(map[string]interface{})
			if !ok {
				t.Error("Transition should be present in payload")
				return
			}

			id, ok := transitionID["id"].(string)
			if !ok || id != "11" {
				t.Errorf("Expected transition ID '11', got %v", transitionID["id"])
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}

	connector, _ := New("test-jira", "cluster-1", config)

	ctx := context.Background()
	err := connector.UpdateStatus(ctx, "cluster-1", "TEST-123", contracts.StatusCompleted)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	if !transitionCalled {
		t.Error("Transition should have been called")
	}
}

// TestUpdateStatus_TransitionNotFound tests error when no transition is found.
func TestUpdateStatus_TransitionNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TEST-123/transitions" && r.Method == "GET" {
			// Return no matching transitions
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"transitions": [
					{"id": "11", "name": "Start Progress"}
				]
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}

	connector, _ := New("test-jira", "cluster-1", config)

	ctx := context.Background()
	err := connector.UpdateStatus(ctx, "cluster-1", "TEST-123", contracts.StatusCompleted)
	if err == nil {
		t.Error("Expected error for transition not found")
	}

	if !contains(err.Error(), "no suitable transition") {
		t.Errorf("Expected 'no suitable transition' error, got: %v", err)
	}
}

// TestFetchAndCommentWorkflow tests the complete proof-of-work workflow.
func TestFetchAndCommentWorkflow(t *testing.T) {
	issueFetched := false
	commentAdded := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROOF-456" {
			issueFetched = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"key": "PROOF-456",
				"id": "10002",
				"fields": {
					"summary": "Proof-of-Work Test Issue",
					"description": "Test workflow for proof-of-work integration",
					"created": "2026-03-07T10:00:00.000+0000",
					"status": {"name": "In Progress"},
					"priority": {"name": "High"},
					"issuetype": {"name": "Task"},
					"project": {"key": "PROOF"},
					"reporter": {"displayName": "Test User"},
					"labels": ["test", "workflow"]
				}
			}`))
			return
		}

		if r.URL.Path == "/rest/api/3/issue/PROOF-456/transitions" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"transitions": [
					{"id": "21", "name": "Done"}
				]
			}`))
			return
		}

		if r.URL.Path == "/rest/api/3/issue/PROOF-456/transitions" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.URL.Path == "/rest/api/3/issue/PROOF-456/comment" {
			commentAdded = true

			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			// Verify comment structure
			body, _ := payload["body"].(map[string]interface{})
			content, _ := body["content"].([]interface{})
			if len(content) > 0 {
				paragraph, _ := content[0].(map[string]interface{})
				paragraphContent, _ := paragraph["content"].([]interface{})
				if len(paragraphContent) > 0 {
					text, _ := paragraphContent[0].(map[string]interface{})
					bodyText, _ := text["text"].(string)

					if !contains(bodyText, "Proof-of-Work") {
						t.Errorf("Comment should contain 'Proof-of-Work': %s", bodyText)
					}
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": "10003"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}

	connector, _ := New("test-jira", "cluster-1", config)

	ctx := context.Background()

	// Step 1: Fetch the issue
	workItem, err := connector.FetchBySourceKey(ctx, "cluster-1", "PROOF-456")
	if err != nil {
		t.Fatalf("Failed to fetch issue: %v", err)
	}

	if !issueFetched {
		t.Error("Issue should have been fetched")
	}

	if workItem.ID != "PROOF-456" {
		t.Errorf("Expected issue ID 'PROOF-456', got %s", workItem.ID)
	}

	if workItem.Status != contracts.StatusRunning {
		t.Errorf("Expected status StatusRunning, got %v", workItem.Status)
	}

	// Step 2: Add proof-of-work comment
	comment := &contracts.Comment{
		ID:         "proof-1",
		WorkItemID: "PROOF-456",
		Body:       "Proof-of-Work:\n\nTask completed successfully.\n\nFiles changed: 5\nTests run: 10\nTests passed: 10",
		Author:     "zen-brain",
		CreatedAt:  time.Now(),
		Attribution: &contracts.AIAttribution{
			AgentRole: "factory",
			ModelUsed: "factory-v1",
			SessionID: "session-789",
			TaskID:    "task-012",
			Timestamp: time.Now(),
		},
	}

	err = connector.AddComment(ctx, "cluster-1", "PROOF-456", comment)
	if err != nil {
		t.Fatalf("Failed to add comment: %v", err)
	}

	if !commentAdded {
		t.Error("Comment should have been added")
	}

	// Step 3: Update status to completed
	err = connector.UpdateStatus(ctx, "cluster-1", "PROOF-456", contracts.StatusCompleted)
	if err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}
}

// TestAddComment_HTTPError tests error handling when Jira returns an error.
func TestAddComment_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TEST-123/comment" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"errorMessages": ["Internal server error"]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}

	connector, _ := New("test-jira", "cluster-1", config)

	ctx := context.Background()
	comment := &contracts.Comment{
		ID:         "comment-1",
		WorkItemID: "TEST-123",
		Body:       "Test comment",
		Author:     "test-user",
		CreatedAt:  time.Now(),
	}

	err := connector.AddComment(ctx, "cluster-1", "TEST-123", comment)
	if err == nil {
		t.Error("Expected error for HTTP 500")
	}

	if !contains(err.Error(), "failed to add comment") {
		t.Errorf("Expected 'failed to add comment' error, got: %v", err)
	}
}

// TestUpdateStatus_HTTPError tests error handling when Jira returns an error.
func TestUpdateStatus_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TEST-123/transitions" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"errorMessages": ["Internal server error"]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}

	connector, _ := New("test-jira", "cluster-1", config)

	ctx := context.Background()
	err := connector.UpdateStatus(ctx, "cluster-1", "TEST-123", contracts.StatusCompleted)
	if err == nil {
		t.Error("Expected error for HTTP 500")
	}

	if !contains(err.Error(), "failed to get transitions") && !contains(err.Error(), "failed to execute transition") {
		t.Errorf("Expected transition error, got: %v", err)
	}
}

// TestVerticalSliceOfficeFlow_AddCommentAddAttachmentUpdateStatus exercises the full
// Office path used by vertical-slice (non-mock): Fetch, AddComment, AddAttachment, UpdateStatus.
func TestVerticalSliceOfficeFlow_AddCommentAddAttachmentUpdateStatus(t *testing.T) {
	var fetched, commentAdded, attachmentAdded, statusUpdated bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/VS-1" && r.Method == "GET" {
			fetched = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"key":"VS-1","id":"1","fields":{"summary":"Vertical slice flow","description":"","created":"2026-01-01T00:00:00.000+0000","updated":"2026-01-01T00:00:00.000+0000","status":{"name":"To Do"},"priority":{"name":"Medium"},"issuetype":{"name":"Task"},"project":{"key":"VS"},"reporter":{},"assignee":{},"labels":[]}}`))
			return
		}
		if r.URL.Path == "/rest/api/3/issue/VS-1/comment" && r.Method == "POST" {
			commentAdded = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":"101"}`))
			return
		}
		if r.URL.Path == "/rest/api/3/issue/VS-1/attachments" && r.Method == "POST" {
			attachmentAdded = true
			if r.Header.Get("X-Atlassian-Token") != "no-check" {
				t.Error("expected X-Atlassian-Token: no-check for attachment")
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"id":"1"}]`))
			return
		}
		if r.URL.Path == "/rest/api/3/issue/VS-1/transitions" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"transitions":[{"id":"31","name":"Done"}]}`))
			return
		}
		if r.URL.Path == "/rest/api/3/issue/VS-1/transitions" && r.Method == "POST" {
			statusUpdated = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}
	connector, err := New("vertical-slice-flow", "cluster-1", config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	// Fetch
	item, err := connector.Fetch(ctx, "cluster-1", "VS-1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !fetched {
		t.Error("Fetch did not hit server")
	}
	if item.ID != "VS-1" {
		t.Errorf("Fetch: got ID %q", item.ID)
	}

	// AddComment (proof-of-work style)
	comment := &contracts.Comment{
		ID:         "pow-1",
		WorkItemID: "VS-1",
		Body:       "Proof-of-Work summary\n\nTask completed.",
		Author:     "zen-brain",
		CreatedAt:  time.Now(),
		Attribution: &contracts.AIAttribution{
			AgentRole: "factory",
			ModelUsed: "factory-v1",
			SessionID: "s1",
			TaskID:    "t1",
			Timestamp: time.Now(),
		},
	}
	if err := connector.AddComment(ctx, "cluster-1", "VS-1", comment); err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if !commentAdded {
		t.Error("AddComment did not hit server")
	}

	// AddAttachment
	att := &contracts.Attachment{
		ID:          "att-1",
		WorkItemID:  "VS-1",
		Filename:    "proof-of-work.json",
		ContentType: "application/json",
		Size:        10,
		CreatedAt:   time.Now(),
	}
	if err := connector.AddAttachment(ctx, "cluster-1", "VS-1", att, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("AddAttachment: %v", err)
	}
	if !attachmentAdded {
		t.Error("AddAttachment did not hit server")
	}

	// UpdateStatus
	if err := connector.UpdateStatus(ctx, "cluster-1", "VS-1", contracts.StatusCompleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if !statusUpdated {
		t.Error("UpdateStatus did not hit server")
	}
}

// minimalJiraWebhookBody returns a valid Jira webhook JSON body for the given event type.
func minimalJiraWebhookBody(eventType string) []byte {
	body := []byte(`{"webhookEvent":"` + eventType + `","timestamp":0,"issue":{"key":"W-1","id":"1","self":"","fields":{"summary":"Watch test","description":"","status":{"name":"Open"},"priority":{"name":"Medium"},"issuetype":{"name":"Task"},"project":{"key":"W"},"reporter":{"displayName":""},"assignee":{"displayName":""},"labels":[],"created":"2026-01-01T00:00:00.000+0000","updated":"2026-01-01T00:00:00.000+0000"}}}`)
	return body
}

// computeWebhookSignature returns HMAC-SHA256 hex of body using secret.
func computeWebhookSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// TestWatch_ValidatesWebhookSecret ensures that when WebhookSecret is set, requests without
// a valid X-Atlassian-Webhook-Signature are rejected with 401.
func TestWatch_ValidatesWebhookSecret(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	config := &Config{
		BaseURL:       "https://test.atlassian.net",
		Email:         "test@example.com",
		APIToken:      "test-token",
		WebhookPort:   port,
		WebhookPath:   "/webhook",
		WebhookSecret: "test-secret",
	}
	connector, err := New("watch-secret", "cluster-1", config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := connector.Watch(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	_ = ch
	time.Sleep(100 * time.Millisecond)

	body := minimalJiraWebhookBody("jira:issue_created")
	webhookURL := "http://127.0.0.1:" + strconv.Itoa(port) + "/webhook"

	// POST without signature -> 401
	req, _ := http.NewRequest("POST", webhookURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST (no sig): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without signature, got %d", resp.StatusCode)
	}

	// POST with wrong signature -> 401
	req2, _ := http.NewRequest("POST", webhookURL, bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Atlassian-Webhook-Signature", "wrong")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("POST (wrong sig): %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong signature, got %d", resp2.StatusCode)
	}

	// POST with valid signature -> 200
	sig := computeWebhookSignature("test-secret", body)
	req3, _ := http.NewRequest("POST", webhookURL, bytes.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Atlassian-Webhook-Signature", sig)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("POST (valid sig): %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with valid signature, got %d", resp3.StatusCode)
	}
}

// TestWatch_EmitsCreatedUpdatedCommentedEvents ensures that created/updated/commented
// webhook events result in the correct WorkItemEvent types on the channel.
func TestWatch_EmitsCreatedUpdatedCommentedEvents(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	config := &Config{
		BaseURL:       "https://test.atlassian.net",
		Email:         "test@example.com",
		APIToken:      "test-token",
		WebhookPort:   port,
		WebhookPath:   "/webhook",
		WebhookSecret: "watch-events-secret",
	}
	connector, err := New("watch-events", "cluster-1", config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := connector.Watch(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	webhookURL := "http://127.0.0.1:" + strconv.Itoa(port) + "/webhook"
	events := []struct {
		webhookEvent string
		expectedType pkgoffice.WorkEventType
	}{
		{"jira:issue_created", pkgoffice.WorkItemCreated},
		{"jira:issue_updated", pkgoffice.WorkItemUpdated},
		{"comment_created", pkgoffice.WorkItemCommented},
	}

	for _, e := range events {
		body := minimalJiraWebhookBody(e.webhookEvent)
		sig := computeWebhookSignature("watch-events-secret", body)
		req, _ := http.NewRequest("POST", webhookURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Atlassian-Webhook-Signature", sig)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", e.webhookEvent, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("POST %s: expected 200, got %d", e.webhookEvent, resp.StatusCode)
		}

		select {
		case ev := <-ch:
			if ev.Type != e.expectedType {
				t.Errorf("event type: expected %s, got %s", e.expectedType, ev.Type)
			}
			if ev.WorkItem == nil || ev.WorkItem.ID != "W-1" {
				t.Errorf("event WorkItem: expected ID W-1, got %v", ev.WorkItem)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("timeout waiting for event %s", e.webhookEvent)
		}
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

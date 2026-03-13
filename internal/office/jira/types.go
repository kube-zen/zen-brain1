package jira

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JiraTime is a custom time type for parsing Jira's ISO 8601 timestamps.
type JiraTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler.
func (jt *JiraTime) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// Jira returns ISO 8601 timestamps like "2024-01-01T12:00:00.000+0000"
	// Try multiple formats
	formats := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}

	var t time.Time
	var err error
	for _, format := range formats {
		t, err = time.Parse(format, s)
		if err == nil {
			jt.Time = t
			return nil
		}
	}

	return err
}

// JiraADFNode represents a node in Atlassian Document Format (ADF).
type JiraADFNode struct {
	Type    string            `json:"type"`
	Content []map[string]any  `json:"content,omitempty"`
	Attrs   map[string]string   `json:"attrs,omitempty"`
	Mark    []map[string]any   `json:"marks,omitempty"`
	Text     string            `json:"text,omitempty"`
}

// JiraDescription represents a Jira description that can be:
// - string (legacy/compatibility)
// - null (empty description)
// - ADF object (Jira REST API v3 format)
type JiraDescription struct {
	raw any
}

// UnmarshalJSON implements json.Unmarshaler for JiraDescription.
// Handles string, null, and ADF object formats.
func (jd *JiraDescription) UnmarshalJSON(data []byte) error {
	// Handle null
	if string(data) == "null" {
		jd.raw = nil
		return nil
	}

	// Try string first (backward compatibility)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		jd.raw = s
		return nil
	}

	// Try ADF object (Jira API v3 format)
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		jd.raw = obj
		return nil
	}

	return fmt.Errorf("unrecognized Jira description format: %s", string(data))
}

// PlainText extracts readable plain text from the description.
// Handles string, null, and ADF object formats.
func (jd *JiraDescription) PlainText() string {
	if jd.raw == nil {
		return ""
	}

	// If it's already a string, return it
	if s, ok := jd.raw.(string); ok {
		return s
	}

	// If it's an ADF object, extract text
	if obj, ok := jd.raw.(map[string]any); ok {
		return extractADFText(obj)
	}

	return ""
}

// String returns a string representation (for compatibility).
func (jd *JiraDescription) String() string {
	return jd.PlainText()
}

// extractADFText recursively extracts text from an ADF document structure.
// Handles common ADF node types: doc, paragraph, text, orderedList, bulletList.
func extractADFText(adf map[string]any) string {
	if adf == nil {
		return ""
	}

	// Get content array
	content, ok := adf["content"].([]any)
	if !ok {
		return ""
	}

	var text strings.Builder

	for _, item := range content {
		if node, ok := item.(map[string]any); ok {
			nodeType, _ := node["type"].(string)

			switch nodeType {
			case "paragraph", "heading", "text":
				// Recursively extract from content
				if paraContent, ok := node["content"].([]any); ok {
					for _, child := range paraContent {
						if childNode, ok := child.(map[string]any); ok {
							if childType, _ := childNode["type"].(string); childType == "text" {
								if txt, ok := childNode["text"].(string); ok {
									text.WriteString(txt)
									if nodeType == "paragraph" {
										text.WriteString("\n\n")
									} else if nodeType == "heading" {
										text.WriteString("\n")
									}
								}
							}
						}
					}
				}

			case "orderedList", "bulletList":
				// Extract list items
				if listContent, ok := node["content"].([]any); ok {
					for i, listItem := range listContent {
						if itemNode, ok := listItem.(map[string]any); ok {
							if itemContent, ok := itemNode["content"].([]any); ok {
								text.WriteString(fmt.Sprintf("%d. ", i+1))
								for _, child := range itemContent {
									if childNode, ok := child.(map[string]any); ok {
										if childType, _ := childNode["type"].(string); childType == "text" {
											if txt, ok := childNode["text"].(string); ok {
												text.WriteString(txt)
											}
										}
									}
								}
								text.WriteString("\n")
							}
						}
					}
				}

			default:
				// Best-effort: try to extract text from unknown node types
				if nodeContent, ok := node["content"].([]any); ok {
					for _, child := range nodeContent {
						if childNode, ok := child.(map[string]any); ok {
							if txt, ok := childNode["text"].(string); ok {
								text.WriteString(txt)
								text.WriteString(" ")
							}
						}
					}
				}
			}
		}
	}

	return strings.TrimSpace(text.String())
}

// JiraIssue represents a Jira issue from the REST API.
type JiraIssue struct {
	Key    string `json:"key"`
	ID     string `json:"id"`
	Self   string `json:"self"`
	Fields struct {
		Summary              string          `json:"summary"`
		Description          JiraDescription `json:"description"`
		Created              JiraTime       `json:"created"`
		Updated              JiraTime       `json:"updated"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
		Issuetype struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Project struct {
			Key  string `json:"key"`
			Name string `json:"name,omitempty"`
		} `json:"project"`
		Reporter struct {
			DisplayName string `json:"displayName"`
		} `json:"reporter"`
		Assignee struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Labels []string `json:"labels"`
		// Optional common fields
		Components []struct {
			Name string `json:"name"`
		} `json:"components,omitempty"`
		Parent *struct {
			Key string `json:"key"`
		} `json:"parent,omitempty"`
		FixVersions []struct {
			Name string `json:"name"`
		} `json:"fixVersions,omitempty"`
		Versions []struct {
			Name string `json:"name"`
		} `json:"versions,omitempty"`
		Resolution *struct {
			Name string `json:"name"`
		} `json:"resolution,omitempty"`
		// Common custom fields (examples)
		EpicLink    string `json:"customfield_10014,omitempty"`
		StoryPoints int    `json:"customfield_10016,omitempty"`
		Sprint      []struct {
			Name string `json:"name"`
		} `json:"customfield_10020,omitempty"`
	} `json:"fields"`
}

// JiraComment represents a comment in Jira.
type JiraComment struct {
	ID     string `json:"id"`
	Author struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// JiraTransition represents a status transition in Jira.
type JiraTransition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		Name string `json:"name"`
	} `json:"to"`
}

// JiraSearchResult represents search results from Jira.
type JiraSearchResult struct {
	StartAt    int         `json:"startAt"`
	MaxResults int         `json:"maxResults"`
	Total      int         `json:"total"`
	Issues     []JiraIssue `json:"issues"`
}

// JiraWebhookEvent represents a webhook event from Jira.
type JiraWebhookEvent struct {
	Timestamp    int64     `json:"timestamp"`
	WebhookEvent string    `json:"webhookEvent"`
	Issue        JiraIssue `json:"issue"`
	User         struct {
		DisplayName string `json:"displayName"`
	} `json:"user"`
	Changelog *struct {
		Items []struct {
			Field      string `json:"field"`
			FromString string `json:"fromString"`
			ToString   string `json:"toString"`
		} `json:"items"`
	} `json:"changelog,omitempty"`
	Comment *JiraComment `json:"comment,omitempty"`
}

// JiraStatusMapping maps Jira status names to canonical WorkStatus.
var JiraStatusMapping = map[string]string{
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

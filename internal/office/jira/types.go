package jira

import (
	"encoding/json"
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

// JiraIssue represents a Jira issue from the REST API.
type JiraIssue struct {
	Key    string `json:"key"`
	ID     string `json:"id"`
	Self   string `json:"self"`
	Fields struct {
		Summary     string   `json:"summary"`
		Description string   `json:"description"`
		Created     JiraTime `json:"created"`
		Updated     JiraTime `json:"updated"`
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

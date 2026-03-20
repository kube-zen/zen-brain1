 `package contracts

import (
	"context"
	"fmt"

	// WorkItem represents a work item from the Office connector
type WorkItem struct {
	ID                string    `json:"id"`
	Title               string    `json:"title"`
	Summary         string    `json:"summary"`
	Body          string    `json:"body"`
	WorkType       WorkType ` `json:"work_type"`
	WorkDomain     WorkDomain `json:"work_domain"`
	Priority     Priority `json:"priority"`
	Status       WorkStatus`json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Reporter       string    `json:"reporter"`
	Assignee       string    `json:"assignee"`
	Labels               map[string]string `json:"labels"`
	Source               SourceMetadata `json:"source"`
	SourceMetadata `json:"source"`
	SourceMetadata
)
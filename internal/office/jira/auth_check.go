package jira

import (
	"context"
	"fmt"
)

// CheckAuth performs authentication validation via GET /rest/api/3/myself.
// This is separate from project access validation.
func (j *JiraOffice) CheckAuth(ctx context.Context) error {
	if err := j.ValidateConfig(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Use /myself endpoint for pure auth check
	path := "/rest/api/3/myself"
	resp, err := j.jiraRequest(ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("auth failed (status %d)", resp.StatusCode)
	}

	return nil
}

// CheckProjectAccess validates that the configured project key is accessible.
// This is separate from authentication validation.
func (j *JiraOffice) CheckProjectAccess(ctx context.Context) error {
	if j.config.ProjectKey == "" {
		return fmt.Errorf("project_key not configured")
	}

	// Check direct project access
	path := fmt.Sprintf("/rest/api/3/project/%s", j.config.ProjectKey)
	resp, err := j.jiraRequest(ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("project access request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("project %s not found or not accessible", j.config.ProjectKey)
	}

	if resp.StatusCode == 403 {
		return fmt.Errorf("project %s forbidden (account lacks permissions)", j.config.ProjectKey)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("project %s access failed (status %d)", j.config.ProjectKey, resp.StatusCode)
	}

	return nil
}

// Ping is kept for backward compatibility but now checks auth only (via /myself).
// Deprecated: Use CheckAuth() and CheckProjectAccess() for explicit validation.
func (j *JiraOffice) Ping(ctx context.Context) error {
	// For backward compatibility, check auth first
	if err := j.CheckAuth(ctx); err != nil {
		return err
	}

	// Then check project if configured
	if j.config.ProjectKey != "" {
		return j.CheckProjectAccess(ctx)
	}

	return nil
}

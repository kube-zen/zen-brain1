// Package secrets provides canonical secret resolution for office integrations.
package secrets

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveJira_ZenLockDir(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Create ZenLock-style files
	testURL := "https://test.atlassian.net"
	testEmail := "test@example.com"
	testToken := "test-token"
	testProject := "TEST"

	os.WriteFile(filepath.Join(tempDir, "JIRA_URL"), []byte(testURL), 0600)
	os.WriteFile(filepath.Join(tempDir, "JIRA_EMAIL"), []byte(testEmail), 0600)
	os.WriteFile(filepath.Join(tempDir, "JIRA_API_TOKEN"), []byte(testToken), 0600)
	os.WriteFile(filepath.Join(tempDir, "JIRA_PROJECT_KEY"), []byte(testProject), 0600)

	opts := JiraResolveOptions{
		DirPath:          tempDir,
		AllowEnvFallback: false,
	}

	material, err := ResolveJira(ctx, opts)
	if err != nil {
		t.Fatalf("ResolveJira failed: %v", err)
	}

	if material.BaseURL != testURL {
		t.Errorf("Expected BaseURL %s, got %s", testURL, material.BaseURL)
	}
	if material.Email != testEmail {
		t.Errorf("Expected Email %s, got %s", testEmail, material.Email)
	}
	if material.APIToken != testToken {
		t.Errorf("Expected APIToken %s, got %s", testToken, material.APIToken)
	}
	if material.ProjectKey != testProject {
		t.Errorf("Expected ProjectKey %s, got %s", testProject, material.ProjectKey)
	}
	if material.Source != "zenlock-dir:"+tempDir {
		t.Errorf("Expected Source %s, got %s", "zenlock-dir:"+tempDir, material.Source)
	}
}

func TestResolveJira_HostFileStringData(t *testing.T) {
	ctx := context.Background()
	tempFile := t.TempDir() + "/jira.yaml"

	// Create host file with stringData format
	yamlContent := `stringData:
  JIRA_URL: "https://test.atlassian.net"
  JIRA_EMAIL: "test@example.com"
  JIRA_API_TOKEN: "test-token"
  JIRA_PROJECT_KEY: "TEST"
`
	os.WriteFile(tempFile, []byte(yamlContent), 0600)

	opts := JiraResolveOptions{
		FilePath:         tempFile,
		AllowEnvFallback: false,
	}

	material, err := ResolveJira(ctx, opts)
	if err != nil {
		t.Fatalf("ResolveJira failed: %v", err)
	}

	if material.Source != "host-file:"+tempFile {
		t.Fatalf("Expected Source %s, got %s", "host-file:"+tempFile, material.Source)
	}
	if material.BaseURL != "https://test.atlassian.net" {
		t.Errorf("Expected BaseURL %s, got %s", "https://test.atlassian.net", material.BaseURL)
	}
	if material.Email != "test@example.com" {
		t.Errorf("Expected Email %s, got %s", "test@example.com", material.Email)
	}
	if material.APIToken != "test-token" {
		t.Errorf("Expected APIToken %s, got %s", "test-token", material.APIToken)
	}
	if material.ProjectKey != "TEST" {
		t.Errorf("Expected ProjectKey %s, got %s", "TEST", material.ProjectKey)
	}
}

func TestResolveJira_HostFileFlatKeys(t *testing.T) {
	ctx := context.Background()
	tempFile := t.TempDir() + "/jira.yaml"

	// Create host file with flat YAML keys
	yamlContent := `JIRA_URL: "https://test.atlassian.net"
JIRA_EMAIL: "test@example.com"
JIRA_API_TOKEN: "test-token"
JIRA_PROJECT_KEY: "TEST"
`
	os.WriteFile(tempFile, []byte(yamlContent), 0600)

	opts := JiraResolveOptions{
		FilePath:         tempFile,
		AllowEnvFallback: false,
	}

	material, err := ResolveJira(ctx, opts)
	if err != nil {
		t.Fatalf("ResolveJira failed: %v", err)
	}

	if material.BaseURL != "https://test.atlassian.net" {
		t.Errorf("Expected BaseURL %s, got %s", "https://test.atlassian.net", material.BaseURL)
	}
}

func TestResolveJira_EnvFallbackEnabled(t *testing.T) {
	ctx := context.Background()

	// Set env vars
	os.Setenv("JIRA_URL", "https://env.atlassian.net")
	os.Setenv("JIRA_EMAIL", "env@example.com")
	os.Setenv("JIRA_API_TOKEN", "env-token")
	os.Setenv("JIRA_PROJECT_KEY", "ENVPROJ")
	defer func() {
		os.Unsetenv("JIRA_URL")
		os.Unsetenv("JIRA_EMAIL")
		os.Unsetenv("JIRA_API_TOKEN")
		os.Unsetenv("JIRA_PROJECT_KEY")
	}()

	opts := JiraResolveOptions{
		AllowEnvFallback: true,
	}

	material, err := ResolveJira(ctx, opts)
	if err != nil {
		t.Fatalf("ResolveJira failed: %v", err)
	}

	if material.BaseURL != "https://env.atlassian.net" {
		t.Errorf("Expected BaseURL %s, got %s", "https://env.atlassian.net", material.BaseURL)
	}
	if material.Source != "env" {
		t.Errorf("Expected Source env, got %s", material.Source)
	}
}

func TestResolveJira_EnvFallbackDisabled(t *testing.T) {
	ctx := context.Background()

	// Set env vars
	os.Setenv("JIRA_URL", "https://env.atlassian.net")
	os.Setenv("JIRA_EMAIL", "env@example.com")
	os.Setenv("JIRA_API_TOKEN", "env-token")
	os.Setenv("JIRA_PROJECT_KEY", "ENVPROJ")
	defer func() {
		os.Unsetenv("JIRA_URL")
		os.Unsetenv("JIRA_EMAIL")
		os.Unsetenv("JIRA_API_TOKEN")
		os.Unsetenv("JIRA_PROJECT_KEY")
	}()

	opts := JiraResolveOptions{
		AllowEnvFallback: false, // Disabled by default
	}

	material, err := ResolveJira(ctx, opts)
	if err != nil {
		t.Fatalf("ResolveJira failed: %v", err)
	}

	if material.Source != "none" {
		t.Errorf("Expected Source none (env fallback disabled), got %s", material.Source)
	}
	if material.BaseURL != "" {
		t.Errorf("Expected empty BaseURL (env fallback disabled), got %s", material.BaseURL)
	}
}

func TestResolveJira_SourcePriority(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Create ZenLock directory
	os.WriteFile(filepath.Join(tempDir, "JIRA_URL"), []byte("https://zenlock.atlassian.net"), 0600)
	os.WriteFile(filepath.Join(tempDir, "JIRA_API_TOKEN"), []byte("zenlock-token"), 0600)

	// Also set env vars
	os.Setenv("JIRA_URL", "https://env.atlassian.net")
	os.Setenv("JIRA_API_TOKEN", "env-token")
	defer func() {
		os.Unsetenv("JIRA_URL")
		os.Unsetenv("JIRA_API_TOKEN")
	}()

	// Test: ZenLock dir should take priority over env
	opts := JiraResolveOptions{
		DirPath:          tempDir,
		AllowEnvFallback: true,
	}

	material, err := ResolveJira(ctx, opts)
	if err != nil {
		t.Fatalf("ResolveJira failed: %v", err)
	}

	if material.BaseURL != "https://zenlock.atlassian.net" {
		t.Errorf("ZenLock dir should have priority. Expected BaseURL %s, got %s", "https://zenlock.atlassian.net", material.BaseURL)
	}
	if material.APIToken != "zenlock-token" {
		t.Errorf("ZenLock dir should have priority. Expected APIToken %s, got %s", "zenlock-token", material.APIToken)
	}
	if material.Source != "zenlock-dir:"+tempDir {
		t.Errorf("Expected Source %s, got %s", "zenlock-dir:"+tempDir, material.Source)
	}
}

func TestResolveJira_EmptyCredentials(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Create ZenLock directory but without credentials
	opts := JiraResolveOptions{
		DirPath:          tempDir,
		AllowEnvFallback: false,
	}

	material, err := ResolveJira(ctx, opts)
	if err != nil {
		t.Fatalf("ResolveJira failed: %v", err)
	}

	if material.Source != "none" {
		t.Errorf("Expected Source none, got %s", material.Source)
	}
	if material.BaseURL != "" || material.APIToken != "" {
		t.Errorf("Expected empty credentials, got BaseURL=%s, APIToken=%s", material.BaseURL, material.APIToken)
	}
}

func TestResolveJira_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tempDir := t.TempDir()
	opts := JiraResolveOptions{
		DirPath:          tempDir,
		AllowEnvFallback: false,
	}

	material, err := ResolveJira(ctx, opts)
	if err != ctx.Err() {
		t.Errorf("Expected context cancellation error, got %v", err)
	}
	if material != nil && material.Source != "none" {
		t.Errorf("Expected no credentials on cancellation, got %v", material)
	}
}

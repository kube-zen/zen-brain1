package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	// Read from ZenLock secrets
	jiraURL, err := os.ReadFile("/zen-lock/secrets/JIRA_URL")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to read JIRA_URL: %v\n", err)
		os.Exit(1)
	}

	jiraEmail, err := os.ReadFile("/zen-lock/secrets/JIRA_EMAIL")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to read JIRA_EMAIL: %v\n", err)
		os.Exit(1)
	}

	jiraToken, err := os.ReadFile("/zen-lock/secrets/JIRA_API_TOKEN")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to read JIRA_API_TOKEN: %v\n", err)
		os.Exit(1)
	}

	jiraProjectKey, err := os.ReadFile("/zen-lock/secrets/JIRA_PROJECT_KEY")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to read JIRA_PROJECT_KEY: %v\n", err)
		os.Exit(1)
	}

	// Trim whitespace
	urlStr := string(jiraURL)
	email := string(jiraEmail)
	token := string(jiraToken)
	projectKey := string(jiraProjectKey)

	// Remove newlines
	for i := 0; i < len(urlStr); i++ {
		if urlStr[i] == '\n' || urlStr[i] == '\r' {
			urlStr = urlStr[:i]
			break
		}
	}
	for i := 0; i < len(email); i++ {
		if email[i] == '\n' || email[i] == '\r' {
			email = email[:i]
			break
		}
	}
	for i := 0; i < len(token); i++ {
		if token[i] == '\n' || token[i] == '\r' {
			token = token[:i]
			break
		}
	}
	for i := 0; i < len(projectKey); i++ {
		if projectKey[i] == '\n' || projectKey[i] == '\r' {
			projectKey = projectKey[:i]
			break
		}
	}

	fmt.Println("=== Jira Project Access Test ===")
	fmt.Printf("URL: %s\n", urlStr)
	fmt.Printf("Email: %s\n", email)
	fmt.Printf("Token length: %d\n", len(token))
	fmt.Printf("Project Key: %s\n\n", projectKey)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test /myself
	fmt.Println("TEST 1: GET /rest/api/3/myself")
	req1, _ := http.NewRequestWithContext(ctx, "GET", urlStr+"/rest/api/3/myself", nil)
	req1.SetBasicAuth(email, token)
	req1.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp1, err := client.Do(req1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp1.Body.Close()

	body1, _ := io.ReadAll(resp1.Body)
	fmt.Printf("HTTP Status: %d\n", resp1.StatusCode)
	if resp1.StatusCode == 200 {
		fmt.Println("Result: ✅ AUTH PASS")
	} else {
		fmt.Println("Result: ❌ AUTH FAIL")
		fmt.Printf("Response: %s\n", string(body1)[:min(200, len(body1))])
	}
	fmt.Println()

	// Test /project/{key}
	fmt.Printf("TEST 2: GET /rest/api/3/project/%s\n", projectKey)
	req2, _ := http.NewRequestWithContext(ctx, "GET", urlStr+"/rest/api/3/project/"+projectKey, nil)
	req2.SetBasicAuth(email, token)
	req2.Header.Set("Accept", "application/json")

	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	fmt.Printf("HTTP Status: %d\n", resp2.StatusCode)
	if resp2.StatusCode == 200 {
		fmt.Println("Result: ✅ PROJECT PASS")
		fmt.Printf("Response (first 300 chars): %s\n", string(body2)[:min(300, len(body2))])
	} else {
		fmt.Println("Result: ❌ PROJECT FAIL")
		fmt.Printf("Response: %s\n", string(body2)[:min(200, len(body2))])
	}
	fmt.Println()

	// Test /project/search
	fmt.Println("TEST 3: GET /rest/api/3/project/search")
	req3, _ := http.NewRequestWithContext(ctx, "GET", urlStr+"/rest/api/3/project/search", nil)
	req3.SetBasicAuth(email, token)
	req3.Header.Set("Accept", "application/json")

	resp3, err := client.Do(req3)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp3.Body.Close()

	body3, _ := io.ReadAll(resp3.Body)
	fmt.Printf("HTTP Status: %d\n", resp3.StatusCode)
	if resp3.StatusCode == 200 {
		fmt.Println("Result: ✅ PROJECT SEARCH PASS")
		fmt.Printf("Response (first 300 chars): %s\n", string(body3)[:min(300, len(body3))])
	} else {
		fmt.Println("Result: ❌ PROJECT SEARCH FAIL")
		fmt.Printf("Response: %s\n", string(body3)[:min(200, len(body3))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

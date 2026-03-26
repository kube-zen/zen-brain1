package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func testAuth(urlStr, email, token string) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	myselfURL := urlStr + "/rest/api/3/myself"
	req, err := http.NewRequestWithContext(ctx, "GET", myselfURL, nil)
	if err != nil {
		return 0, fmt.Sprintf("ERROR: Failed to create request: %v", err)
	}

	req.SetBasicAuth(email, token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Sprintf("ERROR: Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	response := string(body)
	if len(response) > 200 {
		response = response[:200] + "..."
	}

	return resp.StatusCode, response
}

func main() {
	// Read from ZenLock secrets
	jiraURL, err := os.ReadFile("/zen-lock/secrets/JIRA_URL")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to read JIRA_URL: %v\n", err)
		os.Exit(1)
	}

	jiraToken, err := os.ReadFile("/zen-lock/secrets/JIRA_API_TOKEN")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to read JIRA_API_TOKEN: %v\n", err)
		os.Exit(1)
	}

	// Trim whitespace
	urlStr := string(jiraURL)
	token := string(jiraToken)

	// Remove newlines
	for i := 0; i < len(urlStr); i++ {
		if urlStr[i] == '\n' || urlStr[i] == '\r' {
			urlStr = urlStr[:i]
			break
		}
	}
	for i := 0; i < len(token); i++ {
		if token[i] == '\n' || token[i] == '\r' {
			token = token[:i]
			break
		}
	}

	fmt.Println("=== Jira Auth Identity Test ===")
	fmt.Printf("URL: %s\n", urlStr)
	fmt.Printf("Token length: %d\n\n", len(token))

	// Test both emails
	emails := []string{
		"zen@kube-zen.io",
		"zen@kube-zen.io",
	}

	results := make(map[string]struct {
		status int
		response string
	})

	for _, email := range emails {
		fmt.Printf("Testing: %s\n", email)
		status, response := testAuth(urlStr, email, token)
		results[email] = struct {
			status int
			response string
		}{status, response}

		fmt.Printf("  HTTP Status: %d\n", status)
		if status == 200 {
			fmt.Printf("  Result: ✅ PASS\n")
		} else {
			fmt.Printf("  Result: ❌ FAIL\n")
			fmt.Printf("  Response: %s\n", response)
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("=== Summary ===")
	for _, email := range emails {
		r := results[email]
		status := "FAIL"
		if r.status == 200 {
			status = "PASS"
		}
		fmt.Printf("%s: %s (HTTP %d)\n", email, status, r.status)
	}
}

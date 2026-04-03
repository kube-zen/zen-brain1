package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// Minimal Jira client for testing
type JiraClient struct {
	URL   string
	Email string
	Token string
}

type JiraIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Labels      []string        `json:"labels"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
	} `json:"fields"`
}

func main() {
	// Load credentials from environment or secret
	jiraURL := os.Getenv("JIRA_URL")
	jiraEmail := os.Getenv("JIRA_EMAIL")
	jiraToken := os.Getenv("JIRA_API_TOKEN")

	if jiraURL == "" || jiraEmail == "" || jiraToken == "" {
		log.Fatal("Set JIRA_URL, JIRA_EMAIL, and JIRA_API_TOKEN environment variables")
	}

	client := &JiraClient{
		URL:   jiraURL,
		Email: jiraEmail,
		Token: jiraToken,
	}

	// Fetch backlog tickets
	fmt.Println("=== Fetching backlog tickets from Jira ===")
	issues, err := client.Search("project=ZB AND status=Backlog AND labels=bug ORDER BY priority DESC", 20)
	if err != nil {
		log.Fatalf("Failed to fetch tickets: %v", err)
	}

	fmt.Printf("Found %d tickets\n\n", len(issues))

	// Classify each ticket
	classification := map[string][]string{
		"ready_l1_now":      []string{},
		"auto_normalizable": []string{},
		"needs_human_context": []string{},
		"unsafe_for_autofix": []string{},
	}

	for _, issue := range issues {
		fmt.Printf("Processing %s: %s\n", issue.Key, issue.Fields.Summary[:min(60, len(issue.Fields.Summary))])
		
		// Extract description text from ADF
		description := extractTextFromADF(issue.Fields.Description)
		
		// Infer bounded scope
		hasFile := strings.Contains(description, ".go") || 
			strings.Contains(description, ".md") ||
			strings.Contains(description, ".yaml") ||
			strings.Contains(description, "file:") ||
			strings.Contains(description, "internal/") ||
			strings.Contains(description, "cmd/")
		
		isDocs := strings.Contains(strings.ToLower(description), "docs/") || 
			strings.Contains(strings.ToLower(issue.Fields.Summary), "doc")
		
		isLowRisk := isDocs || hasFile
		
		// Classify
		if hasFile && isLowRisk {
			classification["ready_l1_now"] = append(classification["ready_l1_now"], issue.Key)
		} else if hasFile {
			classification["auto_normalizable"] = append(classification["auto_normalizable"], issue.Key)
		} else {
			classification["needs_human_context"] = append(classification["needs_human_context"], issue.Key)
		}
	}

	// Print classification report
	fmt.Println("\n=== CLASSIFICATION REPORT ===")
	fmt.Printf("ready_l1_now: %d\n", len(classification["ready_l1_now"]))
	for _, key := range classification["ready_l1_now"] {
		fmt.Printf("  - %s\n", key)
	}
	
	fmt.Printf("\nauto_normalizable: %d\n", len(classification["auto_normalizable"]))
	for _, key := range classification["auto_normalizable"] {
		fmt.Printf("  - %s\n", key)
	}
	
	fmt.Printf("\nneeds_human_context: %d\n", len(classification["needs_human_context"]))
	for _, key := range classification["needs_human_context"] {
		fmt.Printf("  - %s\n", key)
	}
	
	fmt.Printf("\nunsafe_for_autofix: %d\n", len(classification["unsafe_for_autofix"]))
	for _, key := range classification["unsafe_for_autofix"] {
		fmt.Printf("  - %s\n", key)
	}

	// Generate execution packet for first ready ticket
	if len(classification["ready_l1_now"]) > 0 {
		targetKey := classification["ready_l1_now"][0]
		fmt.Printf("\n=== GENERATING EXECUTION PACKET FOR %s ===\n", targetKey)
		
		// Fetch full ticket details
		issue, err := client.GetIssue(targetKey)
		if err != nil {
			log.Fatalf("Failed to fetch %s: %v", targetKey, err)
		}
		
		description := extractTextFromADF(issue.Fields.Description)
		
		// Infer file paths
		files := inferTargetFiles(issue.Fields.Summary, description)
		fmt.Printf("\nInferred target files:\n")
		for _, f := range files {
			fmt.Printf("  - %s (confidence: %.2f, reason: %s)\n", f.Path, f.Confidence, f.Reason)
		}
		
		// Generate execution packet
		packet := generateExecutionPacket(issue, files)
		
		fmt.Printf("\nGenerated Execution Packet:\n")
		fmt.Println(packet)
	}
}

func (c *JiraClient) Search(jql string, maxResults int) ([]JiraIssue, error) {
	url := c.URL + "/rest/api/3/search/jql"
	
	payload := map[string]interface{}{
		"jql":        jql,
		"maxResults": maxResults,
		"fields":     []string{"key", "summary", "description", "labels", "status", "priority"},
	}
	
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.SetBasicAuth(c.Email, c.Token)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes[:min(200, len(bodyBytes))]))
	}
	
	var result struct {
		Issues []JiraIssue `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result.Issues, nil
}

func (c *JiraClient) GetIssue(key string) (*JiraIssue, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=key,summary,description,labels,status,priority", c.URL, key)
	
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(c.Email, c.Token)
	req.Header.Set("Accept", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	var issue JiraIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}
	
	return &issue, nil
}

func extractTextFromADF(adf json.RawMessage) string {
	// Simple ADF extraction - recursively extract "text" fields
	var parse func(interface{}) string
	parse = func(v interface{}) string {
		var sb strings.Builder
		
		switch val := v.(type) {
		case string:
			return val
		case []interface{}:
			for _, item := range val {
				sb.WriteString(parse(item))
			}
		case map[string]interface{}:
			if text, ok := val["text"].(string); ok {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
			if content, ok := val["content"]; ok {
				sb.WriteString(parse(content))
				sb.WriteString("\n")
			}
		}
		
		return sb.String()
	}
	
	var parsed interface{}
	if err := json.Unmarshal(adf, &parsed); err != nil {
		return string(adf)
	}
	
	return parse(parsed)
}

type TargetFile struct {
	Path       string
	Confidence float64
	Reason     string
}

func inferTargetFiles(summary, description string) []TargetFile {
	var files []TargetFile
	text := summary + " " + description
	
	// Simple file path extraction
	if strings.Contains(text, "internal/") {
		files = append(files, TargetFile{
			Path:       "internal/",
			Confidence: 0.80,
			Reason:     "Mentioned in ticket text",
		})
	}
	
	if strings.Contains(text, "cmd/") {
		files = append(files, TargetFile{
			Path:       "cmd/",
			Confidence: 0.80,
			Reason:     "Mentioned in ticket text",
		})
	}
	
	if strings.Contains(text, "docs/") {
		files = append(files, TargetFile{
			Path:       "docs/",
			Confidence: 0.90,
			Reason:     "Documentation change",
		})
	}
	
	return files
}

func generateExecutionPacket(issue *JiraIssue, files []TargetFile) string {
	// Generate YAML execution packet
	packet := fmt.Sprintf(`BOUNDED_EXECUTION:
  version: "1.0"
  target_files:
`)
	
	for _, f := range files {
		packet += fmt.Sprintf(`    - path: %s
      confidence: %.2f
      reason: %q
`, f.Path, f.Confidence, f.Reason)
	}
	
	packet += fmt.Sprintf(`
  evidence:
    type: ticket_description
    source: %s
    confidence: 0.75
  
  acceptance_criteria:
    - description: "File is modified and validated"
      validation_cmd: "go build ./..."
      confidence: 0.70
  
  scope:
    blast_radius: low
    execution_class: docs
    bounded: true
  
  inference_metadata:
    generated_by: auto_normalizer_v1
    overall_confidence: 0.78
`, issue.Key)
	
	return packet
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package main

import (
	"fmt"
	"log"
	"os"
)

// logStartupConfig logs the effective configuration at startup for observability.
func logStartupConfig(cfg interface {
	GetJiraCredentialsSource() string
	GetJiraBaseURL() string
	GetJiraEmail() string
	GetTier1RedisAddr() string
	GetLLMModel() string
	GetLLMTimeoutSeconds() int
}) {
	log.Println("=== FOREMAN STARTUP CONFIG ===")

	// Config load source
	configSource := "file"
	if os.Getenv("ZEN_BRAIN_CONFIG_FILE") != "" {
		configSource = "env:" + os.Getenv("ZEN_BRAIN_CONFIG_FILE")
	}
	log.Printf("  Config source: %s", configSource)

	// Jira credential source
	jiraCredSource := cfg.GetJiraCredentialsSource()
	log.Printf("  Jira credential source: %s", jiraCredSource)
	log.Printf("  Jira URL: %s", cfg.GetJiraBaseURL())
	log.Printf("  Jira email: %s", cfg.GetJiraEmail())

	// Tier1 Redis
	tier1Redis := cfg.GetTier1RedisAddr()
	if tier1Redis == "" {
		tier1Redis = "(not configured)"
	}
	log.Printf("  Tier1 Redis: %s", tier1Redis)

	// Local LLM config
	model := cfg.GetLLMModel()
	if model == "" {
		model = "qwen3.5:0.8b"
	}
	timeout := cfg.GetLLMTimeoutSeconds()
	if timeout == 0 {
		timeout = 2700
	}
	log.Printf("  Local LLM model: %s", model)
	log.Printf("  Local LLM timeout: %ds", timeout)

	// Keep-alive and stale threshold (from env vars)
	keepAlive := os.Getenv("ZEN_FOREMAN_LLM_KEEP_ALIVE")
	if keepAlive == "" {
		keepAlive = "45m"
	}
	staleThreshold := os.Getenv("ZEN_FOREMAN_STALE_THRESHOLD")
	if staleThreshold == "" {
		staleThreshold = "60m"
	}
	log.Printf("  Keep-alive: %s", keepAlive)
	log.Printf("  Stale threshold: %s", staleThreshold)

	log.Println("===============================")
}

// stubConfigLoader provides interface for testing
type stubConfigLoader struct {
	jiraCredSource    string
	jiraBaseURL       string
	jiraEmail         string
	tier1RedisAddr    string
	llmModel          string
	llmTimeoutSeconds int
}

func (s *stubConfigLoader) GetJiraCredentialsSource() string { return s.jiraCredSource }
func (s *stubConfigLoader) GetJiraBaseURL() string           { return s.jiraBaseURL }
func (s *stubConfigLoader) GetJiraEmail() string             { return s.jiraEmail }
func (s *stubConfigLoader) GetTier1RedisAddr() string        { return s.tier1RedisAddr }
func (s *stubConfigLoader) GetLLMModel() string              { return s.llmModel }
func (s *stubConfigLoader) GetLLMTimeoutSeconds() int        { return s.llmTimeoutSeconds }

func main() {
	// Example usage
	cfg := &stubConfigLoader{
		jiraCredSource:    "zenlock-dir:/zen-lock/secrets",
		jiraBaseURL:       "https://zen-mesh.atlassian.net",
		jiraEmail:         "zen@zen-mesh.io",
		tier1RedisAddr:    "zencontext-redis.zen-context.svc.cluster.local:6379",
		llmModel:          "qwen3.5:0.8b",
		llmTimeoutSeconds: 2700,
	}

	logStartupConfig(cfg)

	fmt.Println("Done")
	os.Exit(0)
}

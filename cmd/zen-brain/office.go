// Package main: office subcommands (doctor, search, fetch, watch).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/integration"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/office/jira"
)

func runOfficeCommand() {
	if len(os.Args) < 3 {
		printOfficeUsage()
		os.Exit(1)
	}
	sub := os.Args[2]
	switch sub {
	case "doctor":
		runOfficeDoctor()
	case "search":
		if len(os.Args) < 4 {
			fmt.Println("Usage: zen-brain office search <query>")
			os.Exit(1)
		}
		runOfficeSearch(os.Args[3])
	case "fetch":
		if len(os.Args) < 4 {
			fmt.Println("Usage: zen-brain office fetch <jira-key>")
			os.Exit(1)
		}
		runOfficeFetch(os.Args[3])
	case "watch":
		runOfficeWatch()
	default:
		fmt.Printf("Unknown office subcommand: %s\n", sub)
		printOfficeUsage()
		os.Exit(1)
	}
}

func printOfficeUsage() {
	fmt.Println("Usage: zen-brain office <subcommand> [options]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  doctor         Print config source, connectors, cluster mapping, Jira URL, project, webhook, credentials, API reachability")
	fmt.Println("  search <query> Search work items (JQL or plain text); prints key, title, status, work type, priority")
	fmt.Println("  fetch <key>    Fetch one item by Jira key; prints canonical mapping")
	fmt.Println("  watch          Start Jira webhook listener and stream events until interrupted")
}

func runOfficeDoctor() {
	fmt.Println("=== Office Doctor ===")
	cfg, cfgErr := config.LoadConfig("")
	if cfgErr != nil {
		fmt.Printf("Config: failed to load (%v)\n", cfgErr)
	} else {
		fmt.Println("Config: loaded from file/env")
	}

	mgr, err := integration.InitOfficeManagerFromConfig(cfg)
	if err != nil {
		fmt.Printf("Office manager: init failed: %v\n", err)
		return
	}
	if cfg == nil || !cfg.Jira.Enabled {
		mgr = office.NewManager()
		// Try env fallback
		conn, _ := jira.NewFromEnv("jira", "default")
		if conn != nil {
			_ = mgr.Register("jira", conn)
			_ = mgr.RegisterForCluster("default", "jira")
		}
	}

	connectors := mgr.ListConnectors()
	fmt.Printf("Connectors: %s\n", strings.Join(connectors, ", "))
	if len(connectors) == 0 {
		fmt.Println("Cluster mapping: (none)")
		fmt.Println("Jira: not configured")
		return
	}
	fmt.Println("Cluster mapping: default -> jira")

	conn, err := mgr.GetConnectorForCluster("default")
	if err != nil {
		fmt.Printf("Default connector: %v\n", err)
		return
	}
	jiraConn, ok := conn.(*jira.JiraOffice)
	if !ok {
		fmt.Println("Default connector is not Jira; doctor only supports Jira")
		return
	}

	// Sanitized base URL (no credentials)
	baseURL := jiraConn.Config().BaseURL
	if baseURL == "" {
		baseURL = "(not set)"
	}
	fmt.Printf("Jira base URL: %s\n", baseURL)
	fmt.Printf("Project key: %s\n", jiraConn.Config().ProjectKey)
	webhookEnabled := jiraConn.Config().WebhookPath != "" || jiraConn.Config().WebhookPort > 0
	fmt.Printf("Webhook: enabled=%v, path=%s, port=%d\n",
		webhookEnabled, jiraConn.Config().WebhookPath, jiraConn.Config().WebhookPort)
	credsPresent := jiraConn.Config().APIToken != "" && jiraConn.Config().Email != ""
	fmt.Printf("Credentials: present=%v\n", credsPresent)

	if err := jiraConn.ValidateConfig(); err != nil {
		fmt.Printf("ValidateConfig: %v\n", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := jiraConn.Ping(ctx); err != nil {
		fmt.Printf("API reachability: failed (%v)\n", err)
	} else {
		fmt.Println("API reachability: ok")
	}
}

func runOfficeSearch(query string) {
	mgr, err := getOfficeManager()
	if err != nil {
		log.Fatalf("Office: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	items, err := mgr.Search(ctx, "default", query)
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}
	fmt.Printf("Found %d item(s):\n", len(items))
	for _, w := range items {
		fmt.Printf("  %s  %s  status=%s  type=%s  priority=%s\n",
			w.ID, truncate(w.Title, 40), w.Status, w.WorkType, w.Priority)
	}
}

func runOfficeFetch(jiraKey string) {
	mgr, err := getOfficeManager()
	if err != nil {
		log.Fatalf("Office: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	item, err := mgr.Fetch(ctx, "default", jiraKey)
	if err != nil {
		log.Fatalf("Fetch failed: %v", err)
	}
	fmt.Println("ID:", item.ID)
	fmt.Println("Title:", item.Title)
	fmt.Println("Status:", item.Status)
	fmt.Println("Work type:", item.WorkType)
	fmt.Println("Work domain:", item.WorkDomain)
	fmt.Println("Source metadata:", fmt.Sprintf("%+v", item.Source))
	fmt.Println("Tags:", item.Tags)
}

func runOfficeWatch() {
	mgr, err := getOfficeManager()
	if err != nil {
		log.Fatalf("Office: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := mgr.Watch(ctx, "default")
	if err != nil {
		log.Fatalf("Watch failed: %v", err)
	}
	conn, _ := mgr.GetConnectorForCluster("default")
	jiraConn, _ := conn.(*jira.JiraOffice)
	if jiraConn != nil {
		fmt.Printf("Webhook listening on path=%s port=%d\n", jiraConn.Config().WebhookPath, jiraConn.Config().WebhookPort)
	}
	fmt.Println("Streaming events (Ctrl+C to stop)...")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case e := <-ch:
			if e.WorkItem != nil {
				fmt.Printf("Event: %s  %s  %s\n", e.Type, e.WorkItem.ID, truncate(e.WorkItem.Title, 50))
			} else {
				fmt.Printf("Event: %s\n", e.Type)
			}
		case <-sig:
			fmt.Println("Stopping...")
			return
		}
	}
}

func getOfficeManager() (*office.Manager, error) {
	cfg, _ := config.LoadConfig("")
	if cfg != nil && cfg.Jira.Enabled {
		return integration.InitOfficeManagerFromConfig(cfg)
	}
	mgr := office.NewManager()
	conn, err := jira.NewFromEnv("jira", "default")
	if err != nil {
		return nil, err
	}
	if err := mgr.Register("jira", conn); err != nil {
		return nil, err
	}
	if err := mgr.RegisterForCluster("default", "jira"); err != nil {
		return nil, err
	}
	return mgr, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
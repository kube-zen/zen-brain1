package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kube-zen/shared/logging"

	// Import policy configuration
	"github.com/kube-zen/zen-brain1/src/config/policy"
)

var (
	// Version set at build time
	Version   = "1.0.0"
	BuildSHA  = "dev"
	BuildTime = "unknown"
)

func main() {
	// Setup context
	ctx := context.Background()

	// Setup logging
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	// Initialize logging
	logging.Init(logging.Config{
		Level:        logLevel,
		Format:       "json",
		Output:       os.Stdout,
		ServiceName:  "zen-brain",
		ServiceType:  "ai-brain",
	})

	// Log startup
	logging.Info(ctx, "zen-brain starting",
		logging.String("version", Version),
		logging.String("build_sha", BuildSHA),
		logging.String("build_time", BuildTime),
	)

	// Load policy configuration
	configDir := os.Getenv("POLICY_CONFIG_DIR")
	if configDir == "" {
		configDir = "./config/policy/"
	}

	logging.Info(ctx, "Loading policy configuration from "+configDir,
		logging.Op("policy_load"),
	)

	policyConfig, errors := policy.LoadConfig(configDir)
	if len(errors) > 0 {
		logging.Error(ctx, "Policy configuration failed to load", fmt.Errorf("%d validation errors found", len(errors)),
			logging.Op("policy_load"),
			logging.ErrorCode("POLICY_LOAD_FAILED"),
		)
		for _, err := range errors {
			logging.Error(ctx, "Policy configuration error", err,
				logging.Op("policy_load"),
			)
		}
		os.Exit(1)
	}

	logging.Info(ctx, "Policy configuration loaded successfully",
		logging.Op("policy_load"),
		logging.Int("roles_loaded", len(policyConfig.Roles)),
		logging.Int("tasks_loaded", len(policyConfig.Tasks)),
		logging.Int("providers_loaded", len(policyConfig.Providers)),
		logging.Int("chains_loaded", len(policyConfig.Chains)),
	)

	// Get default role
	defaultRole := policyConfig.GetDefaultRole()
	if defaultRole != nil {
		logging.Info(ctx, "Default role set to "+defaultRole.Name,
			logging.Op("policy_config"),
			logging.String("default_provider", defaultRole.DefaultProvider),
		)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// TODO: Start HTTP server, AI providers, metrics, etc.
	// This is a placeholder for integration work

	logging.Info(ctx, "zen-brain ready",
		logging.Op("startup"),
		logging.String("version", Version),
		logging.String("build_sha", BuildSHA),
		logging.Int("policy_roles", len(policyConfig.Roles)),
		logging.Int("policy_tasks", len(policyConfig.Tasks)),
		logging.Int("policy_providers", len(policyConfig.Providers)),
	)

	// Wait for shutdown signal
	<-sigChan

	logging.Info(ctx, "Shutting down gracefully",
		logging.Op("shutdown"),
	)

	// TODO: Cleanup resources, close connections, etc.

	logging.Info(ctx, "zen-brain shutdown complete",
		logging.Op("shutdown"),
	)
}

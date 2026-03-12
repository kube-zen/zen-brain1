package runtime

import (
	"os"
	"strings"

	"github.com/kube-zen/zen-brain1/internal/config"
)

// Requirements holds which Block 3 capabilities are required (fail‑closed).
type Requirements struct {
	ZenContext bool
	QMD        bool
	Ledger     bool
	MessageBus bool
}

// GetRequirements returns the required‑capability matrix based on environment
// variables and config fields.
func GetRequirements(cfg *config.Config) Requirements {
	var req Requirements

	// Global strict profile
	if IsStrictProfile() {
		req.ZenContext = true
		req.QMD = true
		req.Ledger = true
		req.MessageBus = true
	}

	// Individual require flags
	if os.Getenv("ZEN_BRAIN_REQUIRE_ZENCONTEXT") != "" {
		req.ZenContext = true
	}
	if os.Getenv("ZEN_BRAIN_REQUIRE_QMD") != "" {
		req.QMD = true
	}
	if os.Getenv("ZEN_BRAIN_REQUIRE_LEDGER") != "" {
		req.Ledger = true
	}
	if os.Getenv("ZEN_BRAIN_REQUIRE_MESSAGEBUS") != "" {
		req.MessageBus = true
	}

	// Config overrides (config.Required fields)
	if cfg != nil {
		if cfg.ZenContext.Required {
			req.ZenContext = true
		}
		if cfg.Ledger.Required {
			req.Ledger = true
		}
		if cfg.MessageBus.Required {
			req.MessageBus = true
		}
		// QMD required if ZenContext.Required and Tier2QMD.RepoPath set?
		// For now, QMD is only required by explicit env.
	}

	return req
}

// IsStrictProfile returns true if the current runtime profile is prod or staging,
// or if ZEN_BRAIN_STRICT_RUNTIME is set.
func IsStrictProfile() bool {
	profile := detectRuntimeProfile()
	return profile == "prod" || profile == "staging" || os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != ""
}

// detectRuntimeProfile determines the runtime profile from environment.
// It is copied from preflight_enhanced.go to avoid import cycles.
func detectRuntimeProfile() string {
	// Explicit profile
	if profile := os.Getenv("ZEN_RUNTIME_PROFILE"); profile != "" {
		return strings.ToLower(profile)
	}

	// Strict mode implies prod
	if os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" {
		return "prod"
	}

	// Environment‑based detection
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		if os.Getenv("ZEN_BRAIN_ENV") == "production" {
			return "prod"
		}
		return "staging"
	}

	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return "ci"
	}

	if os.Getenv("GO_TEST") != "" {
		return "test"
	}

	return "dev"
}

// ShouldFailClosed returns true if the given component is required and must be healthy.
// component is one of: zen_context, qmd, ledger, message_bus.
func ShouldFailClosed(component string, cfg *config.Config) bool {
	req := GetRequirements(cfg)
	switch component {
	case "zen_context":
		return req.ZenContext
	case "qmd":
		return req.QMD
	case "ledger":
		return req.Ledger
	case "message_bus":
		return req.MessageBus
	default:
		return false
	}
}
package runtime

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/kube-zen/zen-brain1/internal/config"
	internalcontext "github.com/kube-zen/zen-brain1/internal/context"
	"github.com/kube-zen/zen-brain1/internal/context/tier1"
	"github.com/kube-zen/zen-brain1/internal/context/tier3"
	internalLedger "github.com/kube-zen/zen-brain1/internal/ledger"
	"github.com/kube-zen/zen-brain1/internal/messagebus/redis"
	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	ledger "github.com/kube-zen/zen-brain1/pkg/ledger"
	"github.com/kube-zen/zen-brain1/pkg/messagebus"
)

// Runtime holds the bootstrapped Block 3 components and their report.
type Runtime struct {
	ZenContext zenctx.ZenContext
	Ledger     ledger.ZenLedgerClient
	MessageBus messagebus.MessageBus
	Report     *RuntimeReport
}

// strictness reads env and config to determine if a capability is required.
// ZEN_RUNTIME_PROFILE=prod (or ZEN_BRAIN_STRICT_RUNTIME) makes all capabilities required (fail-closed).
func strictness(cfg *config.Config) (requireZenContext, requireQMD, requireLedger, requireMessageBus bool) {
	if os.Getenv("ZEN_RUNTIME_PROFILE") == "prod" || os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" {
		requireZenContext = true
		requireQMD = true
		requireLedger = true
		requireMessageBus = true
	}
	if os.Getenv("ZEN_BRAIN_REQUIRE_ZENCONTEXT") != "" {
		requireZenContext = true
	}
	if os.Getenv("ZEN_BRAIN_REQUIRE_QMD") != "" {
		requireQMD = true
	}
	if os.Getenv("ZEN_BRAIN_REQUIRE_LEDGER") != "" {
		requireLedger = true
	}
	if os.Getenv("ZEN_BRAIN_REQUIRE_MESSAGEBUS") != "" {
		requireMessageBus = true
	}
	if cfg != nil {
		if cfg.ZenContext.Required {
			requireZenContext = true
		}
		if cfg.Ledger.Required {
			requireLedger = true
		}
		if cfg.MessageBus.Required {
			requireMessageBus = true
		}
	}
	return requireZenContext, requireQMD, requireLedger, requireMessageBus
}

// configToZenContextConfig maps config.ZenContextConfig to internal/context.ZenContextConfig.
func configToZenContextConfig(c *config.ZenContextConfig) *internalcontext.ZenContextConfig {
	if c == nil {
		return nil
	}
	out := &internalcontext.ZenContextConfig{
		ClusterID: c.ClusterID,
		Verbose:   c.Verbose,
	}
	// Tier1 Redis
	out.Tier1Redis = &tier1.RedisConfig{
		Addr:         c.Tier1Redis.Addr,
		Password:     c.Tier1Redis.Password,
		DB:           c.Tier1Redis.DB,
		PoolSize:     c.Tier1Redis.PoolSize,
		MinIdleConns: c.Tier1Redis.MinIdleConns,
		DialTimeout:  c.Tier1Redis.DialTimeout,
		ReadTimeout:  c.Tier1Redis.ReadTimeout,
		WriteTimeout: c.Tier1Redis.WriteTimeout,
	}
	// FAIL CLOSED: Do not default to localhost:6379
	// Redis must be explicitly configured via TIER1_REDIS_ADDR or config file
	if out.Tier1Redis.Addr == "" {
		log.Printf("[Bootstrap] FAIL CLOSED: Tier1 Redis not configured - ZenContext disabled (set TIER1_REDIS_ADDR to enable)")
		// Return nil config to indicate disabled, not stub
		return nil
	}
	// Tier2 QMD
	out.Tier2QMD = &internalcontext.QMDConfig{
		RepoPath:      c.Tier2QMD.RepoPath,
		QMDBinaryPath: c.Tier2QMD.QMDBinaryPath,
		Verbose:       c.Tier2QMD.Verbose,
	}
	// Tier3 S3
	out.Tier3S3 = &tier3.S3Config{
		Bucket:            c.Tier3S3.Bucket,
		Region:            c.Tier3S3.Region,
		Endpoint:          c.Tier3S3.Endpoint,
		AccessKeyID:       c.Tier3S3.AccessKeyID,
		SecretAccessKey:   c.Tier3S3.SecretAccessKey,
		SessionToken:      c.Tier3S3.SessionToken,
		UsePathStyle:      c.Tier3S3.UsePathStyle,
		DisableSSL:        c.Tier3S3.DisableSSL,
		ForceRenameBucket: c.Tier3S3.ForceRenameBucket,
		MaxRetries:        c.Tier3S3.MaxRetries,
		Timeout:           c.Tier3S3.Timeout,
		PartSize:          c.Tier3S3.PartSize,
		Concurrency:       c.Tier3S3.Concurrency,
		Verbose:           c.Tier3S3.Verbose,
	}
	// Journal
	if c.Journal.JournalPath != "" {
		out.Journal = &internalcontext.JournalConfig{
			JournalPath:      c.Journal.JournalPath,
			EnableQueryIndex: c.Journal.EnableQueryIndex,
		}
	}
	return out
}

// Bootstrap builds Block 3 runtime from config and env, fills RuntimeReport, and returns error only when a required capability is unavailable.
func Bootstrap(ctx context.Context, cfg *config.Config) (*Runtime, error) {
	reqZC, reqQMD, reqLedger, reqMB := strictness(cfg)
	report := &RuntimeReport{}

	// ENHANCED PREFLIGHT: Run comprehensive preflight checks before initialization
	// In prod mode, this fails fast on missing critical services
	preflightReport, preflightErr := EnhancedStrictPreflight(ctx, cfg, report)
	if preflightErr != nil {
		return nil, fmt.Errorf("preflight checks failed: %w", preflightErr)
	}
	
	// Store preflight report for diagnostics
	// In prod mode, preflight failure already returned error
	// In dev mode, we continue even with warnings
	report.PreflightReport = preflightReport

	// 1) ZenContext from config
	zcConfig := configToZenContextConfig(&cfg.ZenContext)

	// FAIL CLOSED: If Redis required but not configured, fail immediately
	if zcConfig != nil && zcConfig.Tier1Redis != nil && zcConfig.Tier1Redis.Addr == "" {
		if reqZC || (cfg != nil && cfg.ZenContext.Required) {
			return nil, fmt.Errorf("tier1_redis.addr is required when zen_context is required (set TIER1_REDIS_ADDR)")
		}
		log.Printf("[Bootstrap] Warning: Tier1 Redis not configured, using stub/mode=stub")
	}

	zenContext, errZC := internalcontext.NewZenContext(zcConfig)
	if errZC != nil {
		report.ZenContext = CapabilityStatus{Name: "zen_context", Mode: ModeDegraded, Healthy: false, Required: reqZC, Message: errZC.Error()}
		report.Tier1Hot = CapabilityStatus{Name: "tier1_hot", Mode: ModeDegraded, Healthy: false, Required: reqZC, Message: errZC.Error()}
		report.Tier2Warm = CapabilityStatus{Name: "tier2_warm", Mode: ModeDisabled, Healthy: false, Required: reqQMD}
		report.Tier3Cold = CapabilityStatus{Name: "tier3_cold", Mode: ModeDisabled, Healthy: false}
		report.Journal = CapabilityStatus{Name: "journal", Mode: ModeDisabled, Healthy: false}
		if reqZC {
			return &Runtime{Report: report}, fmt.Errorf("zen_context required but init failed: %w", errZC)
		}
		// Leave ZenContext nil; caller may use mock
	} else {
		report.ZenContext = CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: reqZC}
		report.Tier1Hot = CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: reqZC}
		t2, t3, j := inferTier2Tier3Journal(zcConfig)
		report.Tier2Warm = t2
		report.Tier2Warm.Required = reqQMD
		report.Tier3Cold = t3
		report.Journal = j
		// Run health checks so Healthy reflects actual reachability
		if errHot := internalcontext.CheckHot(ctx, zenContext); errHot != nil {
			report.Tier1Hot.Healthy = false
			report.Tier1Hot.Message = errHot.Error()
			report.ZenContext.Healthy = false
			report.ZenContext.Message = errHot.Error()
		}
		if errWarm := internalcontext.CheckWarm(ctx, zenContext); errWarm != nil && report.Tier2Warm.Mode == ModeReal {
			report.Tier2Warm.Healthy = false
			report.Tier2Warm.Message = errWarm.Error()
		}
		if errCold := internalcontext.CheckCold(ctx, zenContext); errCold != nil && report.Tier3Cold.Mode == ModeReal {
			report.Tier3Cold.Healthy = false
			report.Tier3Cold.Message = errCold.Error()
		}
	}

	// 2) Ledger
	dsn := os.Getenv("ZEN_LEDGER_DSN")
	if dsn == "" {
		dsn = os.Getenv("LEDGER_DATABASE_URL")
	}
	if dsn == "" && cfg != nil && cfg.Ledger.Enabled && cfg.Ledger.Host != "" {
		dsn = buildLedgerDSN(&cfg.Ledger)
	}
	ledgerClient, errLedger := internalLedger.NewCockroachLedger(dsn)
	if errLedger != nil || ledgerClient == nil {
		// FAIL CLOSED: Never silently use stub ledger
		if reqLedger {
			msg := "ledger required but no DSN (set ZEN_LEDGER_DSN or LEDGER_DATABASE_URL) or init failed"
			if errLedger != nil {
				msg = fmt.Sprintf("ledger required but init failed: %v", errLedger)
			}
			report.Ledger = CapabilityStatus{Name: "ledger", Mode: ModeStub, Healthy: false, Required: true, Message: msg}
			return &Runtime{ZenContext: zenContext, Report: report}, fmt.Errorf("%s", msg)
		}
		// In non-strict mode, ledger is disabled (not stub)
		report.Ledger = CapabilityStatus{Name: "ledger", Mode: ModeDisabled, Healthy: false, Required: false, Message: "no ledger DSN configured"}
		ledgerClient = nil
	} else {
		report.Ledger = CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: reqLedger}
		if errPing := internalLedger.Ping(ctx, ledgerClient); errPing != nil {
			report.Ledger.Healthy = false
			report.Ledger.Message = errPing.Error()
		}
	}

	// 3) Message bus
	var msgBus messagebus.MessageBus
	if cfg != nil && cfg.MessageBus.Enabled && cfg.MessageBus.Kind == "redis" {
		redisURL := cfg.MessageBus.RedisURL
		if redisURL == "" {
			redisURL = os.Getenv("REDIS_URL")
		}
		// FAIL CLOSED: Require explicit Redis URL for message bus
		if redisURL == "" {
			if cfg.MessageBus.Required {
				return &Runtime{ZenContext: zenContext, Ledger: ledgerClient, Report: report}, fmt.Errorf("message_bus redis URL required (set MESSAGEBUS_REDIS_URL or REDIS_URL)")
			}
			report.MessageBus = CapabilityStatus{Name: "message_bus", Mode: ModeStub, Healthy: true, Required: cfg.MessageBus.Required, Message: "no redis URL configured"}
		} else {
			bus, errBus := redis.New(&redis.Config{RedisURL: redisURL})
			if errBus != nil {
				report.MessageBus = CapabilityStatus{Name: "message_bus", Mode: ModeDegraded, Healthy: false, Required: cfg.MessageBus.Required, Message: errBus.Error()}
				if cfg.MessageBus.Required {
					return &Runtime{ZenContext: zenContext, Ledger: ledgerClient, Report: report}, fmt.Errorf("message_bus required but init failed: %w", errBus)
				}
			} else {
				msgBus = bus
				report.MessageBus = CapabilityStatus{Name: "message_bus", Mode: ModeReal, Healthy: true, Required: cfg.MessageBus.Required}
			}
		}
	} else {
		report.MessageBus = CapabilityStatus{Name: "message_bus", Mode: ModeDisabled, Healthy: false, Required: reqMB}
	}

	return &Runtime{
		ZenContext: zenContext,
		Ledger:     ledgerClient,
		MessageBus: msgBus,
		Report:     report,
	}, nil
}

func buildLedgerDSN(c *config.LedgerConfig) string {
	if c.Host == "" {
		return ""
	}
	port := c.Port
	if port == 0 {
		port = 26257
	}
	db := c.Database
	if db == "" {
		db = "defaultdb"
	}
	user := c.User
	if user == "" {
		user = "root"
	}
	ssl := c.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	return fmt.Sprintf("postgres://%s@%s:%d/%s?sslmode=%s", user, c.Host, port, db, ssl)
}

func inferTier2Tier3Journal(zc *internalcontext.ZenContextConfig) (tier2, tier3, journal CapabilityStatus) {
	tier2 = CapabilityStatus{Name: "tier2_warm", Mode: ModeDisabled, Healthy: false}
	if zc != nil && zc.Tier2QMD != nil && zc.Tier2QMD.RepoPath != "" {
		tier2.Mode = ModeReal
		tier2.Healthy = true
	}
	tier3 = CapabilityStatus{Name: "tier3_cold", Mode: ModeDisabled, Healthy: false}
	if zc != nil && zc.Tier3S3 != nil && zc.Tier3S3.Bucket != "" {
		tier3.Mode = ModeReal
		tier3.Healthy = true
	}
	journal = CapabilityStatus{Name: "journal", Mode: ModeDisabled, Healthy: false}
	if zc != nil && zc.Journal != nil && zc.Journal.JournalPath != "" {
		journal.Mode = ModeReal
		journal.Healthy = true
	}
	return tier2, tier3, journal
}

// Close releases resources held by the runtime. Caller may call Ledger.Close() and MessageBus.Close() separately if needed.
func (r *Runtime) Close() error {
	var errs []error
	if r.ZenContext != nil {
		if c, ok := r.ZenContext.(interface{ Close() error }); ok {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if r.MessageBus != nil {
		if err := r.MessageBus.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c, ok := r.Ledger.(interface{ Close() error }); ok && c != nil {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

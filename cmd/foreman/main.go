// Command foreman runs the Foreman controller (Block 4.2).
// It watches BrainTask resources and reconciles them (scheduling, status updates).
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/agent"
	"github.com/kube-zen/zen-brain1/internal/config"
	internalcontext "github.com/kube-zen/zen-brain1/internal/context"
	"github.com/kube-zen/zen-brain1/internal/evidence"
	"github.com/kube-zen/zen-brain1/internal/feedback"
	"github.com/kube-zen/zen-brain1/internal/foreman"
	"github.com/kube-zen/zen-brain1/internal/office/jira"
	"github.com/kube-zen/zen-brain1/internal/gate"
	internalguardian "github.com/kube-zen/zen-brain1/internal/guardian"
	internalledger "github.com/kube-zen/zen-brain1/internal/ledger"
	internalruntime "github.com/kube-zen/zen-brain1/internal/runtime"
	gatepkg "github.com/kube-zen/zen-brain1/pkg/gate"
	"github.com/kube-zen/zen-brain1/pkg/guardian"
	"github.com/kube-zen/zen-brain1/pkg/ledger"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func main() {
	log.Printf("[MAIN] Foreman binary starting")
	var metricsAddr, probeAddr string
	var numWorkers int
	var runtimeDir, workspaceHome string
	var preferRealTemplates bool
	var useGitWorktree bool
	var sourceRepoPath, worktreeBasePath, sourceRef string
	var reuseSessionWorktree bool
	var clusterID string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Address for metrics.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", "0.0.0.0:8081", "Address for health probes.")
	flag.IntVar(&numWorkers, "workers", 2, "Number of worker goroutines for task execution (Block 4.3).")
	flag.StringVar(&runtimeDir, "factory-runtime-dir", envStr("ZEN_FOREMAN_RUNTIME_DIR", ""), "Runtime dir for Factory workspaces and proof-of-work (FAIL CLOSED: no default).")
	flag.StringVar(&workspaceHome, "factory-workspace-home", envStr("ZEN_FOREMAN_WORKSPACE_HOME", ""), "Workspace home for Factory (workspaces created under <home>/workspaces) (FAIL CLOSED: no default).")
	flag.BoolVar(&preferRealTemplates, "factory-prefer-real-templates", envBool("ZEN_FOREMAN_PREFER_REAL_TEMPLATES", true), "Prefer real templates when workDomain is empty (implementation, docs, debug, refactor, review).")
	flag.BoolVar(&useGitWorktree, "factory-use-git-worktree", envBool("ZEN_FOREMAN_USE_GIT_WORKTREE", false), "Use real git worktrees from source repo (Block 4 real execution lane).")
	flag.StringVar(&sourceRepoPath, "factory-source-repo", envStr("ZEN_FOREMAN_SOURCE_REPO", ""), "Path to git repo (required if factory-use-git-worktree).")
	flag.StringVar(&worktreeBasePath, "factory-worktree-base", envStr("ZEN_FOREMAN_WORKTREE_BASE", ""), "Base path for git worktrees (default <runtime-dir>/worktrees).")
	flag.StringVar(&sourceRef, "factory-source-ref", envStr("ZEN_FOREMAN_SOURCE_REF", "HEAD"), "Git ref for worktree (e.g. HEAD, main).")
	flag.BoolVar(&reuseSessionWorktree, "factory-reuse-session-worktree", envBool("ZEN_FOREMAN_REUSE_SESSION_WORKTREE", false), "Reuse one worktree per session when using git worktrees.")
	// Factory LLM configuration (ZB-022G)
	var enableFactoryLLM bool
	var llmBaseURL, llmModel string
	var llmTimeoutSeconds int
	var llmEnableThinking bool
	flag.BoolVar(&enableFactoryLLM, "factory-enable-llm", envBool("ZEN_FOREMAN_ENABLE_LLM", false), "Enable LLM-powered Factory execution (ZB-022G).")
	flag.StringVar(&llmBaseURL, "factory-llm-base-url", envStr("ZEN_FOREMAN_LLM_BASE_URL", ""), "LLM endpoint for Factory (e.g. http://host.k3d.internal:11434).")
	flag.StringVar(&llmModel, "factory-llm-model", envStr("ZEN_FOREMAN_LLM_MODEL", "qwen3.5:0.8b"), "LLM model for Factory (default qwen3.5:0.8b for CPU inference).")
	flag.IntVar(&llmTimeoutSeconds, "factory-llm-timeout-seconds", envInt("ZEN_FOREMAN_LLM_TIMEOUT_SECONDS", 2700), "LLM request timeout in seconds (default 2700s=45m for CPU path).")
	flag.BoolVar(&llmEnableThinking, "factory-llm-enable-thinking", envBool("ZEN_FOREMAN_LLM_ENABLE_THINKING", false), "Enable chain-of-thought reasoning (default false for CPU path).")

	flag.StringVar(&clusterID, "cluster-id", envStr("CLUSTER_ID", "default"), "Cluster identifier for session/context lookups and ZenContext.")
	zenContextRedis := flag.String("zen-context-redis", envStr("ZEN_CONTEXT_REDIS_URL", ""), "Redis URL for ZenContext (ReMe). When set, Worker uses ReMeBinder for session context on continuation.")
	sessionAffinity := flag.Bool("session-affinity", envBool("ZEN_FOREMAN_SESSION_AFFINITY", false), "Route tasks by session (same session → same worker).")
	gateMode := flag.String("gate", envStr("ZEN_FOREMAN_GATE", "policy"), "Gate mode: log (audit only), policy (enforce BrainPolicy when present).")
	guardianMode := flag.String("guardian", envStr("ZEN_FOREMAN_GUARDIAN", "log"), "Guardian mode: log (audit log, allow all), circuit-breaker (log + per-session rate limit).")
	guardianCircuitMax := flag.Int("guardian-circuit-max-per-session-per-min", envInt("ZEN_FOREMAN_GUARDIAN_CIRCUIT_MAX_PER_SESSION_PER_MIN", 0), "Max tasks per session per minute when guardian=circuit-breaker; 0 = no limit.")
	flag.Parse()

	// FAIL CLOSED: Validate required flags/env vars
	if runtimeDir == "" {
		log.Fatalf("ZEN_FOREMAN_RUNTIME_DIR or --factory-runtime-dir not set (cannot use default /tmp/zen-brain-factory)")
	}
	if workspaceHome == "" {
		log.Fatalf("ZEN_FOREMAN_WORKSPACE_HOME or --factory-workspace-home not set (cannot use default /tmp/zen-brain-factory)")
	}

	// Set up controller-runtime logger using klog
	ctrl.SetLogger(klog.NewKlogr())

	// Block 3: Create context early for strict runtime bootstrap
	ctx := ctrl.SetupSignalHandler()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		log.Fatalf("Foreman: failed to create manager: %v", err)
	}

	// Block 3: Canonical strict runtime bootstrap
	profile := os.Getenv("ZEN_RUNTIME_PROFILE")
	if profile == "" {
		profile = "dev"
	}

	appCfg, errCfg := config.LoadConfig("")
	if errCfg != nil {
		log.Printf("Config load failed (%v), using defaults with env overrides", errCfg)
		appCfg = config.DefaultConfig()
		// CRITICAL: Apply env overrides after DefaultConfig() or TIER1_REDIS_ADDR etc. are ignored
		appCfg.ApplyEnvOverrides()
		log.Printf("Config: applied env overrides (tier1_redis.addr=%q)", appCfg.ZenContext.Tier1Redis.Addr)
	} else {
		log.Printf("Config: loaded from file (tier1_redis.addr=%q)", appCfg.ZenContext.Tier1Redis.Addr)
	}

	strictRT, errRT := internalruntime.NewStrictRuntime(ctx, &internalruntime.StrictRuntimeConfig{
		Profile:        profile,
		Config:         appCfg,
		EnableHealthCh: true,
	})

	if errRT != nil {
		// In strict mode (prod/staging), fail immediately
		if profile == "prod" || profile == "staging" {
			log.Fatalf("Foreman: strict runtime bootstrap failed: %v", errRT)
		}
		// In dev mode, continue with warning
		log.Printf("Foreman: runtime bootstrap warning (dev mode): %v", errRT)
	}

	// Start live health checker for dynamic readiness
	var healthChecker *internalruntime.LiveHealthChecker
	if strictRT != nil {
		healthChecker = internalruntime.NewLiveHealthChecker(&internalruntime.LiveHealthCheckerConfig{
			StrictRuntime: strictRT,
			RefreshPeriod: 30e9, // 30 seconds
		})
		if err := healthChecker.Start(ctx); err != nil {
			log.Printf("Foreman: warning: live health checker failed to start: %v", err)
		} else {
			defer healthChecker.Stop()
		}
	}

	// Wire readiness check to use StrictRuntime
	if strictRT != nil {
		if err := mgr.AddReadyzCheck("readyz", func(req *http.Request) error {
			return strictRT.CheckReadiness(req.Context())
		}); err != nil {
			log.Fatalf("Foreman: failed to add readyz check: %v", err)
		}
		log.Printf("Foreman: readiness check using strict runtime (profile=%s)", profile)
	} else {
		if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
			log.Fatalf("Foreman: failed to add readyz check: %v", err)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Fatalf("Foreman: failed to add healthz check: %v", err)
	}
	log.Printf("Foreman: manager created, health probe server on %s", probeAddr)

	cfg := foreman.FactoryTaskRunnerConfig{
		RuntimeDir:            runtimeDir,
		WorkspaceHome:         workspaceHome,
		PreferRealTemplates:   preferRealTemplates,
		UseGitWorktree:        useGitWorktree,
		SourceRepoPath:        sourceRepoPath,
		WorktreeBasePath:      worktreeBasePath,
		SourceRef:             sourceRef,
		ReuseSessionWorktree:  reuseSessionWorktree,
		EnableFactoryLLM:     enableFactoryLLM,
		LLMBaseURL:          llmBaseURL,
		LLMModel:            llmModel,
		LLMTimeoutSeconds:    llmTimeoutSeconds,
		LLMEnableThinking:     llmEnableThinking,
	}
	runner, err := foreman.NewFactoryTaskRunner(cfg)
	if err != nil {
		log.Printf("Foreman: failed to create FactoryTaskRunner: %v", err)
		os.Exit(1)
	}
	runner.Vault = evidence.NewMemoryVault() // proof-of-work evidence stored when tasks succeed

	// ZB-024: Log local CPU profile clearly
	if enableFactoryLLM {
		log.Printf("ZB-024: Local CPU profile active - model=%s timeout=%ds thinking=%v", llmModel, llmTimeoutSeconds, llmEnableThinking)
	}
	if useGitWorktree {
		log.Printf("Foreman: FactoryTaskRunner (git-worktree mode, repo=%s, base=%s)", sourceRepoPath, worktreeBasePath)
	} else {
		log.Printf("Foreman: FactoryTaskRunner (runtime %s, workspace %s, prefer-real=%v)", runtimeDir, workspaceHome, preferRealTemplates)
	}

	worker := foreman.NewWorker(mgr.GetClient(), runner, numWorkers)
	worker.SessionAffinity = *sessionAffinity
	if ledgerClient := foremanLedgerClient(); ledgerClient != nil {
		worker.LedgerClient = ledgerClient
		if closer, ok := ledgerClient.(interface{ Close() error }); ok {
			defer closer.Close()
		}
		log.Printf("Foreman: ZenLedger enabled (task completion will be recorded)")
	}
	if *zenContextRedis != "" {
		zc, err := internalcontext.NewMinimalZenContext(*zenContextRedis, clusterID)
		if err != nil {
			log.Printf("Warning: ZenContext (ReMe) not available: %v", err)
		} else {
			defer zc.Close()
			worker.ContextBinder = agent.NewReMeBinder(zc, clusterID)
			log.Printf("Foreman: ReMe enabled (ZenContext Redis, cluster=%s)", clusterID)
		}
	}
	
	// ZB-027G: Initialize Jira feedback service
	if feedbackSvc := foremanFeedbackService(mgr.GetClient(), appCfg); feedbackSvc != nil {
		worker.FeedbackService = feedbackSvc
		log.Printf("Foreman: Jira feedback enabled (task results will be reported to Jira)")
	}
	
	worker.Start(ctx)

	// Re-enqueue tasks stuck in Scheduled phase (pod restart recovery)
	go func() {
		// Wait for cache to be ready before listing tasks
		time.Sleep(5 * time.Second)

		var taskList v1alpha1.BrainTaskList
		if err := mgr.GetClient().List(ctx, &taskList); err != nil {
			log.Printf("Warning: failed to list BrainTasks for re-enqueue: %v", err)
			return
		}

		requeued := 0
		for _, task := range taskList.Items {
			if task.Status.Phase == v1alpha1.BrainTaskPhaseScheduled {
				if err := worker.Dispatch(ctx, &task); err != nil {
					log.Printf("Warning: failed to re-enqueue task %s: %v", task.Name, err)
					continue
				}
				requeued++
			}
		}

		if requeued > 0 {
			log.Printf("Foreman: re-enqueued %d scheduled tasks after startup", requeued)
		}
	}()

	var g guardian.ZenGuardian
	switch *guardianMode {
	case "log":
		g = internalguardian.NewLogGuardian()
		log.Printf("Foreman: Guardian=log (audit log)")
	case "circuit-breaker":
		cbCfg := internalguardian.CircuitBreakerConfig{
			MaxTasksPerSessionPerMinute: *guardianCircuitMax,
			Window:                      0, // default 1 min
		}
		g = internalguardian.NewCircuitBreakerGuardian(internalguardian.NewLogGuardian(), cbCfg)
		log.Printf("Foreman: Guardian=circuit-breaker (max %d/session/min)", *guardianCircuitMax)
	default:
		// Default to log guardian (fail-safe, not allow-all)
		g = internalguardian.NewLogGuardian()
		log.Printf("Foreman: Guardian=log (audit log, default)")
	}
	var zenGate gatepkg.ZenGate
	switch *gateMode {
	case "log":
		zenGate = gate.NewLogGate()
		log.Printf("Foreman: Gate=log (audit only)")
	case "policy":
		zenGate = gate.NewPolicyGate(mgr.GetClient())
		log.Printf("Foreman: Gate=policy (enforce BrainPolicy)")
	default:
		// Default to policy gate (fail-safe)
		zenGate = gate.NewPolicyGate(mgr.GetClient())
		log.Printf("Foreman: Gate=policy (enforce BrainPolicy, default)")
	}
	reconciler := &foreman.Reconciler{
		Client:     mgr.GetClient(),
		Gate:       zenGate,
		Guardian:   g,
		Dispatcher: worker,
	}
	if err = reconciler.SetupWithManager(mgr); err != nil {
		os.Exit(1)
	}

	queueStatusReconciler := &foreman.QueueStatusReconciler{Client: mgr.GetClient()}
	if err = queueStatusReconciler.SetupWithManager(mgr); err != nil {
		os.Exit(1)
	}

	if err := mgr.Start(ctx); err != nil {
		os.Exit(1)
	}
}

func envStr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "1" || v == "true" || v == "yes"
	}
	return defaultVal
}

func envInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

// foremanFeedbackService returns a BrainTask->Jira feedback service when Jira is configured.
// ZB-027G: Required for live Jira loop completion.
func foremanFeedbackService(k8sClient client.Client, cfg *config.Config) foreman.FeedbackService {
	// Check if Jira is configured
	if cfg.Jira.BaseURL == "" || cfg.Jira.Email == "" || cfg.Jira.APIToken == "" {
		log.Printf("Foreman: Jira feedback disabled (missing Jira credentials)")
		return nil
	}
	
	// Create Jira connection
	jiraConfig := &jira.Config{
		BaseURL:    cfg.Jira.BaseURL,
		Email:      cfg.Jira.Email,
		APIToken:   cfg.Jira.APIToken,
		ProjectKey: cfg.Jira.ProjectKey,
	}
	
	jiraConn, err := jira.New("foreman-feedback", "default", jiraConfig)
	if err != nil {
		log.Printf("Warning: failed to create Jira connection for feedback: %v", err)
		return nil
	}
	
	// Create and return feedback service
	feedbackConfig := feedback.DefaultBrainTaskToJiraConfig()
	return feedback.NewBrainTaskToJiraService(k8sClient, jiraConn, feedbackConfig)
}

// foremanLedgerClient returns a ZenLedgerClient when ZEN_LEDGER_DSN or LEDGER_DATABASE_URL is set (Block 4 completeness).
// Caller may defer Close() on the returned value if it implements interface{ Close() error }.
func foremanLedgerClient() ledger.ZenLedgerClient {
	dsn := os.Getenv("ZEN_LEDGER_DSN")
	if dsn == "" {
		dsn = os.Getenv("LEDGER_DATABASE_URL")
	}
	if dsn == "" {
		return nil
	}
	cl, err := internalledger.NewCockroachLedger(dsn)
	if err != nil {
		log.Printf("Foreman: ZenLedger unavailable: %v", err)
		return nil
	}
	return cl
}

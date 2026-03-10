// Command apiserver runs the zen-brain API server (Block 3.4).
// Serves /healthz, /readyz and optional future REST endpoints.
// Block 3: bootstrap runtime first; /readyz reflects real dependency state; /api/v1/health returns runtime report.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kube-zen/zen-brain1/internal/apiserver"
	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/internal/runtime"
)

func main() {
	ctx := context.Background()
	addr := ":8080"
	if p := os.Getenv("API_SERVER_PORT"); p != "" {
		addr = ":" + p
	}

	// Block 3: canonical bootstrap from config
	cfg, errLoad := config.LoadConfig("")
	if errLoad != nil || cfg == nil {
		if errLoad != nil {
			log.Printf("Config load failed (%v), using defaults", errLoad)
		}
		cfg = config.DefaultConfig()
	}
	rt, errBootstrap := runtime.Bootstrap(ctx, cfg)
	if errBootstrap != nil {
		log.Printf("Runtime bootstrap warning: %v", errBootstrap)
	}
	if rt != nil && rt.Report != nil {
		log.Println("Block 3 capability banner:", capabilityBanner(rt.Report))
	}

	checker := apiserver.NewRuntimeChecker(rt.Report)
	srv := apiserver.New(addr, checker)
	srv.AuthAPIKey = os.Getenv("ZEN_API_KEY")
	if srv.AuthAPIKey != "" {
		log.Println("API auth enabled (ZEN_API_KEY set); /healthz and /readyz are exempt")
	}
	srv.Handle("/api/v1/sessions", apiserver.SessionsHandler(nil))
	srv.Handle("/api/v1/sessions/", apiserver.SessionDetailHandler(nil))
	srv.Handle("/api/v1/health", apiserver.RuntimeReportHandler(rt.Report))
	srv.Handle("/api/v1/evidence", apiserver.EvidenceHandler(nil))
	gwCfg := llm.DefaultGatewayConfig()
	if s := os.Getenv("OLLAMA_TIMEOUT_SECONDS"); s != "" {
		if sec, err := strconv.Atoi(s); err == nil && sec > 0 {
			gwCfg.LocalWorkerTimeout = sec
			if sec > gwCfg.RequestTimeout {
				gwCfg.RequestTimeout = sec
			}
		}
	}
	gateway, errGW := llm.NewGateway(gwCfg)
	var warmup *llm.OllamaWarmupCoordinator
	if errGW != nil {
		log.Printf("LLM gateway not available: %v", errGW)
		srv.Handle("/api/v1/chat", apiserver.ChatHandler(nil, nil))
	} else {
		if baseURL := os.Getenv("OLLAMA_BASE_URL"); baseURL != "" {
			model := gwCfg.LocalWorkerModel
			keepAlive := os.Getenv("OLLAMA_KEEP_ALIVE")
			if keepAlive == "" {
				keepAlive = llm.DefaultKeepAlive
			}
			warmupSec := gwCfg.LocalWorkerTimeout
			if warmupSec <= 0 {
				warmupSec = 300
			}
			warmup = llm.NewOllamaWarmupCoordinator(baseURL, model, keepAlive, warmupSec)
			go warmup.DoWarmup(context.Background())
		}
		srv.Handle("/api/v1/chat", apiserver.ChatHandler(gateway, warmup))
	}
	if v := os.Getenv("API_VERSION"); v != "" {
		srv.Handle("/api/v1/version", apiserver.VersionHandler(v))
	} else {
		srv.Handle("/api/v1/version", apiserver.VersionHandler("dev"))
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil && err != context.Canceled {
			log.Printf("API server error: %v", err)
		}
	}()

	<-sigCtx.Done()
	log.Println("Shutting down API server...")
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	if rt != nil {
		_ = rt.Close()
	}
}

func capabilityBanner(r *runtime.RuntimeReport) string {
	if r == nil {
		return "ZenContext=? Ledger=? MessageBus=?"
	}
	zc := "disabled"
	if r.ZenContext.Mode != "" {
		zc = string(r.ZenContext.Mode)
		if r.Tier1Hot.Healthy {
			zc += " (tier1 ok)"
		}
		if r.Tier2Warm.Mode == runtime.ModeReal && !r.Tier2Warm.Healthy {
			zc += ", tier2 degraded"
		}
		if r.Tier3Cold.Mode == runtime.ModeDisabled {
			zc += ", tier3 disabled"
		}
	}
	ledger := string(r.Ledger.Mode)
	if r.Ledger.Mode == "" {
		ledger = "stub"
	}
	mb := string(r.MessageBus.Mode)
	if r.MessageBus.Mode == "" {
		mb = "disabled"
	}
	return "ZenContext=" + zc + " Ledger=" + ledger + " MessageBus=" + mb
}

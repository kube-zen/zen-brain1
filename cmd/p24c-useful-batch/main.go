func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	runtimeDir := "/tmp/zen-brain-factory"
	workspaceHome := "/tmp/zen-brain-workspaces"
	outputDir := "/tmp/zen-brain1-foreman-run/final"
	os.MkdirAll(runtimeDir, 0755)
	os.MkdirAll(workspaceHome, 0755)
	os.MkdirAll(outputDir, 0755)

	cfg := foreman.FactoryTaskRunnerConfig{
		RuntimeDir:      runtimeDir,
		WorkspaceHome:    workspaceHome,
		PreferRealTemplates: true,
		EnableFactoryLLM: true,
		LLMBaseURL:       "http://localhost:11434",
		LLMModel:         "qwen3.5:0.8b",
		LLMTimeoutSeconds: 120,
		LLMEnableThinking:   false,
	}

	os.Setenv("ZEN_BRAIN_MLQ_CONFIG", "/home/neves/zen/zen-brain1/config/policy/mlq-levels-local.yaml")

	runner, err := foreman.NewFactoryTaskRunner(cfg)
	if err != nil {
		log.Fatalf("Create runner: %v", err)
	}
}

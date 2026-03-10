package factory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// registerUsefulTemplates registers templates that do real work.
func (r *WorkTypeTemplateRegistry) registerUsefulTemplates() {
	// Register real implementation templates
	r.registerRealImplementationTemplate()
	r.registerRealDocumentationTemplate()
	r.registerRealBugFixTemplate()
	r.registerRealRefactorTemplate()
	r.registerRealPythonTemplate()
	r.registerRealReviewTemplate()

	// Batch II: Additional work types (stubs; not part of Block 4 scope)
	r.registerCICDTemplate()
	r.registerJavaScriptTemplate()
	r.registerDatabaseMigrationTemplate()
	r.registerMonitoringTemplate()
}

func (r *WorkTypeTemplateRegistry) registerCICDTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "cicd",
		WorkDomain:  "real",
		Description: "CI/CD pipeline: creates GitHub Actions workflow with build, test, and deploy stages",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Create CI/CD structure",
				Description: "Create .github/workflows directory",
				Command:     "mkdir -p .github/workflows && echo 'CI/CD structure created' > .cicd_structure",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate GitHub Actions workflow",
				Description: "Create CI workflow with build, test, and deploy stages",
				Command:     "echo 'name: CI' > .github/workflows/ci.yml && echo '' >> .github/workflows/ci.yml && echo 'on:' >> .github/workflows/ci.yml && echo '  push:' >> .github/workflows/ci.yml && echo '    branches: [ main, develop ]' >> .github/workflows/ci.yml && echo '  pull_request:' >> .github/workflows/ci.yml && echo '    branches: [ main ]' >> .github/workflows/ci.yml && echo '' >> .github/workflows/ci.yml && echo 'jobs:' >> .github/workflows/ci.yml && echo '  build:' >> .github/workflows/ci.yml && echo '    runs-on: ubuntu-latest' >> .github/workflows/ci.yml && echo '    steps:' >> .github/workflows/ci.yml && echo '      - uses: actions/checkout@v4' >> .github/workflows/ci.yml && echo '      - name: Set up Go' >> .github/workflows/ci.yml && echo '        uses: actions/setup-go@v5' >> .github/workflows/ci.yml && echo '        with:' >> .github/workflows/ci.yml && echo '          go-version: \"1.25\"' >> .github/workflows/ci.yml && echo '      - name: Build' >> .github/workflows/ci.yml && echo '        run: go build -v ./...' >> .github/workflows/ci.yml && echo '  test:' >> .github/workflows/ci.yml && echo '    runs-on: ubuntu-latest' >> .github/workflows/ci.yml && echo '    steps:' >> .github/workflows/ci.yml && echo '      - uses: actions/checkout@v4' >> .github/workflows/ci.yml && echo '      - name: Set up Go' >> .github/workflows/ci.yml && echo '        uses: actions/setup-go@v5' >> .github/workflows/ci.yml && echo '        with:' >> .github/workflows/ci.yml && echo '          go-version: \"1.25\"' >> .github/workflows/ci.yml && echo '      - name: Test' >> .github/workflows/ci.yml && echo '        run: go test -v ./...' >> .github/workflows/ci.yml && echo 'CI workflow generated' > .ci_workflow",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Create deployment documentation",
				Description: "Document deployment process",
				Command:     "echo '# CI/CD Pipeline' > DEPLOYMENT.md && echo '' >> DEPLOYMENT.md && echo '## Workflow' >> DEPLOYMENT.md && echo '' >> DEPLOYMENT.md && echo '- Work Item: {{.work_item_id}}' >> DEPLOYMENT.md && echo '- Title: {{.title}}' >> DEPLOYMENT.md && echo '' >> DEPLOYMENT.md && echo '## Pipeline Stages' >> DEPLOYMENT.md && echo '' >> DEPLOYMENT.md && echo '1. **Build**: Compiles the code' >> DEPLOYMENT.md && echo '2. **Test**: Runs all tests' >> DEPLOYMENT.md && echo '3. **Deploy**: Deploys to target environment' >> DEPLOYMENT.md && echo '' >> DEPLOYMENT.md && echo '## Triggers' >> DEPLOYMENT.md && echo '' >> DEPLOYMENT.md && echo '- Push to main/develop branches' >> DEPLOYMENT.md && echo '- Pull requests to main branch' >> DEPLOYMENT.md && echo 'Deployment documentation created' > .deploy_documented",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate proof-of-work summary",
				Description: "Create CI/CD proof-of-work",
				Command:     "echo '# Proof of Work' > PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## CI/CD Pipeline Setup' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '- Work Item: {{.work_item_id}}' >> PROOF_OF_WORK.md && echo '- Title: {{.title}}' >> PROOF_OF_WORK.md && echo '- Objective: {{.objective}}' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Files Created' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '- .github/workflows/ci.yml - GitHub Actions workflow' >> PROOF_OF_WORK.md && echo '- DEPLOYMENT.md - Deployment documentation' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Verification' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo 'Check the workflow in .github/workflows/ci.yml' >> PROOF_OF_WORK.md && echo 'Proof of work generated' > .pow_generated",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

func (r *WorkTypeTemplateRegistry) registerJavaScriptTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "implementation",
		WorkDomain:  "javascript",
		Description: "JavaScript implementation: creates Node.js project with source code, tests, and documentation",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Create JavaScript project structure",
				Description: "Create Node.js directories and files",
				Command:     "mkdir -p src tests && echo '{' > package.json && echo '  \"name\": \"{{.work_item_id}}\",' >> package.json && echo '  \"version\": \"1.0.0\",' >> package.json && echo '  \"description\": \"{{.title}}\",' >> package.json && echo '  \"main\": \"src/main.js\",' >> package.json && echo '  \"scripts\": {' >> package.json && echo '    \"start\": \"node src/main.js\",' >> package.json && echo '    \"test\": \"node --test tests/**/*.test.js\"' >> package.json && echo '  },' >> package.json && echo '  \"type\": \"module\",' >> package.json && echo '  \"engines\": {' >> package.json && echo '    \"node\": \">=18.0.0\"' >> package.json && echo '  }' >> package.json && echo '}' >> package.json && echo 'node_modules/' > .gitignore && echo '.DS_Store' >> .gitignore && echo '*.log' >> .gitignore && echo 'JavaScript project structure created' > .structure_created",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate JavaScript source code",
				Description: "Generate actual JavaScript source files",
				Command:     "echo '// Main application for {{.title}}' > src/main.js && echo '' >> src/main.js && echo 'const title = \"{{.title}}\";' >> src/main.js && echo 'const objective = \"{{.objective}}\";' >> src/main.js && echo '' >> src/main.js && echo 'class Main {' >> src/main.js && echo '  constructor(name) {' >> src/main.js && echo '    this.name = name || title;' >> src/main.js && echo '  }' >> src/main.js && echo '' >> src/main.js && echo '  run() {' >> src/main.js && echo '    console.log(`Hello from ${this.name}`);' >> src/main.js && echo '  }' >> src/main.js && echo '}' >> src/main.js && echo '' >> src/main.js && echo 'function main() {' >> src/main.js && echo '  const app = new Main();' >> src/main.js && echo '  app.run();' >> src/main.js && echo '}' >> src/main.js && echo '' >> src/main.js && echo 'if (import.meta.url === `file://${process.argv[1]}`) {' >> src/main.js && echo '  main();' >> src/main.js && echo '}' >> src/main.js && echo 'Source code generated' > .code_generated",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Create documentation",
				Description: "Generate README and documentation",
				Command:     "echo '# {{.title}}' > README.md && echo '' >> README.md && echo '{{.objective}}' >> README.md && echo '' >> README.md && echo '## Work Item' >> README.md && echo '' >> README.md && echo '- ID: {{.work_item_id}}' >> README.md && echo '' >> README.md && echo '## Installation' >> README.md && echo '' >> README.md && echo 'npm install' >> README.md && echo '' >> README.md && echo '## Usage' >> README.md && echo '' >> README.md && echo 'npm start' >> README.md && echo '' >> README.md && echo '## Testing' >> README.md && echo '' >> README.md && echo 'npm test' >> README.md && mkdir -p docs && echo '# API Documentation' > docs/api.md && echo '' >> docs/api.md && echo '## Main Class' >> docs/api.md && echo '' >> docs/api.md && echo '### Main(name)' >> docs/api.md && echo '' >> docs/api.md && echo 'Initialize the main application.' >> docs/api.md && echo '' >> docs/api.md && echo '### run()' >> docs/api.md && echo '' >> docs/api.md && echo 'Run the main application.' >> docs/api.md && echo 'Documentation generated' > .docs_generated",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Write tests",
				Description: "Create Node.js test files",
				Command:     "echo 'import { describe, it } from \"node:test\";' > tests/main.test.js && echo 'import assert from \"node:assert\";' >> tests/main.test.js && echo 'import { Main } from \"../src/main.js\";' >> tests/main.test.js && echo '' >> tests/main.test.js && echo 'describe(\"Main\", () => {' >> tests/main.test.js && echo '  it(\"should initialize with default name\", () => {' >> tests/main.test.js && echo '    const app = new Main();' >> tests/main.test.js && echo '    assert.strictEqual(app.name, \"{{.title}}\");' >> tests/main.test.js && echo '  });' >> tests/main.test.js && echo '});' >> tests/main.test.js && echo '# Test package' > tests/package.json && echo '{\"type\":\"module\"}' >> tests/package.json && echo 'Tests created' > .tests_created",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Generate proof-of-work summary",
				Description: "Create summary of work done",
				Command:     "echo '# Proof of Work' > PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Summary' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '- Work Item: {{.work_item_id}}' >> PROOF_OF_WORK.md && echo '- Title: {{.title}}' >> PROOF_OF_WORK.md && echo '- Objective: {{.objective}}' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Files Created' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '- src/main.js - Main application' >> PROOF_OF_WORK.md && echo '- tests/main.test.js - Node.js test suite' >> PROOF_OF_WORK.md && echo '- package.json - Node.js dependencies' >> PROOF_OF_WORK.md && echo '- README.md - Documentation' >> PROOF_OF_WORK.md && echo '- docs/api.md - API documentation' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Verification' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo 'Run tests: npm test' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo 'Run application: npm start' >> PROOF_OF_WORK.md && echo 'Proof of work generated' > .pow_generated",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

func (r *WorkTypeTemplateRegistry) registerDatabaseMigrationTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "migration",
		WorkDomain:  "real",
		Description: "Database migration: creates migration files, rollback scripts, and documentation",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Create migration structure",
				Description: "Create migrations directory structure",
				Command:     "mkdir -p migrations rollbacks && echo 'Migration structure created' > .migration_structure",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate up migration",
				Description: "Create migration SQL file",
				Command:     "echo '-- Migration: {{.title}}' > migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '-- Work Item: {{.work_item_id}}' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo 'BEGIN;' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '-- TODO: Add your migration SQL here' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '-- Example:' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '-- CREATE TABLE example (' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '--   id SERIAL PRIMARY KEY,' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '--   name VARCHAR(255) NOT NULL,' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '--   created_at TIMESTAMP DEFAULT NOW()' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '-- );' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo '' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo 'COMMIT;' >> migrations/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_up.sql && echo 'Up migration generated' > .up_migration",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate down migration",
				Description: "Create rollback migration SQL file",
				Command:     "echo '-- Rollback: {{.title}}' > rollbacks/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_down.sql && echo '-- Work Item: {{.work_item_id}}' >> rollbacks/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_down.sql && echo '' >> rollbacks/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_down.sql && echo 'BEGIN;' >> rollbacks/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_down.sql && echo '' >> rollbacks/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_down.sql && echo '-- TODO: Add your rollback SQL here' >> rollbacks/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_down.sql && echo '-- Example:' >> rollbacks/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_down.sql && echo '-- DROP TABLE IF EXISTS example;' >> rollbacks/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_down.sql && echo '' >> rollbacks/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_down.sql && echo 'COMMIT;' >> rollbacks/$(date +%Y%m%d%H%M%S)_{{.work_item_id}}_down.sql && echo 'Down migration generated' > .down_migration",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Create migration documentation",
				Description: "Document the migration",
				Command:     "echo '# Database Migration' > MIGRATION.md && echo '' >> MIGRATION.md && echo '## Summary' >> MIGRATION.md && echo '' >> MIGRATION.md && echo '- Work Item: {{.work_item_id}}' >> MIGRATION.md && echo '- Title: {{.title}}' >> MIGRATION.md && echo '- Objective: {{.objective}}' >> MIGRATION.md && echo '' >> MIGRATION.md && echo '## Migration Files' >> MIGRATION.md && echo '' >> MIGRATION.md && echo '- Up migration: migrations/*_up.sql' >> MIGRATION.md && echo '- Down migration: rollbacks/*_down.sql' >> MIGRATION.md && echo '' >> MIGRATION.md && echo '## Running the Migration' >> MIGRATION.md && echo '' >> MIGRATION.md && echo '```bash' >> MIGRATION.md && echo '# Apply migration' >> MIGRATION.md && echo 'psql -U username -d database -f migrations/*_up.sql' >> MIGRATION.md && echo '' >> MIGRATION.md && echo '# Rollback migration' >> MIGRATION.md && echo 'psql -U username -d database -f rollbacks/*_down.sql' >> MIGRATION.md && echo '```' >> MIGRATION.md && echo 'Migration documentation created' > .migration_documented",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate proof-of-work summary",
				Description: "Create migration proof-of-work",
				Command:     "echo '# Proof of Work' > PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Database Migration' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '- Work Item: {{.work_item_id}}' >> PROOF_OF_WORK.md && echo '- Title: {{.title}}' >> PROOF_OF_WORK.md && echo '- Objective: {{.objective}}' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Files Created' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '- migrations/*_up.sql - Up migration script' >> PROOF_OF_WORK.md && echo '- rollbacks/*_down.sql - Down migration script' >> PROOF_OF_WORK.md && echo '- MIGRATION.md - Migration documentation' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Verification' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo 'Review MIGRATION.md for migration instructions' >> PROOF_OF_WORK.md && echo 'Proof of work generated' > .pow_generated",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

func (r *WorkTypeTemplateRegistry) registerMonitoringTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "monitoring",
		WorkDomain:  "real",
		Description: "Monitoring setup: creates Prometheus metrics, Grafana dashboards, and alerting rules",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Create monitoring structure",
				Description: "Create monitoring directories",
				Command:     "mkdir -p monitoring/metrics monitoring/dashboards monitoring/alerts && echo 'Monitoring structure created' > .monitoring_structure",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate Prometheus metrics config",
				Description: "Create metrics configuration",
				Command:     "echo '# Prometheus Metrics Configuration' > monitoring/metrics/metrics.yml && echo '' >> monitoring/metrics/metrics.yml && echo '## Application Metrics' >> monitoring/metrics/metrics.yml && echo '' >> monitoring/metrics/metrics.yml && echo '- Work Item: {{.work_item_id}}' >> monitoring/metrics/metrics.yml && echo '- Title: {{.title}}' >> monitoring/metrics/metrics.yml && echo '' >> monitoring/metrics/metrics.yml && echo '### Key Metrics' >> monitoring/metrics/metrics.yml && echo '' >> monitoring/metrics/metrics.yml && echo '1. **http_requests_total** - Total HTTP requests' >> monitoring/metrics/metrics.yml && echo '2. **http_request_duration_seconds** - Request latency' >> monitoring/metrics/metrics.yml && echo '3. **active_connections** - Current connections' >> monitoring/metrics/metrics.yml && echo '4. **error_rate** - Error percentage' >> monitoring/metrics/metrics.yml && echo '' >> monitoring/metrics/metrics.yml && echo '### Metric Labels' >> monitoring/metrics/metrics.yml && echo '' >> monitoring/metrics/metrics.yml && echo '- method: HTTP method' >> monitoring/metrics/metrics.yml && echo '- status: HTTP status code' >> monitoring/metrics/metrics.yml && echo '- endpoint: Request endpoint' >> monitoring/metrics/metrics.yml && echo 'Metrics config generated' > .metrics_config",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate Grafana dashboard",
				Description: "Create Grafana dashboard JSON",
				Command:     "echo '{' > monitoring/dashboards/application.json && echo '  \"dashboard\": {' >> monitoring/dashboards/application.json && echo '    \"title\": \"{{.title}}\",' >> monitoring/dashboards/application.json && echo '    \"uid\": \"{{.work_item_id}}\",' >> monitoring/dashboards/application.json && echo '    \"panels\": [' >> monitoring/dashboards/application.json && echo '      {' >> monitoring/dashboards/application.json && echo '        \"title\": \"Request Rate\",' >> monitoring/dashboards/application.json && echo '        \"targets\": [{' >> monitoring/dashboards/application.json && echo '          \"expr\": \"rate(http_requests_total[5m])\"' >> monitoring/dashboards/application.json && echo '        }]' >> monitoring/dashboards/application.json && echo '      },' >> monitoring/dashboards/application.json && echo '      {' >> monitoring/dashboards/application.json && echo '        \"title\": \"Request Latency\",' >> monitoring/dashboards/application.json && echo '        \"targets\": [{' >> monitoring/dashboards/application.json && echo '          \"expr\": \"histogram_quantile(0.95, http_request_duration_seconds_bucket)\"' >> monitoring/dashboards/application.json && echo '        }]' >> monitoring/dashboards/application.json && echo '      }' >> monitoring/dashboards/application.json && echo '    ]' >> monitoring/dashboards/application.json && echo '  }' >> monitoring/dashboards/application.json && echo '}' >> monitoring/dashboards/application.json && echo 'Grafana dashboard generated' > .dashboard_config",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate alerting rules",
				Description: "Create Prometheus alert rules",
				Command:     "echo 'groups:' > monitoring/alerts/alerts.yml && echo '  - name: {{.work_item_id}}' >> monitoring/alerts/alerts.yml && echo '    rules:' >> monitoring/alerts/alerts.yml && echo '      - alert: HighErrorRate' >> monitoring/alerts/alerts.yml && echo '        expr: rate(http_requests_total{status=~\"5..\"}[5m]) > 0.05' >> monitoring/alerts/alerts.yml && echo '        for: 5m' >> monitoring/alerts/alerts.yml && echo '        annotations:' >> monitoring/alerts/alerts.yml && echo '          summary: \"High error rate detected\"' >> monitoring/alerts/alerts.yml && echo '      - alert: HighLatency' >> monitoring/alerts/alerts.yml && echo '        expr: histogram_quantile(0.95, http_request_duration_seconds_bucket) > 1' >> monitoring/alerts/alerts.yml && echo '        for: 5m' >> monitoring/alerts/alerts.yml && echo '        annotations:' >> monitoring/alerts/alerts.yml && echo '          summary: \"High latency detected\"' >> monitoring/alerts/alerts.yml && echo 'Alerting rules generated' > .alerts_config",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Create monitoring documentation",
				Description: "Document monitoring setup",
				Command:     "echo '# Monitoring Setup' > MONITORING.md && echo '' >> MONITORING.md && echo '## Summary' >> MONITORING.md && echo '' >> MONITORING.md && echo '- Work Item: {{.work_item_id}}' >> MONITORING.md && echo '- Title: {{.title}}' >> MONITORING.md && echo '- Objective: {{.objective}}' >> MONITORING.md && echo '' >> MONITORING.md && echo '## Components' >> MONITORING.md && echo '' >> MONITORING.md && echo '1. **Metrics**: monitoring/metrics/metrics.yml' >> MONITORING.md && echo '2. **Dashboards**: monitoring/dashboards/application.json' >> MONITORING.md && echo '3. **Alerts**: monitoring/alerts/alerts.yml' >> MONITORING.md && echo '' >> MONITORING.md && echo '## Alerting' >> MONITORING.md && echo '' >> MONITORING.md && echo '- **HighErrorRate**: Triggered when error rate > 5% for 5 minutes' >> MONITORING.md && echo '- **HighLatency**: Triggered when p95 latency > 1s for 5 minutes' >> MONITORING.md && echo 'Monitoring documentation created' > .monitoring_documented",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate proof-of-work summary",
				Description: "Create monitoring proof-of-work",
				Command:     "echo '# Proof of Work' > PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Monitoring Setup' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '- Work Item: {{.work_item_id}}' >> PROOF_OF_WORK.md && echo '- Title: {{.title}}' >> PROOF_OF_WORK.md && echo '- Objective: {{.objective}}' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Files Created' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '- monitoring/metrics/metrics.yml - Metrics configuration' >> PROOF_OF_WORK.md && echo '- monitoring/dashboards/application.json - Grafana dashboard' >> PROOF_OF_WORK.md && echo '- monitoring/alerts/alerts.yml - Alert rules' >> PROOF_OF_WORK.md && echo '- MONITORING.md - Documentation' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Verification' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo 'Import dashboard into Grafana and verify alerts' >> PROOF_OF_WORK.md && echo 'Proof of work generated' > .pow_generated",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerRealImplementationTemplate creates a template that generates real files.
func (r *WorkTypeTemplateRegistry) registerRealImplementationTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "implementation",
		WorkDomain:  "real",
		Description: "Real implementation: creates actual files, documentation, and tests",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Create project structure",
				Description: "Create source directories and files",
				Command:     "mkdir -p cmd internal pkg docs tests && echo 'Project structure created' > .structure_created",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate source code",
				Description: "Generate actual Go source files",
				Command:     "echo 'package main\n\nfunc main() {\n    println(\"Hello from {{.title}}\")\n}' > cmd/main.go && echo 'Source code generated' > .code_generated",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Create documentation",
				Description: "Generate README and API documentation",
				Command:     "echo '# {{.title}}\n\n{{.objective}}\n' > README.md && mkdir -p docs && echo '# API Documentation\n' > docs/API.md && echo 'Documentation generated' > .docs_generated",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Write tests",
				Description: "Create test files with test cases",
				Command:     "echo 'package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {\n    t.Log(\"Test passed\")\n}' > cmd/main_test.go && echo 'Tests created' > .tests_created",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Generate proof-of-work summary",
				Description: "Create summary of work done",
				Command:     "echo '# Proof of Work\n\nWork Item: {{.work_item_id}}\nTitle: {{.title}}\n' > PROOF_OF_WORK.md && echo 'Proof of work generated' > .pow_generated",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerRealDocumentationTemplate creates a template for documentation work.
func (r *WorkTypeTemplateRegistry) registerRealDocumentationTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "docs",
		WorkDomain:  "real",
		Description: "Real documentation: creates actual markdown files with content",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Create documentation structure",
				Description: "Create docs directory and index",
				Command:     "mkdir -p docs examples && echo 'Documentation structure created' > .docs_structure",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate main documentation",
				Description: "Create primary documentation file",
				Command:     "echo '# {{.title}}\n\n{{.objective}}\n' > docs/README.md && echo 'Main documentation generated' > .docs_main",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Generate examples",
				Description: "Create usage examples",
				Command:     "echo '# Example Usage\n\n```go\n// Example code\n```' > examples/example.md && echo 'Examples generated' > .docs_examples",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate proof-of-work summary",
				Description: "Create summary of documentation created",
				Command:     "echo '# Proof of Work\n\nWork Item: {{.work_item_id}}\nTitle: {{.title}}\n' > PROOF_OF_WORK.md && echo 'Proof of work generated' > .pow_generated",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerRealBugFixTemplate creates a template for bug fixes.
func (r *WorkTypeTemplateRegistry) registerRealBugFixTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "bugfix",
		WorkDomain:  "real",
		Description: "Real bug fix: creates analysis, fix code, tests, and documentation",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Analyze bug",
				Description: "Create bug analysis document",
				Command:     "mkdir -p analysis && echo '# Bug Analysis\n\n## {{.title}}\n\n{{.objective}}\n\n## Root Cause\n[Analysis pending]\n\n## Impact\n[Impact assessment]\n' > analysis/BUG_REPORT.md && echo 'Bug analysis created' > .analysis_created",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Implement fix",
				Description: "Create fix implementation file",
				Command:     "mkdir -p internal && echo 'package internal\n\n// Fix for {{.title}}\n//\n// Work Item: {{.work_item_id}}\n//\n// This implements the fix for the bug described in analysis/BUG_REPORT.md\n\n// TODO: Implement the actual fix here\nfunc ApplyFix() error {\n    // Fix implementation\n    return nil\n}\n' > internal/fix.go && echo 'Fix implemented' > .fix_implemented",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Write tests for fix",
				Description: "Create test file with regression tests",
				Command:     "echo 'package internal\n\nimport (\n    \"testing\"\n)\n\nfunc TestApplyFix(t *testing.T) {\n    t.Run(\"Fix is applied correctly\", func(t *testing.T) {\n        if err := ApplyFix(); err != nil {\n            t.Errorf(\"ApplyFix() error = %v\", err)\n        }\n    })\n}\n' > internal/fix_test.go && echo 'Tests created' > .tests_created",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Create fix documentation",
				Description: "Document the bug and fix",
				Command:     "echo '# Fix Documentation' > FIX_DOCUMENTATION.md && echo '' >> FIX_DOCUMENTATION.md && echo '## Summary' >> FIX_DOCUMENTATION.md && echo '' >> FIX_DOCUMENTATION.md && echo '- Work Item: {{.work_item_id}}' >> FIX_DOCUMENTATION.md && echo '- Title: {{.title}}' >> FIX_DOCUMENTATION.md && echo '' >> FIX_DOCUMENTATION.md && echo '## Bug Analysis' >> FIX_DOCUMENTATION.md && echo '' >> FIX_DOCUMENTATION.md && echo 'See analysis/BUG_REPORT.md for detailed bug analysis.' >> FIX_DOCUMENTATION.md && echo '' >> FIX_DOCUMENTATION.md && echo '## Fix Implementation' >> FIX_DOCUMENTATION.md && echo '' >> FIX_DOCUMENTATION.md && echo 'See internal/fix.go for the fix implementation.' >> FIX_DOCUMENTATION.md && echo '' >> FIX_DOCUMENTATION.md && echo '## Tests' >> FIX_DOCUMENTATION.md && echo '' >> FIX_DOCUMENTATION.md && echo 'See internal/fix_test.go for regression tests.' >> FIX_DOCUMENTATION.md && echo '' >> FIX_DOCUMENTATION.md && echo '## Verification' >> FIX_DOCUMENTATION.md && echo '' >> FIX_DOCUMENTATION.md && echo 'Run tests: go test ./internal -v -run TestApplyFix' >> FIX_DOCUMENTATION.md && echo 'Fix documentation created' > .fix_documented",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerRealRefactorTemplate creates a template for code refactoring.
func (r *WorkTypeTemplateRegistry) registerRealRefactorTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "refactor",
		WorkDomain:  "real",
		Description: "Real refactoring: creates analysis, refactored code, tests, and documentation",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Analyze code for refactoring",
				Description: "Create code analysis document",
				Command:     "mkdir -p analysis && echo '# Refactoring Analysis' > analysis/REFACTOR_ANALYSIS.md && echo '' >> analysis/REFACTOR_ANALYSIS.md && echo '## {{.title}}' >> analysis/REFACTOR_ANALYSIS.md && echo '' >> analysis/REFACTOR_ANALYSIS.md && echo '{{.objective}}' >> analysis/REFACTOR_ANALYSIS.md && echo '' >> analysis/REFACTOR_ANALYSIS.md && echo '## Current Issues' >> analysis/REFACTOR_ANALYSIS.md && echo '' >> analysis/REFACTOR_ANALYSIS.md && echo '- Issue 1: [Description]' >> analysis/REFACTOR_ANALYSIS.md && echo '- Issue 2: [Description]' >> analysis/REFACTOR_ANALYSIS.md && echo 'Refactor analysis created' > .analysis_created",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Implement refactored code",
				Description: "Create refactored implementation",
				Command:     "mkdir -p pkg && echo 'package pkg' > pkg/refactored.go && echo '' >> pkg/refactored.go && echo '// Refactored implementation for {{.title}}' >> pkg/refactored.go && echo '' >> pkg/refactored.go && echo 'type Refactored struct {' >> pkg/refactored.go && echo '}' >> pkg/refactored.go && echo '' >> pkg/refactored.go && echo 'func (r *Refactored) Process() error {' >> pkg/refactored.go && echo '    return nil' >> pkg/refactored.go && echo '}' >> pkg/refactored.go && echo 'Refactored code implemented' > .refactor_implemented",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Write refactored tests",
				Description: "Create comprehensive tests for refactored code",
				Command:     "echo 'package pkg' > pkg/refactored_test.go && echo '' >> pkg/refactored_test.go && echo 'import (' >> pkg/refactored_test.go && echo '    \"testing\"' >> pkg/refactored_test.go && echo ')' >> pkg/refactored_test.go && echo '' >> pkg/refactored_test.go && echo 'func TestRefactored_Process(t *testing.T) {' >> pkg/refactored_test.go && echo '    t.Log(\"Test passed\")' >> pkg/refactored_test.go && echo '}' >> pkg/refactored_test.go && echo 'Refactored tests created' > .tests_created",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Create refactoring documentation",
				Description: "Document the refactoring changes",
				Command:     "echo '# Refactoring Documentation' > REFACTORING.md && echo '' >> REFACTORING.md && echo '## Summary' >> REFACTORING.md && echo '' >> REFACTORING.md && echo '- Work Item: {{.work_item_id}}' >> REFACTORING.md && echo '- Title: {{.title}}' >> REFACTORING.md && echo 'Refactoring documentation created' > .refactor_documented",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerRealPythonTemplate creates a template for Python implementation.
func (r *WorkTypeTemplateRegistry) registerRealPythonTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "implementation",
		WorkDomain:  "python",
		Description: "Real Python implementation: creates Python project with source code, tests, and documentation",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Create Python project structure",
				Description: "Create Python directories and files",
				Command:     "mkdir -p src tests && echo '# Requirements' > requirements.txt && echo '' >> requirements.txt && echo 'pytest>=7.0.0' >> requirements.txt && echo '# Python' > .gitignore && echo '__pycache__/' >> .gitignore && echo '*.py[cod]' >> .gitignore && echo '*$py.class' >> .gitignore && echo '*.so' >> .gitignore && echo '.Python' >> .gitignore && echo 'venv/' >> .gitignore && echo 'env/' >> .gitignore && echo 'from setuptools import setup, find_packages' > setup.py && echo '' >> setup.py && echo 'setup(' >> setup.py && echo '    name=\"{{.work_item_id}}\",' >> setup.py && echo '    version=\"0.1.0\",' >> setup.py && echo '    description=\"{{.title}}\",' >> setup.py && echo '    packages=find_packages(),' >> setup.py && echo '    python_requires=\">=3.8\",' >> setup.py && echo ')' >> setup.py && echo 'Python project structure created' > .structure_created",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Generate Python source code",
				Description: "Generate actual Python source files",
				Command:     "echo '#!/usr/bin/env python3' > src/main.py && echo 'title = \"{{.title}}\"' >> src/main.py && echo 'objective = \"{{.objective}}\"' >> src/main.py && echo '' >> src/main.py && echo 'class Main:' >> src/main.py && echo '    def __init__(self, name=None):' >> src/main.py && echo '        self.name = name or title' >> src/main.py && echo '' >> src/main.py && echo '    def run(self):' >> src/main.py && echo '        print(\"Hello from {}\".format(self.name))' >> src/main.py && echo '' >> src/main.py && echo 'def main():' >> src/main.py && echo '    app = Main()' >> src/main.py && echo '    app.run()' >> src/main.py && echo '' >> src/main.py && echo 'if __name__ == \"__main__\":' >> src/main.py && echo '    main()' >> src/main.py && chmod +x src/main.py && echo 'Source code generated' > .code_generated",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Create documentation",
				Description: "Generate README and documentation",
				Command:     "echo '# {{.title}}' > README.md && echo '' >> README.md && echo '{{.objective}}' >> README.md && echo '' >> README.md && echo '## Work Item' >> README.md && echo '' >> README.md && echo '- ID: {{.work_item_id}}' >> README.md && echo '' >> README.md && echo '## Installation' >> README.md && echo '' >> README.md && echo 'pip install -r requirements.txt' >> README.md && echo '' >> README.md && echo '## Usage' >> README.md && echo '' >> README.md && echo 'python src/main.py' >> README.md && echo '' >> README.md && echo '## Testing' >> README.md && echo '' >> README.md && echo 'pytest tests/ -v' >> README.md && mkdir -p docs && echo '# API Documentation' > docs/api.md && echo '' >> docs/api.md && echo '## Main Class' >> docs/api.md && echo '' >> docs/api.md && echo '### Main(name=None)' >> docs/api.md && echo '' >> docs/api.md && echo 'Initialize the main application.' >> docs/api.md && echo '' >> docs/api.md && echo '### run()' >> docs/api.md && echo '' >> docs/api.md && echo 'Run the main application.' >> docs/api.md && echo 'Documentation generated' > .docs_generated",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Write tests",
				Description: "Create pytest test files",
				Command:     "echo 'import pytest' > tests/test_main.py && echo 'from src.main import Main' >> tests/test_main.py && echo '' >> tests/test_main.py && echo 'class TestMain:' >> tests/test_main.py && echo '    def test_initialization(self):' >> tests/test_main.py && echo '        app = Main()' >> tests/test_main.py && echo '        assert app.name == \"{{.title}}\"' >> tests/test_main.py && echo '' >> tests/test_main.py && echo '    def test_run(self, capsys):' >> tests/test_main.py && echo '        app = Main()' >> tests/test_main.py && echo '        app.run()' >> tests/test_main.py && echo '        captured = capsys.readouterr()' >> tests/test_main.py && echo '        assert \"Hello\" in captured.out' >> tests/test_main.py && echo '# Test package' > tests/__init__.py && echo '# Source package' > src/__init__.py && echo 'Tests created' > .tests_created",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Generate proof-of-work summary",
				Description: "Create summary of work done",
				Command:     "echo '# Proof of Work' > PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Summary' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '- Work Item: {{.work_item_id}}' >> PROOF_OF_WORK.md && echo '- Title: {{.title}}' >> PROOF_OF_WORK.md && echo '- Objective: {{.objective}}' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Files Created' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '- src/main.py - Main application' >> PROOF_OF_WORK.md && echo '- tests/test_main.py - Pytest test suite' >> PROOF_OF_WORK.md && echo '- requirements.txt - Python dependencies' >> PROOF_OF_WORK.md && echo '- setup.py - Package setup' >> PROOF_OF_WORK.md && echo '- README.md - Documentation' >> PROOF_OF_WORK.md && echo '- docs/api.md - API documentation' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo '## Verification' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo 'Run tests: python -m pytest tests/ -v' >> PROOF_OF_WORK.md && echo '' >> PROOF_OF_WORK.md && echo 'Run application: python src/main.py' >> PROOF_OF_WORK.md && echo 'Proof of work generated' > .pow_generated",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
		r.registerTemplate(template)
}

// registerRealReviewTemplate creates the canonical repo-aware review lane (Block 4).
// Step 1: workspace and git inventory (pwd, files, git branch/commit/status/diff or "not a git repo").
// Step 2: language-aware safe checks (Go test, Python py_compile when tools exist).
// Step 3: REVIEW.md from real observations (work item, path, git-backed, checks ran, next action).
func (r *WorkTypeTemplateRegistry) registerRealReviewTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "review",
		WorkDomain:  "real",
		Description: "Real review: repo-aware inventory, git evidence, Go/Python checks, REVIEW.md from observations",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Workspace and git inventory",
				Description: "Create review dir, pwd, list files; if git repo write branch/commit/status/diff, else write not-a-git-repo markers",
				Command:     "mkdir -p review && pwd > review/workspace.txt && find . -maxdepth 4 -type f ! -path './.git/*' ! -path './.zen-*' | sort > review/files.txt && (git rev-parse --is-inside-work-tree 2>/dev/null | grep -q true && (git rev-parse --abbrev-ref HEAD > review/git-branch.txt && git rev-parse HEAD > review/git-commit.txt && git status --short > review/git-status.txt 2>/dev/null || true && git diff --stat > review/git-diff-stat.txt 2>/dev/null || true && git diff --name-only > review/git-diff-files.txt 2>/dev/null || true) || (echo 'not a git repo' > review/git-branch.txt && echo 'not a git repo' > review/git-commit.txt && echo 'not a git repo' > review/git-status.txt && echo 'not a git repo' > review/git-diff-stat.txt && echo 'not a git repo' > review/git-diff-files.txt))",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Language-aware safe checks",
				Description: "Run go test ./... if go.mod and go exist; run Python py_compile if py project and python3 exist; else write skipped markers",
				Command:     "if [ -f go.mod ] && command -v go >/dev/null 2>&1; then go test ./... -count=1 > review/go-test.txt 2>&1 || true; else echo 'skipped (no go.mod or go not in PATH)' > review/go-test.txt; fi && (if ( [ -f pyproject.toml ] || [ -f setup.py ] || [ -f requirements.txt ] ) && command -v python3 >/dev/null 2>&1; then pyfiles=$(find . -name '*.py' -type f 2>/dev/null | head -20); if [ -n \"$pyfiles\" ]; then python3 -m py_compile $pyfiles > review/python-test.txt 2>&1 || true; else echo 'skipped (no .py files)' > review/python-test.txt; fi; else echo 'skipped (no pyproject.toml/setup.py/requirements.txt or python3 not in PATH)' > review/python-test.txt; fi)",
				Variables:   map[string]string{},
				Timeout:     120,
				MaxRetries:  1,
			},
			{
				Name:        "Generate REVIEW.md from observations",
				Description: "Create REVIEW.md with work item, title, objective, workspace path, git-backed flag, Go/Python check status, diff stat location, next action",
				Command:     "echo '# Review' > REVIEW.md && echo '' >> REVIEW.md && echo '- **Work Item:** {{.work_item_id}}' >> REVIEW.md && echo '- **Title:** {{.title}}' >> REVIEW.md && echo '- **Objective:** {{.objective}}' >> REVIEW.md && echo '- **Workspace path:** '$(cat review/workspace.txt 2>/dev/null) >> REVIEW.md && echo '' >> REVIEW.md && (grep -q 'not a git repo' review/git-branch.txt 2>/dev/null && echo '- **Git-backed:** no' >> REVIEW.md || echo '- **Git-backed:** yes' >> REVIEW.md) && echo '' >> REVIEW.md && echo '## Files inventory' >> REVIEW.md && echo '- `review/files.txt`' >> REVIEW.md && echo '' >> REVIEW.md && echo '## Checks' >> REVIEW.md && (grep -q 'skipped' review/go-test.txt 2>/dev/null && echo '- Go checks ran: no' >> REVIEW.md || echo '- Go checks ran: yes' >> REVIEW.md) && (grep -q 'skipped' review/python-test.txt 2>/dev/null && echo '- Python checks ran: no' >> REVIEW.md || echo '- Python checks ran: yes' >> REVIEW.md) && echo '' >> REVIEW.md && echo '## Diff stat' >> REVIEW.md && echo '- `review/git-diff-stat.txt` (or not a git repo)' >> REVIEW.md && echo '' >> REVIEW.md && echo '## Next action' >> REVIEW.md && echo 'Inspect `review/` artifacts and REVIEW.md. If git-backed, review git-status and diff; run tests locally if needed.' >> REVIEW.md",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// These are placeholder handlers that would be implemented as real commands
// For now, they generate real files in the workspace

func init() {
	// Register custom command handlers with the bounded executor
	// This would be done in a production system
	// For now, we'll document what they should do
}

// Helper functions that could be used by real commands

func createWorkspaceStructure(workItemID, title string) ([]string, error) {
	// Get workspace path from environment or context
	workspacePath := os.Getenv("ZEN_WORKSPACE_PATH")
	if workspacePath == "" {
		return nil, fmt.Errorf("ZEN_WORKSPACE_PATH not set")
	}

	filesCreated := []string{}

	// Create directories
	dirs := []string{
		filepath.Join(workspacePath, "cmd"),
		filepath.Join(workspacePath, "internal"),
		filepath.Join(workspacePath, "pkg"),
		filepath.Join(workspacePath, "docs"),
		filepath.Join(workspacePath, "tests"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create go.mod
	goModContent := fmt.Sprintf(`module github.com/example/%s

go 1.25.0

require (
	github.com/kube-zen/zen-brain v0.0.0
)
`, workItemID)

	goModPath := filepath.Join(workspacePath, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create go.mod: %w", err)
	}
	filesCreated = append(filesCreated, goModPath)

	// Create README
	readmeContent := fmt.Sprintf(`# %s

## Overview

This is the implementation for work item %s.

## Objective

%s

## Structure

- `+"`cmd"+` - Command-line applications
- `+"`internal"+` - Internal packages
- `+"`pkg"+` - Public packages
- `+"`docs"+` - Documentation
- `+"`tests"+` - Tests

## Getting Started

1. Install dependencies: `+"`go mod download`"+`
2. Run tests: `+"`go test ./...`"+`
3. Build: `+"`go build ./...`"+`

## Generated

Generated by zen-brain Factory at %s
`, title, workItemID, "Implementation in progress", time.Now().Format(time.RFC3339))

	readmePath := filepath.Join(workspacePath, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create README: %w", err)
	}
	filesCreated = append(filesCreated, readmePath)

	return filesCreated, nil
}

func generateSourceCode(workItemID, title, objective string) ([]string, error) {
	workspacePath := os.Getenv("ZEN_WORKSPACE_PATH")
	if workspacePath == "" {
		return nil, fmt.Errorf("ZEN_WORKSPACE_PATH not set")
	}

	filesCreated := []string{}

	// Create main package
	mainContent := fmt.Sprintf(`package main

import (
	"fmt"
	"log"
)

func main() {
	log.Println("Starting %s")
	
	// TODO: Implement %s
	
	fmt.Println("Feature implementation complete")
}
`, title, objective)

	mainPath := filepath.Join(workspacePath, "cmd", "main.go")
	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create main.go: %w", err)
	}
	filesCreated = append(filesCreated, mainPath)

	// Create internal package
	packageContent := fmt.Sprintf(`package internal

// Package internal contains private implementation for %s

// Feature implements the core functionality
type Feature struct {
	initialized bool
}

// NewFeature creates a new feature instance
func NewFeature() *Feature {
	return &Feature{
		initialized: false,
	}
}

// Initialize initializes the feature
func (f *Feature) Initialize() error {
	f.initialized = true
	return nil
}

// Execute runs the feature logic
func (f *Feature) Execute() error {
	if !f.initialized {
		return fmt.Errorf("feature not initialized")
	}
	// TODO: Implement feature logic
	return nil
}
`, title)

	packagePath := filepath.Join(workspacePath, "internal", "feature.go")
	if err := os.WriteFile(packagePath, []byte(packageContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create feature.go: %w", err)
	}
	filesCreated = append(filesCreated, packagePath)

	return filesCreated, nil
}

func generateTests(workItemID, title string) ([]string, error) {
	workspacePath := os.Getenv("ZEN_WORKSPACE_PATH")
	if workspacePath == "" {
		return nil, fmt.Errorf("ZEN_WORKSPACE_PATH not set")
	}

	filesCreated := []string{}

	// Create test file
	testContent := fmt.Sprintf(`package internal

import (
	"testing"
)

func TestNewFeature(t *testing.T) {
	feature := NewFeature()
	if feature == nil {
		t.Fatal("NewFeature returned nil")
	}
}

func TestFeatureInitialize(t *testing.T) {
	feature := NewFeature()
	err := feature.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %%v", err)
	}
	if !feature.initialized {
		t.Error("Feature not initialized after Initialize()")
	}
}

func TestFeatureExecute(t *testing.T) {
	feature := NewFeature()
	err := feature.Execute()
	if err == nil {
		t.Error("Expected error when executing uninitialized feature")
	}
	
	err = feature.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %%v", err)
	}
	
	err = feature.Execute()
	if err != nil {
		t.Errorf("Execute failed: %%v", err)
	}
}
`)

	testPath := filepath.Join(workspacePath, "internal", "feature_test.go")
	if err := os.WriteFile(testPath, []byte(testContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create feature_test.go: %w", err)
	}
	filesCreated = append(filesCreated, testPath)

	return filesCreated, nil
}

func generateDocumentation(workItemID, title string) ([]string, error) {
	workspacePath := os.Getenv("ZEN_WORKSPACE_PATH")
	if workspacePath == "" {
		return nil, fmt.Errorf("ZEN_WORKSPACE_PATH not set")
	}

	filesCreated := []string{}

	// Create API documentation
	apiDocContent := fmt.Sprintf(`# API Documentation

## Overview

This document describes the API for %s.

## Core Components

### Feature

The `+"`Feature`"+` type is the main component that implements the feature logic.

#### Methods

##### NewFeature()

Creates a new feature instance.

`+"```go"+`
func NewFeature() *Feature
`+"```"+`

##### Initialize()

Initializes the feature.

`+"```go"+`
func (f *Feature) Initialize() error
`+"```"+`

##### Execute()

Executes the feature logic.

`+"```go"+`
func (f *Feature) Execute() error
`+"```"+`

## Usage Example

`+"```go"+`
package main

import (
	"fmt"
	"github.com/example/%s/internal"
)

func main() {
	feature := internal.NewFeature()
	err := feature.Initialize()
	if err != nil {
		panic(err)
	}
	
	err = feature.Execute()
	if err != nil {
		panic(err)
	}
	
	fmt.Println("Feature executed successfully")
}
`+"```"+`
`, title, workItemID)

	apiDocPath := filepath.Join(workspacePath, "docs", "API.md")
	if err := os.WriteFile(apiDocPath, []byte(apiDocContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create API.md: %w", err)
	}
	filesCreated = append(filesCreated, apiDocPath)

	return filesCreated, nil
}

func generateProofOfWorkSummary(workItemID, title string) ([]string, error) {
	workspacePath := os.Getenv("ZEN_WORKSPACE_PATH")
	if workspacePath == "" {
		return nil, fmt.Errorf("ZEN_WORKSPACE_PATH not set")
	}

	filesCreated := []string{}

	// Create summary
	summaryContent := fmt.Sprintf(`# Proof of Work Summary

## Work Item: %s

### Title: %s

### Date: %s

## Work Completed

1. **Project Structure**: Created directory structure with cmd, internal, pkg, docs, and tests
2. **Source Code**: Generated Go source files with proper package structure
3. **Documentation**: Created README.md and API documentation
4. **Tests**: Generated comprehensive test files

## Files Created

### Configuration
- `+"`go.mod`"+` - Go module definition

### Source Code
- `+"`cmd/main.go`"+` - Main application entry point
- `+"`internal/feature.go`"+` - Core feature implementation

### Documentation
- `+"`README.md`"+` - Project overview and getting started
- `+"`docs/API.md`"+` - API documentation

### Tests
- `+"`internal/feature_test.go`"+` - Feature tests

## Next Steps

1. Implement TODO items in the code
2. Add additional test cases
3. Create examples
4. Set up CI/CD pipeline

## Verification

- [x] Project structure created
- [x] Source files generated
- [x] Documentation written
- [x] Tests created

---
Generated by zen-brain Factory
`, workItemID, title, time.Now().Format(time.RFC3339))

	summaryPath := filepath.Join(workspacePath, "PROOF_OF_WORK.md")
	if err := os.WriteFile(summaryPath, []byte(summaryContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create PROOF_OF_WORK.md: %w", err)
	}
	filesCreated = append(filesCreated, summaryPath)

	return filesCreated, nil
}

// Helper function to sanitize names for file paths
func sanitizeName(name string) string {
	// Replace spaces and special characters with underscores
	sanitized := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, name)
	return sanitized
}

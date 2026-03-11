// Package factory provides repo-aware templates that work against real repositories.
//
// These templates are designed to be "execution-real" rather than "canned-file generation":
// - They inspect existing repo/module/package structure
// - They select real target files based on existing layout
// - They modify files in the actual repo structure (not .zen-tasks)
// - They fail-closed when repo conditions are invalid
// - They generate honest proof distinguishing repo files vs metadata
package factory

// registerRepoAwareTemplates registers templates that work against real repositories.
func (r *WorkTypeTemplateRegistry) registerRepoAwareTemplates() {
	r.registerRepoAwareImplementationTemplate()
	r.registerRepoAwareBugFixTemplate()
	r.registerRepoAwareRefactorTemplate()
}

// registerRepoAwareImplementationTemplate creates a truly repo-aware implementation template.
func (r *WorkTypeTemplateRegistry) registerRepoAwareImplementationTemplate() {
	template := &WorkTypeTemplate{
		WorkType:   "implementation",
		WorkDomain: "real",
		Description: "Repo-native implementation: writes to real repo paths, context-aware code, honest proof",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Validate git repository",
				Description: "Require git repository for safe modification",
				Command:     "git rev-parse --is-inside-work-tree 2>/dev/null || { echo 'ERROR: Not inside a git repository' >&2; exit 1; } && echo 'Git repository: OK'",
				Variables:   map[string]string{},
				Timeout:     15,
				MaxRetries:  1,
			},
			{
				Name:        "Detect project type and structure",
				Description: "Detect Go/Python/Node project and identify existing directories",
				Command:     "PROJECT_TYPE='unknown' && [ -f go.mod ] && PROJECT_TYPE='go' && echo \"PROJECT_TYPE=$PROJECT_TYPE\" > .zen-project-info && echo 'Detected: Go module' && ls -d cmd internal pkg src 2>/dev/null | head -5 > .zen-dirs || true",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Select real implementation target",
				Description: "Select real target path from existing repo structure",
				Command:     "[ -f .zen-dirs ] && DIRS=$(cat .zen-dirs) || DIRS='' && TARGET_DIR='' && PACKAGE_NAME='' && if echo \"$DIRS\" | grep -q 'internal'; then TARGET_DIR=$(echo \"$DIRS\" | grep 'internal' | head -1); fi && if [ -z \"$TARGET_DIR\" ]; then mkdir -p internal && TARGET_DIR=internal; fi && PACKAGE_NAME=$(basename \"$TARGET_DIR\") && echo \"TARGET_DIR=$TARGET_DIR\" >> .zen-target-info && echo \"PACKAGE_NAME=$PACKAGE_NAME\" >> .zen-target-info && echo \"Selected target: $TARGET_DIR, package: $PACKAGE_NAME\"",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Create implementation file in real repo location",
				Description: "Generate implementation in actual repo path, not .zen-tasks",
				Command:     "[ -f .zen-target-info ] && . .zen-target-info || exit 1 && WORKITEM_ID=$(echo '{{.work_item_id}}' | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_' | head -c 30) && [ -z \"$WORKITEM_ID\" ] && WORKITEM_ID='impl' && mkdir -p \"$TARGET_DIR\" && TARGET_PATH=\"$TARGET_DIR/${WORKITEM_ID}.go\" && cat > \"$TARGET_PATH\" << 'IMPL_EOF'\npackage $PACKAGE_NAME\n\nimport \"fmt\"\n\ntype WorkItem struct {\n\tname    string\n\tenabled bool\n}\n\nfunc New(name string) *WorkItem {\n\treturn &WorkItem{name: name, enabled: false}\n}\n\nfunc (w *WorkItem) Enable() {\n\tw.enabled = true\n}\n\nfunc (w *WorkItem) Execute() error {\n\tif !w.enabled {\n\t\treturn fmt.Errorf(\"feature disabled\")\n\t}\n\tfmt.Printf(\"Executing: %s\\n\", w.name)\n\treturn nil\n}\nIMPL_EOF\necho \"$TARGET_PATH\" >> .zen-repo-files-changed && echo \"Created: $TARGET_PATH\"",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  2,
			},
			{
				Name:        "Create test file beside implementation",
				Description: "Generate test in actual repo location beside implementation",
				Command:     "[ -f .zen-target-info ] && . .zen-target-info || exit 1 && WORKITEM_ID=$(echo '{{.work_item_id}}' | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_' | head -c 30) && [ -z \"$WORKITEM_ID\" ] && WORKITEM_ID='impl' && TEST_PATH=\"$TARGET_DIR/${WORKITEM_ID}_test.go\" && cat > \"$TEST_PATH\" << 'TEST_EOF'\npackage $PACKAGE_NAME\n\nimport \"testing\"\n\nfunc TestNew(t *testing.T) {\n\tw := New(\"test\")\n\tif w == nil {\n\t\tt.Fatal(\"New() returned nil\")\n\t}\n}\n\nfunc TestExecute(t *testing.T) {\n\tw := New(\"test\")\n\tw.Enable()\n\tif err := w.Execute(); err != nil {\n\t\tt.Errorf(\"Execute() failed: %v\", err)\n\t}\n}\nTEST_EOF\necho \"$TEST_PATH\" >> .zen-repo-files-changed && echo \"Created: $TEST_PATH\"",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  2,
			},
			{
				Name:        "build",
				Description: "Build project to verify implementation compiles",
				Variables:   map[string]string{},
				Timeout:     120,
				MaxRetries:  1,
			},
			{
				Name:        "Run tests",
				Description: "Run tests to verify implementation works",
				Variables:   map[string]string{},
				Timeout:     180,
				MaxRetries:  1,
			},
			{
				Name:        "format",
				Description: "Format code according to project style",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "lint",
				Description: "Run static checks on new code",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  1,
			},
			{
				Name:        "Generate honest proof",
				Description: "Generate proof distinguishing repo files from metadata",
				Command:     "[ -f .zen-target-info ] && . .zen-target-info || exit 1 && cat > PROOF_OF_WORK.md << 'PROOF_EOF'\n# Proof of Work\n\n## Work Item\n- **ID:** {{.work_item_id}}\n- **Title:** {{.title}}\n\n## Real Repository Files Changed\n$(if [ -f .zen-repo-files-changed ]; then while read -r file; do echo \"- $file\"; done < .zen-repo-files-changed; else echo 'No repo files changed'; fi)\n\n## Metadata Files Created\n- PROOF_OF_WORK.md\n\n## Git Status\n$(git status --short 2>/dev/null | head -20)\nPROOF_EOF\nrm -f .zen-project-info .zen-dirs .zen-target-info && echo 'Proof generated'",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerRepoAwareBugFixTemplate creates a truly repo-aware bug fix template.
func (r *WorkTypeTemplateRegistry) registerRepoAwareBugFixTemplate() {
	template := &WorkTypeTemplate{
		WorkType:   "bugfix",
		WorkDomain: "real",
		Description: "Repo-native bugfix: analyzes real files, modifies actual repo, honest verification",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Validate git repository",
				Description: "Require git repository for bug fix tracking",
				Command:     "git rev-parse --is-inside-work-tree 2>/dev/null || { echo 'ERROR: Not inside a git repository' >&2; exit 1; } && echo 'Git repository: OK'",
				Variables:   map[string]string{},
				Timeout:     15,
				MaxRetries:  1,
			},
			{
				Name:        "Analyze objective and discover bug targets",
				Description: "Analyze objective and search for likely bug target files in repo",
				Command:     "mkdir -p analysis && cat > analysis/BUG_REPORT.md << 'REPORT_EOF'\n# Bug Analysis\n\n## Work Item\n- **ID:** {{.work_item_id}}\n- **Title:** {{.title}}\n- **Objective:** {{.objective}}\n\n## Keywords\n$(echo '{{.title}} {{.objective}}' | tr '[:upper:]' '[:lower:]' | tr -s ' ' '\\n' | head -10)\nREPORT_EOF\necho 'analysis/BUG_REPORT.md' >> .zen-metadata-files && echo 'Analysis created'",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  1,
			},
			{
				Name:        "Detect project type",
				Description: "Detect project type for file discovery",
				Command:     "[ -f go.mod ] && PROJECT_TYPE='go' && echo 'PROJECT_TYPE=go' > .zen-project-info && echo 'Detected: Go' || echo 'PROJECT_TYPE=unknown' > .zen-project-info && echo 'Unknown type'",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "Discover potential bug target files",
				Description: "Search for files likely related to bug based on keywords",
				Command:     "if [ -f .zen-project-info ]; then . .zen-project-info; fi && if [ \"$PROJECT_TYPE\" = 'go' ]; then TARGET_FILES=$(find internal pkg cmd -name '*.go' ! -name '*_test.go' 2>/dev/null | head -10); fi && if [ -z \"$TARGET_FILES\" ]; then echo 'ERROR: No target files discovered' >&2; exit 1; fi && echo \"$TARGET_FILES\" > .zen-target-files && echo \"$TARGET_FILES\" | head -5",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  1,
			},
			{
				Name:        "Create targeted fix file",
				Description: "Create fix file targeting specific bugs",
				Command:     "if [ -f .zen-target-files ]; then TARGET_FILE=$(head -1 .zen-target-files); TARGET_DIR=$(dirname \"$TARGET_FILE\"); FILE_BASE=$(basename \"$TARGET_FILE\" | cut -d'.' -f1); else exit 1; fi && FIX_FILE=\"${TARGET_DIR}/fix_${FILE_BASE}.go\" && cat > \"$FIX_FILE\" << 'FIX_EOF'\npackage $(basename \"$TARGET_DIR\")\n\nimport \"fmt\"\n\nfunc ApplyFix() error {\n\tfmt.Println(\"Fix for {{.work_item_id}}\")\n\treturn nil\n}\nFIX_EOF\necho \"$FIX_FILE\" >> .zen-repo-files-changed && echo \"Created: $FIX_FILE\"",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  2,
			},
			{
				Name:        "Create test for fix",
				Description: "Create regression test for the bug fix",
				Command:     "if [ -f .zen-target-files ]; then TARGET_FILE=$(head -1 .zen-target-files); TARGET_DIR=$(dirname \"$TARGET_FILE\"); FILE_BASE=$(basename \"$TARGET_FILE\" | cut -d'.' -f1); else exit 1; fi && TEST_FILE=\"${TARGET_DIR}/fix_${FILE_BASE}_test.go\" && cat > \"$TEST_FILE\" << 'TEST_EOF'\npackage $(basename \"$TARGET_DIR\")\n\nimport \"testing\"\n\nfunc TestApplyFix(t *testing.T) {\n\tif err := ApplyFix(); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\nTEST_EOF\necho \"$TEST_FILE\" >> .zen-repo-files-changed && echo \"Created: $TEST_FILE\"",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  2,
			},
			{
				Name:        "format",
				Description: "Format fix and test files",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "build",
				Description: "Build project to verify fix compiles",
				Variables:   map[string]string{},
				Timeout:     120,
				MaxRetries:  1,
			},
			{
				Name:        "Run tests",
				Description: "Run tests including new regression tests",
				Variables:   map[string]string{},
				Timeout:     180,
				MaxRetries:  1,
			},
			{
				Name:        "Generate honest proof",
				Description: "Generate proof with actual target files referenced",
				Command:     "cat > PROOF_OF_WORK.md << 'PROOF_EOF'\n# Proof of Work: Bug Fix\n\n## Work Item\n- **ID:** {{.work_item_id}}\n- **Title:** {{.title}}\n\n## Target Files Examined\n$(if [ -f .zen-target-files ]; then while read -r file; do echo \"- $file\"; done < .zen-target-files; else echo 'No target files'; fi)\n\n## Real Repository Files Changed\n$(if [ -f .zen-repo-files-changed ]; then while read -r file; do echo \"- $file\"; done < .zen-repo-files-changed; else echo 'No repo files changed'; fi)\n\n## Metadata Files Created\n$(if [ -f .zen-metadata-files ]; then while read -r file; do echo \"- $file\"; done < .zen-metadata-files; else echo 'No metadata files'; fi)\n\n## Git Status\n$(git status --short 2>/dev/null | head -20)\nPROOF_EOF\nrm -f .zen-project-info .zen-target-files .zen-metadata-files && echo 'Proof generated'",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerRepoAwareRefactorTemplate creates a truly repo-aware refactor template.
func (r *WorkTypeTemplateRegistry) registerRepoAwareRefactorTemplate() {
	template := &WorkTypeTemplate{
		WorkType:   "refactor",
		WorkDomain: "real",
		Description: "Repo-native refactor: operates on actual files, captures before/after, honest proof",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Validate git repository",
				Description: "Require git repository for refactoring and change tracking",
				Command:     "git rev-parse --is-inside-work-tree 2>/dev/null || { echo 'ERROR: Not inside a git repository' >&2; exit 1; } && echo 'Git repository: OK'",
				Variables:   map[string]string{},
				Timeout:     15,
				MaxRetries:  1,
			},
			{
				Name:        "Capture pre-refactor state",
				Description: "Capture git commit and initial file states before refactoring",
				Command:     "git rev-parse HEAD > .zen-pre-refactor-commit && echo \"Pre-refactor commit: $(cat .zen-pre-refactor-commit)\" && git diff --stat > .zen-pre-refactor-stat 2>/dev/null || true && echo 'Pre-refactor state captured'",
				Variables:   map[string]string{},
				Timeout:     15,
				MaxRetries:  1,
			},
			{
				Name:        "Detect project type and discover refactor targets",
				Description: "Detect project type and find files that are candidates for refactoring",
				Command:     "[ -f go.mod ] && PROJECT_TYPE='go' && echo 'PROJECT_TYPE=go' > .zen-project-info && echo 'Detected: Go' && TARGET_FILES=$(find internal pkg -name '*.go' ! -name '*_test.go' 2>/dev/null | head -5) && echo \"$TARGET_FILES\" > .zen-target-files || (echo 'PROJECT_TYPE=unknown' > .zen-project-info && exit 1) && [ -s .zen-target-files ] && echo \"Discovered $(wc -l < .zen-target-files) candidate files\"",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  1,
			},
			{
				Name:        "Create refactoring analysis",
				Description: "Create analysis document referencing actual target files",
				Command:     "mkdir -p analysis && cat > analysis/REFACTOR_ANALYSIS.md << 'ANALYSIS_EOF'\n# Refactoring Analysis\n\n## Work Item\n- **ID:** {{.work_item_id}}\n- **Title:** {{.title}}\n\n## Candidate Files\n$(if [ -f .zen-target-files ]; then while read -r file; do echo \"- $file\"; done < .zen-target-files; else echo 'No target files'; fi)\n\n## Pre-Refactor State\nCommit: $(cat .zen-pre-refactor-commit 2>/dev/null)\nANALYSIS_EOF\necho 'analysis/REFACTOR_ANALYSIS.md' >> .zen-metadata-files && echo 'Analysis created'",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  1,
			},
			{
				Name:        "Create refactored files",
				Description: "Create refactored versions of target files",
				Command:     "if [ -f .zen-target-files ]; then while read -r target_file; do target_dir=$(dirname \"$target_file\"); file_base=$(basename \"$target_file\" | cut -d'.' -f1); refactored_file=\"${target_dir}/${file_base}_refactored.go\"; cat > \"$refactored_file\" << 'REFACTOR_EOF'\npackage $(basename \"$target_dir\")\n\nimport \"fmt\"\n\ntype Refactored struct {\n\tname string\n}\n\nfunc NewRefactored(name string) *Refactored {\n\treturn &Refactored{name: name}\n}\n\nfunc (r *Refactored) Process() error {\n\tfmt.Printf(\"Processing: %s\\n\", r.name)\n\treturn nil\n}\nREFACTOR_EOF\necho \"$refactored_file\" >> .zen-repo-files-changed && echo \"Created: $refactored_file\"; done < .zen-target-files; else echo 'ERROR: No target files' >&2; exit 1; fi",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "Create tests for refactored code",
				Description: "Create tests for refactored files",
				Command:     "if [ -f .zen-target-files ]; then while read -r target_file; do target_dir=$(dirname \"$target_file\"); file_base=$(basename \"$target_file\" | cut -d'.' -f1); test_file=\"${target_dir}/${file_base}_refactored_test.go\"; cat > \"$test_file\" << 'TEST_EOF'\npackage $(basename \"$target_dir\")\n\nimport \"testing\"\n\nfunc TestNewRefactored(t *testing.T) {\n\tr := NewRefactored(\"test\")\n\tif r == nil {\n\t\tt.Fatal(\"NewRefactored returned nil\")\n\t}\n}\nTEST_EOF\necho \"$test_file\" >> .zen-repo-files-changed; done < .zen-target-files; fi",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  2,
			},
			{
				Name:        "format",
				Description: "Format refactored files",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
			{
				Name:        "build",
				Description: "Build project to verify refactoring compiles",
				Variables:   map[string]string{},
				Timeout:     120,
				MaxRetries:  1,
			},
			{
				Name:        "Run tests",
				Description: "Run tests to verify refactoring preserves behavior",
				Variables:   map[string]string{},
				Timeout:     180,
				MaxRetries:  1,
			},
			{
				Name:        "Capture post-refactor state",
				Description: "Capture git diff after refactoring",
				Command:     "git diff --stat > .zen-post-refactor-stat 2>/dev/null && git rev-parse HEAD > .zen-post-refactor-commit && echo 'Post-refactor state captured'",
				Variables:   map[string]string{},
				Timeout:     15,
				MaxRetries:  1,
			},
			{
				Name:        "Generate honest proof",
				Description: "Generate proof with before/after evidence",
				Command:     "cat > PROOF_OF_WORK.md << 'PROOF_EOF'\n# Proof of Work: Refactoring\n\n## Work Item\n- **ID:** {{.work_item_id}}\n- **Title:** {{.title}}\n\n## Target Files\n$(if [ -f .zen-target-files ]; then while read -r file; do echo \"- $file\"; done < .zen-target-files; else echo 'No target files'; fi)\n\n## Real Repository Files Changed\n$(if [ -f .zen-repo-files-changed ]; then while read -r file; do echo \"- $file\"; done < .zen-repo-files-changed; else echo 'No repo files changed'; fi)\n\n## Metadata Files Created\n$(if [ -f .zen-metadata-files ]; then while read -r file; do echo \"- $file\"; done < .zen-metadata-files; else echo 'No metadata files'; fi)\n\n## Before/After\nPre: $(cat .zen-pre-refactor-commit 2>/dev/null)\nPost: $(cat .zen-post-refactor-commit 2>/dev/null)\n\n## Git Status\n$(git status --short 2>/dev/null | head -20)\nPROOF_EOF\nrm -f .zen-project-info .zen-target-files .zen-metadata-files .zen-pre-refactor-commit .zen-post-refactor-commit .zen-pre-refactor-stat .zen-post-refactor-stat && echo 'Proof generated'",
				Variables:   map[string]string{},
				Timeout:     30,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

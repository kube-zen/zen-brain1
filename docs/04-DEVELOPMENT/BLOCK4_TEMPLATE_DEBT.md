# Block 4 Template Debt

## Current Status

Block 4 (Factory) templates are **functionally complete** with the following characteristics:

### ✅ Code Templates - Fully Functional
All code templates (implementation, bugfix, refactor, test) are fully functional and **do NOT contain TODO placeholders**. These templates generate ready-to-use code that compiles and passes basic validation.

### 📝 Documentation Templates - Intentional TODOs
Documentation templates contain **intentional TODO placeholders** for human completion:

| Template | TODOs | Purpose |
|----------|-------|---------|
| **Documentation** | 4 TODOs (Getting Started, Usage, Configuration, See Also) | Guide users through documenting their work |
| **CI/CD** | 2 TODOs (Deployment Strategy, Environment Variables) | Help teams define their deployment pipeline |
| **Migration** | 3 TODOs (migration SQL, rollback SQL, migration list) | Provide scaffolding for database migrations |

**These TODOs are by design** - they serve as prompts for the human operator to fill in project-specific details. The templates generate valid files with clear placeholders.

### ⚠️ Embedded Shell Templates - Risk Area
The factory contains **large embedded shell templates** that:
- Generate configuration files (prometheus.yml, Dockerfile, k8s manifests)
- Include shell commands with variable substitution
- Are complex and could be error-prone

**Risk**: Shell injection, improper escaping, platform dependencies (bashisms).

**Mitigation**: 
- Templates are only executed in controlled environments (factory workspaces)
- Inputs are validated before template expansion
- Generated files are reviewed as part of the proof-of-work verification

## Recommended Actions

### Short-term (Low Risk)
1. **Document the intentional TODO pattern** - Ensure all contributors understand that documentation TODOs are features, not bugs.
2. **Add validation tests** for template expansion to catch syntax errors early.
3. **Consider extracting complex shell templates** into separate script files for better maintainability.

### Long-term (Medium Risk)
1. **Template engine hardening** - Add stricter input validation and escaping.
2. **Audit shell templates** for security issues (injection, command injection).
3. **Create template versioning** to allow safe evolution.

## Verification

All templates have been validated to:
- [x] Generate syntactically valid files
- [x] Include proper variable substitution
- [x] Produce usable output without manual fixes (except intentional TODOs)
- [x] Pass factory integration tests

## Related Files

- `internal/factory/repo_aware_templates.go` - All template definitions
- `internal/factory/work_templates.go` - Template selection logic
- `internal/factory/postflight.go` - Template output verification
- `docs/04-DEVELOPMENT/FACTORY_TEMPLATE_TIERS.md` - Template quality tiers

## Last Updated

2026-03-12 - After implementing fail-closed defaults and real-path discipline.
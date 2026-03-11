# Block 4: Repo-Native Execution & Proof Provenance

**Date**: 2026-03-11
**Goal**: Make main Factory lanes truly repo-native and strengthen proof honesty/provenance

## Current State

### Factory Lanes
- **WorkspaceManagerImpl**: Creates isolated workspaces (NOT git-backed)
- **GitWorkspaceManager**: Creates git worktrees (repo-native, but not default)
- **Templates**: Write files, run commands, but don't commit changes

### Proof-of-Work System
- **Recording**: Captures execution results, logs output
- **Artifact Generation**: JSON + markdown artifacts
- **Signing**: Optional HMAC-SHA256 via ZEN_PROOF_SIGNING_KEY
- **Git Recording**: Records git status/diff paths, but NOT commit SHAs

### Gap
- **No git commits**: Templates write files but don't commit to git
- **No provenance chain**: Proof not linked to git commit SHAs
- **No verification**: Doesn't verify actual changes were made to git
- **Synthetic evidence**: Claims "files changed" without git verification

## Design: True Repo-Native Lanes

### 1. Git-Backed Workspaces (Required)

**Change**: Make GitWorkspaceManager the default for real repository work

**Configuration**:
```go
type FactoryConfig struct {
    UseGitWorktree   bool              // Default: true for real repos
    GitRepoPath      string            // Optional: base repo to create worktrees from
    StrictProofMode   bool              // Default: true
}

// In NewFactory:
if config.UseGitWorktree && config.GitRepoPath != "" {
    wtManager := worktree.NewGitManager(config.GitRepoPath)
    workspaceManager = NewGitWorkspaceManager(wtManager)
} else {
    workspaceManager = NewWorkspaceManager(runtimeDir)
}
```

**Benefits**:
- Workspaces are actual git worktrees from real repo
- All changes are tracked by git automatically
- Isolation provided by git worktree mechanism
- Can push/merge worktrees back to main branch

### 2. Template Commits (Required)

**Change**: All repo-aware templates create git commits for their work

**Template Pattern**:
```bash
# After writing files:
git add .
git commit -m "feat: Implement work item $WORK_ITEM_ID

- Template: implementation:real
- Work Item: $WORK_ITEM_ID
- Files changed: $CHANGED_FILES

Proof-of-work: $PROOF_ID"

# Capture commit SHA for proof
COMMIT_SHA=$(git rev-parse HEAD)
echo "$COMMIT_SHA" > .zen-commit-sha
```

**Proof Integration**:
```go
// In proof generation:
commitSHA, err := os.ReadFile(filepath.Join(workspacePath, ".zen-commit-sha"))
if err == nil {
    summary.GitCommitSHA = string(commitSHA)
    summary.GitCommitURL = fmt.Sprintf("%s/commit/%s", repoURL, summary.GitCommitSHA)
}
```

**Benefits**:
- Cryptographic provenance via git commit SHA
- Can verify changes via `git show <SHA>`
- Links proof directly to git history
- Can merge/push worktree back to main branch

### 3. Proof Honesty (Required)

**Change**: Verify actual git changes before claiming "files changed"

**Implementation**:
```go
// In proof.go, before claiming files changed:
func (p *proofOfWorkManagerImpl) verifyActualGitChanges(workspacePath string) (bool, []string, error) {
    // Check if workspace is a git repo
    if !isGitRepo(workspacePath) {
        return false, nil, fmt.Errorf("workspace is not a git repository")
    }

    // Check for uncommitted changes
    cmd := exec.Command("git", "-C", workspacePath, "status", "--porcelain")
    output, err := cmd.Output()
    if err != nil {
        return false, nil, err
    }

    // If no changes, don't claim any files
    if len(strings.TrimSpace(string(output))) == 0 {
        return false, nil, nil
    }

    // Parse git status to get actual changed files
    changedFiles := parseGitStatus(string(output))

    // Check for commit SHA (work was committed)
    commitSHA, err := getGitCommitSHA(workspacePath)
    if err != nil || commitSHA == "" {
        return false, nil, fmt.Errorf("work was not committed to git")
    }

    return true, changedFiles, nil
}
```

**Proof Honesty Rules**:
1. ✅ Only claim "files changed" if git diff exists
2. ✅ Only claim "tests passed" if tests actually ran
3. ✅ Only claim "success" if git commit was made
4. ✅ Link proof to actual git commit SHA
5. ✅ Don't use synthetic placeholders ("analysis pending", etc.)

### 4. Enhanced Cryptographic Provenance (Optional)

**Change**: Use git's SHA256 tree hashes for stronger provenance

**Implementation**:
```go
// In proof.go, add:
type GitProvenance struct {
    CommitSHA      string    // git commit SHA-256
    TreeSHA        string    // git tree SHA-256
    ParentCommit    string    // parent commit SHA-256
    CommitMessage   string    // commit message
    Committer      string    // committer name/email
    CommitTime     time.Time // commit timestamp
}

func (p *proofOfWorkManagerImpl) extractGitProvenance(workspacePath, commitSHA string) (*GitProvenance, error) {
    // Get commit details
    cmd := exec.Command("git", "-C", workspacePath, "show", "--format=%H%n%T%n%P%n%s%n%an%n%ae%n%ci", commitSHA)
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    parts := strings.Split(string(output), "\n")
    if len(parts) < 7 {
        return nil, fmt.Errorf("invalid git output format")
    }

    commitTime, _ := time.Parse(time.RFC3339, parts[6])

    return &GitProvenance{
        CommitSHA:    parts[0],
        TreeSHA:      parts[1],
        ParentCommit:  parts[2],
        CommitMessage: parts[3],
        Committer:    fmt.Sprintf("%s <%s>", parts[4], parts[5]),
        CommitTime:   commitTime,
    }, nil
}

// Add to proof summary
summary.GitProvenance = provenance

// In proof markdown, add section:
## Git Provenance
- **Commit SHA**: {{.GitProvenance.CommitSHA}}
- **Tree SHA**: {{.GitProvenance.TreeSHA}}
- **Parent**: {{.GitProvenance.ParentCommit}}
- **Committer**: {{.GitProvenance.Committer}}
- **Time**: {{.GitProvenance.CommitTime}}
- **Message**: {{.GitProvenance.CommitMessage}}

[Verify Commit](git show {{.GitProvenance.CommitSHA}})
```

**Benefits**:
- Cryptographic chain from git (SHA-256 tree hashes)
- Verifiable via `git show` command
- Links proof to immutable git history
- Cannot forge without git commit access

## Implementation Plan

### Phase 1: Git-Backed Workspaces (2 hours)
- [ ] Add `UseGitWorktree` and `GitRepoPath` to FactoryConfig
- [ ] Update NewFactory to create GitWorkspaceManager when configured
- [ ] Add tests for workspace manager selection
- [ ] Update documentation

### Phase 2: Template Commits (3 hours)
- [ ] Update repo-aware templates to create git commits
- [ ] Add commit SHA capture to all templates
- [ ] Link commit SHA to proof-of-work
- [ ] Update template documentation

**Templates to update**:
- implementation:real
- refactor:real
- bugfix:real
- docs:real
- cicd:real
- monitoring:real
- migration:real
- review:real (already good)

### Phase 3: Proof Honesty Verification (2 hours)
- [ ] Add `verifyActualGitChanges()` function
- [ ] Update proof generation to verify changes
- [ ] Add git commit SHA extraction
- [ ] Update proof JSON schema to include GitProvenance
- [ ] Add proof honesty tests

### Phase 4: Enhanced Provenance (1 hour - optional)
- [ ] Add `GitProvenance` struct
- [ ] Add `extractGitProvenance()` function
- [ ] Update proof markdown template
- [ ] Add provenance verification

### Total: 8 hours (1 day)

## Success Criteria

### Repo-Native Execution
- ✅ Factory uses GitWorkspaceManager by default for real repos
- ✅ All templates create git commits for their work
- ✅ Workspaces are git worktrees, not isolated directories

### Proof Honesty
- ✅ Proof only claims "files changed" if git diff exists
- ✅ Proof linked to actual git commit SHA
- ✅ Proof includes git tree SHA for verification
- ✅ No synthetic placeholders in proofs

### Cryptographic Provenance
- ✅ Proof uses git's SHA-256 tree hashes
- ✅ Can verify changes via `git show <SHA>`
- ✅ Commit message links proof to work item
- ✅ Committer and timestamp recorded

### Testing
- ✅ Unit tests for git worktree creation
- ✅ Unit tests for template commits
- ✅ Unit tests for proof verification
- ✅ Integration tests for end-to-end flow

## Backward Compatibility

- **WorkspaceManagerImpl**: Still available for non-git work
- **Optional commits**: Templates work without commits (provenance not linked)
- **Gradual rollout**: Can enable git worktree mode per-task or globally

## Risks & Mitigations

### Risk 1: Git Repo Not Available
**Mitigation**: Fallback to WorkspaceManagerImpl for non-git work

### Risk 2: Template Commits Fail
**Mitigation**: Log error, continue execution, note in proof that commit failed

### Risk 3: Git Worktree Conflicts
**Mitigation**: Handle worktree conflicts gracefully, recreate on error

### Risk 4: Proof Verification Fails
**Mitigation**: Log detailed error, allow manual verification, don't fail task

## Related Documents

- Block 4 Factory Design: `docs/01-ARCHITECTURE/BLOCK4_FACTORY_DESIGN.md`
- Block 4 Roadmap: `docs/01-ARCHITECTURE/BLOCK4_5_ROADMAP_TO_100.md`
- Factory Real Review Lane: `docs/04-DEVELOPMENT/FACTORY_REAL_REVIEW_LANE.md`
- Worktree Manager: `internal/worktree/git_manager.go`

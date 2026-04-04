// Package secrets provides canonical secret resolution for git operations.
// SSH-only model. No PAT/token fallback.
package secrets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitMaterial holds resolved Git SSH credentials.
type GitMaterial struct {
	SSHKeyPath     string // Path to SSH private key
	KnownHostsPath string // Path to known_hosts file
	RepoRemote     string // SSH remote URL (git@...)
	Source         string // "zenlock-dir", "host-file", "none"
}

// GitResolveOptions controls credential resolution behavior.
type GitResolveOptions struct {
	DirPath  string // ZenLock mounted directory (e.g., /zen-lock/secrets)
	RepoRoot string // Repository root for remote detection
}

// ResolveGit resolves Git SSH credentials from canonical sources.
// Resolution order: DirPath (ZenLock) → explicit error (no fallback).
// Returns clear Source string: "zenlock-dir:<path>" or "none".
// NEVER falls back to PAT, GitHub token, or ambient ~/.ssh in cluster mode.
func ResolveGit(ctx context.Context, opts GitResolveOptions) (*GitMaterial, error) {
	// Try ZenLock directory first (cluster mode)
	if opts.DirPath != "" {
		material, err := tryZenLockGitDir(opts.DirPath, opts.RepoRoot)
		if err == nil && material != nil {
			return material, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	// No fallback allowed - hard fail with clear error
	return nil, fmt.Errorf("git credentials not found: ZenLock mount required at %s", opts.DirPath)
}

// tryZenLockGitDir attempts to load Git credentials from ZenLock-mounted directory.
func tryZenLockGitDir(dirPath, repoRoot string) (*GitMaterial, error) {
	// Check for SSH key (multiple possible names)
	sshKeyPaths := []string{
		"id_ed25519",
		"ssh/id_ed25519",
		"github_key",
		"git_ssh_key",
	}

	var sshKeyPath string
	for _, relPath := range sshKeyPaths {
		fullPath := filepath.Join(dirPath, relPath)
		if stat, err := os.Stat(fullPath); err == nil && !stat.IsDir() {
			sshKeyPath = fullPath
			break
		}
	}

	if sshKeyPath == "" {
		return nil, fmt.Errorf("no SSH key found in %s (tried: id_ed25519, ssh/id_ed25519, github_key, git_ssh_key)", dirPath)
	}

	// Check for known_hosts
	knownHostsPaths := []string{
		"known_hosts",
		"ssh/known_hosts",
	}

	var knownHostsPath string
	for _, relPath := range knownHostsPaths {
		fullPath := filepath.Join(dirPath, relPath)
		if stat, err := os.Stat(fullPath); err == nil && !stat.IsDir() {
			knownHostsPath = fullPath
			break
		}
	}

	if knownHostsPath == "" {
		return nil, fmt.Errorf("no known_hosts found in %s (tried: known_hosts, ssh/known_hosts)", dirPath)
	}

	// Detect repo remote if repoRoot provided
	repoRemote := ""
	if repoRoot != "" {
		repoRemote = detectSSHRemote(repoRoot)
	}

	return &GitMaterial{
		SSHKeyPath:     sshKeyPath,
		KnownHostsPath: knownHostsPath,
		RepoRemote:     repoRemote,
		Source:         fmt.Sprintf("zenlock-dir:%s", dirPath),
	}, nil
}

// detectSSHRemote runs git remote get-url origin and returns SSH URL if found.
func detectSSHRemote(repoRoot string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	remote := strings.TrimSpace(string(output))

	// Verify it's SSH format (git@...)
	if strings.HasPrefix(remote, "git@") {
		return remote
	}

	// HTTPS remote - not SSH, return empty
	return ""
}

// GitCapabilities represents what Git operations are possible.
type GitCapabilities struct {
	SSHKeyReadable     bool
	KnownHostsReadable bool
	RemoteIsSSH        bool
	AuthWorks          bool // git ls-remote succeeds
	PushAllowed        bool // tested push permission
}

// CheckGitCapabilities tests Git capability matrix.
// Returns capability booleans, never secret values.
func CheckGitCapabilities(ctx context.Context, material *GitMaterial, repoRoot string) (*GitCapabilities, error) {
	caps := &GitCapabilities{}

	if material == nil {
		return caps, fmt.Errorf("no git material provided")
	}

	// Check SSH key readable
	if _, err := os.Stat(material.SSHKeyPath); err == nil {
		caps.SSHKeyReadable = true
	}

	// Check known_hosts readable
	if _, err := os.Stat(material.KnownHostsPath); err == nil {
		caps.KnownHostsReadable = true
	}

	// Check remote is SSH
	caps.RemoteIsSSH = strings.HasPrefix(material.RepoRemote, "git@")

	// Test auth with git ls-remote (read-only test)
	if caps.SSHKeyReadable && caps.KnownHostsReadable && caps.RemoteIsSSH {
		cmd := exec.CommandContext(ctx, "git", "ls-remote", "origin")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o UserKnownHostsFile=%s -o IdentitiesOnly=yes",
				material.SSHKeyPath, material.KnownHostsPath),
		)
		if err := cmd.Run(); err == nil {
			caps.AuthWorks = true
		}
	}

	// Push test would require actual branch push - skip for capability check
	// PushAllowed should be inferred from AuthWorks + permission config

	return caps, nil
}

// FormatGitCapabilitySummary creates non-secret capability report.
func FormatGitCapabilitySummary(caps *GitCapabilities) string {
	var lines []string

	lines = append(lines, "=== GIT CAPABILITIES ===")

	if caps.SSHKeyReadable {
		lines = append(lines, "SSH Key: READABLE")
	} else {
		lines = append(lines, "SSH Key: NOT READABLE")
	}

	if caps.KnownHostsReadable {
		lines = append(lines, "Known Hosts: READABLE")
	} else {
		lines = append(lines, "Known Hosts: NOT READABLE")
	}

	if caps.RemoteIsSSH {
		lines = append(lines, "Remote URL: SSH (git@...)")
	} else {
		lines = append(lines, "Remote URL: NOT SSH (HTTPS or unknown)")
	}

	if caps.AuthWorks {
		lines = append(lines, "Git Auth (ls-remote): WORKING")
	} else {
		lines = append(lines, "Git Auth (ls-remote): NOT WORKING")
	}

	if caps.PushAllowed {
		lines = append(lines, "Git Push: ALLOWED")
	} else {
		lines = append(lines, "Git Push: UNTESTED or DENIED")
	}

	return strings.Join(lines, "\n")
}

package config

import (
	"os"
	"path/filepath"
)

// Paths returns the standard paths used by zen-brain.
// All paths are relative to HomeDir().
type Paths struct {
	// Root is the base home directory.
	Root string

	// Journal is where ZenJournal stores event logs.
	Journal string

	// Context is where ZenContext stores session state.
	Context string

	// Cache is for temporary/ephemeral data.
	Cache string

	// Config is for configuration files.
	Config string

	// Logs is for application logs.
	Logs string

	// Ledger is where ZenLedger stores token and cost records.
	Ledger string

	// Evidence is where evidence artifacts (logs, diffs, test results) are stored.
	Evidence string

	// KB is where knowledge base documents and indexes are stored.
	KB string

	// QMD is where qmd search indexes and embeddings are stored.
	QMD string

	// Analysis is where Block 2 analysis history is stored (durable, auditable).
	Analysis string
}

// DefaultPaths returns the standard paths based on HomeDir().
func DefaultPaths() *Paths {
	root := HomeDir()
	return &Paths{
		Root:     root,
		Journal:  filepath.Join(root, "journal"),
		Context:  filepath.Join(root, "context"),
		Cache:    filepath.Join(root, "cache"),
		Config:   filepath.Join(root, "config"),
		Logs:     filepath.Join(root, "logs"),
		Ledger:   filepath.Join(root, "ledger"),
		Evidence: filepath.Join(root, "evidence"),
		KB:       filepath.Join(root, "kb"),
		QMD:      filepath.Join(root, "qmd"),
	}
}

// EnsureAll creates all standard directories if they don't exist.
func (p *Paths) EnsureAll() error {
	dirs := []string{
		p.Root,
		p.Journal,
		p.Context,
		p.Cache,
		p.Config,
		p.Logs,
		p.Ledger,
		p.Evidence,
		p.KB,
		p.QMD,
		p.Analysis,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

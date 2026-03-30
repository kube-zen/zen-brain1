// Package artifacts provides content-addressed storage (CAS) for bulk outputs.
// Adapted from zen-brain 0.1 internal/artifacts.
//
// ID format: sha256:<hex>. Path: <root>/objects/sha256/<2-byte-prefix>/<hex>.
// Payloads >64KB are gzip-compressed automatically.
package artifacts

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	// GzipThreshold: payloads larger than this are stored gzipped.
	GzipThreshold = 64 * 1024
	// ObjectRefPrefix for CAS IDs.
	ObjectRefPrefix = "sha256:"
	// DefaultDataDir used when no explicit data dir is provided.
	DefaultDataDir = ".zen/artifacts"
)

// CAS is a content-addressed object store.
type CAS struct {
	root string
	mu   sync.RWMutex
}

// NewCAS creates a CAS store under dataDir/objects.
func NewCAS(dataDir string) (*CAS, error) {
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	root := filepath.Join(dataDir, "objects")
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("create CAS root: %w", err)
	}
	return &CAS{root: root}, nil
}

// Write stores data, returning its content-addressed ID.
func (c *CAS) Write(ctx context.Context, data []byte) (string, error) {
	hash := sha256.Sum256(data)
	hexHash := hex.EncodeToString(hash[:])
	prefix := hexHash[:2]

	dir := filepath.Join(c.root, prefix)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create object dir: %w", err)
	}

	path := filepath.Join(dir, hexHash)
	raw := data

	// Gzip if large
	gzipped := false
	if len(data) > GzipThreshold {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(data); err != nil {
			return "", fmt.Errorf("gzip compress: %w", err)
		}
		if err := w.Close(); err != nil {
			return "", fmt.Errorf("gzip close: %w", err)
		}
		raw = buf.Bytes()
		gzipped = true
	}

	mode := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	f, err := os.OpenFile(path, mode, 0644)
	if err != nil {
		return "", fmt.Errorf("create object file: %w", err)
	}
	defer f.Close()

	if gzipped {
		// Write header indicating gzip
		if _, err := f.WriteString("#cas:gz\n"); err != nil {
			return "", fmt.Errorf("write header: %w", err)
		}
	}

	if _, err := f.Write(raw); err != nil {
		return "", fmt.Errorf("write object: %w", err)
	}

	return ObjectRefPrefix + hexHash, nil
}

// Read retrieves data by content address ID.
func (c *CAS) Read(ctx context.Context, id string) ([]byte, error) {
	if !strings.HasPrefix(id, ObjectRefPrefix) {
		return nil, fmt.Errorf("invalid CAS ID: must start with %s", ObjectRefPrefix)
	}

	hexHash := strings.TrimPrefix(id, ObjectRefPrefix)
	if len(hexHash) != 64 {
		return nil, fmt.Errorf("invalid CAS ID: hash must be 64 hex chars")
	}

	path := filepath.Join(c.root, hexHash[:2], hexHash)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found: %s", id)
		}
		return nil, fmt.Errorf("read object: %w", err)
	}

	// Check for gzip header
	if bytes.HasPrefix(raw, []byte("#cas:gz\n")) {
		r, err := gzip.NewReader(bytes.NewReader(raw[8:]))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer r.Close()
		raw, err = io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("gzip decompress: %w", err)
		}
	}

	return raw, nil
}

// Exists checks whether an object exists.
func (c *CAS) Exists(ctx context.Context, id string) bool {
	hexHash := strings.TrimPrefix(id, ObjectRefPrefix)
	if len(hexHash) != 64 {
		return false
	}
	_, err := os.Stat(filepath.Join(c.root, hexHash[:2], hexHash))
	return err == nil
}

// Delete removes an object.
func (c *CAS) Delete(ctx context.Context, id string) error {
	hexHash := strings.TrimPrefix(id, ObjectRefPrefix)
	if len(hexHash) != 64 {
		return fmt.Errorf("invalid CAS ID")
	}
	path := filepath.Join(c.root, hexHash[:2], hexHash)
	return os.Remove(path)
}

// Fingerprint returns the SHA-256 hash hex of data without storing it.
func Fingerprint(data []byte) string {
	h := sha256.Sum256(data)
	return ObjectRefPrefix + hex.EncodeToString(h[:])
}

// List returns all object IDs in the store.
func (c *CAS) List(ctx context.Context) ([]string, error) {
	var ids []string

	entries, err := os.ReadDir(c.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read CAS root: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(c.root, entry.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			ids = append(ids, ObjectRefPrefix+entry.Name()+f.Name())
		}
	}

	sort.Strings(ids)
	return ids, nil
}

// Stats returns the number of objects and total size on disk.
func (c *CAS) Stats(ctx context.Context) (count int, totalBytes int64, err error) {
	entries, err := os.ReadDir(c.root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(c.root, entry.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			count++
			totalBytes += info.Size()
		}
	}

	return count, totalBytes, nil
}

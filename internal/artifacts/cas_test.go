package artifacts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCAS_WriteRead(t *testing.T) {
	dir := t.TempDir()
	cas, err := NewCAS(filepath.Join(dir, "cas"))
	require.NoError(t, err)

	data := []byte("hello, world!")
	id, err := cas.Write(context.Background(), data)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(id, "sha256:"))
	assert.Len(t, id, 71) // "sha256:" (7) + 64 hex

	got, err := cas.Read(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestCAS_Exists(t *testing.T) {
	dir := t.TempDir()
	cas, _ := NewCAS(filepath.Join(dir, "cas"))

	assert.False(t, cas.Exists(context.Background(), "sha256:"+strings.Repeat("0", 64)))

	id, _ := cas.Write(context.Background(), []byte("test"))
	assert.True(t, cas.Exists(context.Background(), id))
}

func TestCAS_Delete(t *testing.T) {
	dir := t.TempDir()
	cas, _ := NewCAS(filepath.Join(dir, "cas"))

	id, _ := cas.Write(context.Background(), []byte("delete me"))
	assert.True(t, cas.Exists(context.Background(), id))

	err := cas.Delete(context.Background(), id)
	require.NoError(t, err)
	assert.False(t, cas.Exists(context.Background(), id))
}

func TestCAS_Dedup(t *testing.T) {
	dir := t.TempDir()
	cas, _ := NewCAS(filepath.Join(dir, "cas"))

	data := []byte("same content")
	id1, _ := cas.Write(context.Background(), data)
	id2, _ := cas.Write(context.Background(), data)
	assert.Equal(t, id1, id2) // same data = same ID
}

func TestCAS_GzipCompression(t *testing.T) {
	dir := t.TempDir()
	cas, _ := NewCAS(filepath.Join(dir, "cas"))

	// Create data larger than threshold
	data := make([]byte, GzipThreshold+1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	id, err := cas.Write(context.Background(), data)
	require.NoError(t, err)

	// Verify on-disk is compressed (smaller)
	hexHash := strings.TrimPrefix(id, ObjectRefPrefix)
	path := filepath.Join(cas.root, hexHash[:2], hexHash)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Less(t, info.Size(), int64(len(data)))

	// Read back — should be original
	got, err := cas.Read(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestCAS_List(t *testing.T) {
	dir := t.TempDir()
	cas, _ := NewCAS(filepath.Join(dir, "cas"))

	_, _ = cas.Write(context.Background(), []byte("a"))
	_, _ = cas.Write(context.Background(), []byte("b"))
	_, _ = cas.Write(context.Background(), []byte("c"))

	ids, err := cas.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, ids, 3)
}

func TestCAS_Stats(t *testing.T) {
	dir := t.TempDir()
	cas, _ := NewCAS(filepath.Join(dir, "cas"))

	count, bytes, err := cas.Stats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Equal(t, int64(0), bytes)

	_, _ = cas.Write(context.Background(), []byte("test"))
	count, bytes, err = cas.Stats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Greater(t, bytes, int64(0))
}

func TestFingerprint(t *testing.T) {
	fp := Fingerprint([]byte("deterministic"))
	assert.Equal(t, fp, Fingerprint([]byte("deterministic")))
	assert.NotEqual(t, fp, Fingerprint([]byte("different")))
	assert.True(t, strings.HasPrefix(fp, "sha256:"))
}

func TestCAS_InvalidID(t *testing.T) {
	dir := t.TempDir()
	cas, _ := NewCAS(filepath.Join(dir, "cas"))

	_, err := cas.Read(context.Background(), "invalid")
	assert.Error(t, err)

	_, err = cas.Read(context.Background(), "sha256:tooshort")
	assert.Error(t, err)

	err = cas.Delete(context.Background(), "sha256:tooshort")
	assert.Error(t, err)
}

func TestCAS_NotFound(t *testing.T) {
	dir := t.TempDir()
	cas, _ := NewCAS(filepath.Join(dir, "cas"))

	_, err := cas.Read(context.Background(), "sha256:"+strings.Repeat("0", 64))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

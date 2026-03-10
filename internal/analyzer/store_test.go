package analyzer

import (
	"context"
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileAnalysisStore_StoreAndGetHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileAnalysisStore(dir)
	require.NoError(t, err)
	ctx := context.Background()

	workItem := &contracts.WorkItem{
		ID:       "WI-1",
		Title:    "Test item",
		WorkType: contracts.WorkTypeImplementation,
		Source:   contracts.SourceMetadata{IssueKey: "PROJ-1"},
	}
	result := &contracts.AnalysisResult{
		WorkItem:       workItem,
		BrainTaskSpecs: []contracts.BrainTaskSpec{{ID: "t1", Title: "Task 1"}},
		Confidence:    0.9,
	}
	EnrichForAudit(result, workItem, "zen-brain", "1.0")

	err = store.Store(ctx, "WI-1", result)
	require.NoError(t, err)

	history, err := store.GetHistory(ctx, "WI-1")
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, 0.9, history[0].Confidence)
	assert.NotZero(t, history[0].AnalyzedAt)
	assert.Equal(t, "zen-brain", history[0].AnalyzedBy)
	assert.Equal(t, "1.0", history[0].AnalyzerVersion)
	require.NotNil(t, history[0].WorkItemSnapshot)
	assert.Equal(t, "WI-1", history[0].WorkItemSnapshot.ID)
	assert.Equal(t, "PROJ-1", history[0].WorkItemSnapshot.SourceKey)
	assert.Equal(t, "Test item", history[0].WorkItemSnapshot.Title)
}

func TestFileAnalysisStore_GetHistory_Empty(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileAnalysisStore(dir)
	require.NoError(t, err)
	ctx := context.Background()

	history, err := store.GetHistory(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, history)
}

func TestFileAnalysisStore_AppendHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileAnalysisStore(dir)
	require.NoError(t, err)
	ctx := context.Background()

	workItem := &contracts.WorkItem{ID: "WI-2", Title: "Item", Source: contracts.SourceMetadata{IssueKey: "K"}}
	r1 := &contracts.AnalysisResult{WorkItem: workItem, Confidence: 0.5}
	EnrichForAudit(r1, workItem, "a", "1")
	r2 := &contracts.AnalysisResult{WorkItem: workItem, Confidence: 0.8}
	EnrichForAudit(r2, workItem, "b", "2")

	require.NoError(t, store.Store(ctx, "WI-2", r1))
	require.NoError(t, store.Store(ctx, "WI-2", r2))

	history, err := store.GetHistory(ctx, "WI-2")
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, 0.5, history[0].Confidence)
	assert.Equal(t, 0.8, history[1].Confidence)
}

func TestSanitizeWorkItemID(t *testing.T) {
	assert.Equal(t, "a_b", sanitizeWorkItemID("a/b"))
	assert.Equal(t, "_empty", sanitizeWorkItemID(""))
	assert.NotContains(t, sanitizeWorkItemID("PROJ-123"), "/")
}

func TestNewFileAnalysisStore_RejectsEmptyDir(t *testing.T) {
	_, err := NewFileAnalysisStore("")
	require.Error(t, err)
}

func TestFileAnalysisStore_Store_NilResult(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileAnalysisStore(dir)
	require.NoError(t, err)
	err = store.Store(context.Background(), "x", nil)
	require.Error(t, err)
}

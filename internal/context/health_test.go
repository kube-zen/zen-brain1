// Package context tests health check functions.
package context

import (
	stdctx "context"
	"errors"
	"testing"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// mockZenContextForHealth is a minimal ZenContext implementation for health testing.
type mockZenContextForHealth struct {
	statsFn    func(stdctx.Context) (map[zenctx.Tier]interface{}, error)
	statsDelay time.Duration
}

func (m *mockZenContextForHealth) Stats(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
	if m.statsDelay > 0 {
		select {
		case <-time.After(m.statsDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.statsFn != nil {
		return m.statsFn(ctx)
	}
	return map[zenctx.Tier]interface{}{
		zenctx.TierHot: "available",
	}, nil
}

func (m *mockZenContextForHealth) GetSessionContext(ctx stdctx.Context, clusterID, sessionID string) (*zenctx.SessionContext, error) {
	return nil, nil
}

func (m *mockZenContextForHealth) StoreSessionContext(ctx stdctx.Context, clusterID string, session *zenctx.SessionContext) error {
	return nil
}

func (m *mockZenContextForHealth) DeleteSessionContext(ctx stdctx.Context, clusterID, sessionID string) error {
	return nil
}

func (m *mockZenContextForHealth) QueryKnowledge(ctx stdctx.Context, opts zenctx.QueryOptions) ([]zenctx.KnowledgeChunk, error) {
	return nil, nil
}

func (m *mockZenContextForHealth) StoreKnowledge(ctx stdctx.Context, chunks []zenctx.KnowledgeChunk) error {
	return nil
}

func (m *mockZenContextForHealth) ArchiveSession(ctx stdctx.Context, clusterID, sessionID string) error {
	return nil
}

func (m *mockZenContextForHealth) ReconstructSession(ctx stdctx.Context, req zenctx.ReMeRequest) (*zenctx.ReMeResponse, error) {
	return nil, nil
}

func (m *mockZenContextForHealth) Close() error {
	return nil
}

func TestCheckHot_NilContext(t *testing.T) {
	err := CheckHot(nil, nil)
	if err != nil {
		t.Errorf("CheckHot(nil, nil) = %v, want nil", err)
	}
}

func TestCheckHot_NilZenContext(t *testing.T) {
	err := CheckHot(stdctx.Background(), nil)
	if err != nil {
		t.Errorf("CheckHot(bg, nil) = %v, want nil", err)
	}
}

func TestCheckHot_Success(t *testing.T) {
	z := &mockZenContextForHealth{
		statsFn: func(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
			return map[zenctx.Tier]interface{}{
				zenctx.TierHot: "available",
			}, nil
		},
	}

	err := CheckHot(stdctx.Background(), z)
	if err != nil {
		t.Errorf("CheckHot = %v, want nil", err)
	}
}

func TestCheckHot_MissingTierHot(t *testing.T) {
	z := &mockZenContextForHealth{
		statsFn: func(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
			return map[zenctx.Tier]interface{}{}, nil
		},
	}

	err := CheckHot(stdctx.Background(), z)
	if err != nil {
		t.Errorf("CheckHot(missing TierHot) = %v, want nil (degraded)", err)
	}
}

func TestCheckHot_StatsError(t *testing.T) {
	expectedErr := errors.New("connection failed")
	z := &mockZenContextForHealth{
		statsFn: func(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
			return nil, expectedErr
		},
	}

	err := CheckHot(stdctx.Background(), z)
	if err != expectedErr {
		t.Errorf("CheckHot(error) = %v, want %v", err, expectedErr)
	}
}

func TestCheckHot_Timeout(t *testing.T) {
	z := &mockZenContextForHealth{
		statsDelay: 10 * time.Second, // Longer than healthTimeout (5s)
	}

	// The function creates its own timeout context, so we need to test that
	// it respects the internal healthTimeout
	err := CheckHot(stdctx.Background(), z)
	if err == nil {
		t.Error("CheckHot(timeout) = nil, want timeout error")
	}
}

func TestCheckWarm_NilContext(t *testing.T) {
	err := CheckWarm(nil, nil)
	if err != nil {
		t.Errorf("CheckWarm(nil, nil) = %v, want nil", err)
	}
}

func TestCheckWarm_NilZenContext(t *testing.T) {
	err := CheckWarm(stdctx.Background(), nil)
	if err != nil {
		t.Errorf("CheckWarm(bg, nil) = %v, want nil", err)
	}
}

func TestCheckWarm_Success(t *testing.T) {
	z := &mockZenContextForHealth{
		statsFn: func(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
			return map[zenctx.Tier]interface{}{
				zenctx.TierWarm: "available",
			}, nil
		},
	}

	err := CheckWarm(stdctx.Background(), z)
	if err != nil {
		t.Errorf("CheckWarm = %v, want nil", err)
	}
}

func TestCheckWarm_StatsError(t *testing.T) {
	expectedErr := errors.New("warm tier unavailable")
	z := &mockZenContextForHealth{
		statsFn: func(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
			return nil, expectedErr
		},
	}

	err := CheckWarm(stdctx.Background(), z)
	if err != expectedErr {
		t.Errorf("CheckWarm(error) = %v, want %v", err, expectedErr)
	}
}

func TestCheckCold_NilContext(t *testing.T) {
	err := CheckCold(nil, nil)
	if err != nil {
		t.Errorf("CheckCold(nil, nil) = %v, want nil", err)
	}
}

func TestCheckCold_NilZenContext(t *testing.T) {
	err := CheckCold(stdctx.Background(), nil)
	if err != nil {
		t.Errorf("CheckCold(bg, nil) = %v, want nil", err)
	}
}

func TestCheckCold_Success(t *testing.T) {
	z := &mockZenContextForHealth{
		statsFn: func(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
			return map[zenctx.Tier]interface{}{
				zenctx.TierCold: "available",
			}, nil
		},
	}

	err := CheckCold(stdctx.Background(), z)
	if err != nil {
		t.Errorf("CheckCold = %v, want nil", err)
	}
}

func TestCheckCold_StatsError(t *testing.T) {
	expectedErr := errors.New("cold tier error")
	z := &mockZenContextForHealth{
		statsFn: func(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
			return nil, expectedErr
		},
	}

	err := CheckCold(stdctx.Background(), z)
	if err != expectedErr {
		t.Errorf("CheckCold(error) = %v, want %v", err, expectedErr)
	}
}

func TestAllChecks_ProvideContext(t *testing.T) {
	z := &mockZenContextForHealth{
		statsFn: func(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
			return map[zenctx.Tier]interface{}{
				zenctx.TierHot:  "hot",
				zenctx.TierWarm: "warm",
				zenctx.TierCold: "cold",
			}, nil
		},
	}

	ctx := stdctx.Background()

	if err := CheckHot(ctx, z); err != nil {
		t.Errorf("CheckHot = %v", err)
	}
	if err := CheckWarm(ctx, z); err != nil {
		t.Errorf("CheckWarm = %v", err)
	}
	if err := CheckCold(ctx, z); err != nil {
		t.Errorf("CheckCold = %v", err)
	}
}

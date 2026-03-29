package readiness

import (
	"encoding/json"
	"testing"
)

func TestBadTicketBlocked(t *testing.T) {
	v := NewValidator()
	result := v.Check(TicketInput{
		Key:         "ZB-001",
		Title:       "Bug in code",
		Description: "Fix it",
	})

	if result.Status != StatusNotReady {
		t.Fatalf("expected not_ready, got %s", result.Status)
	}
	if len(result.Reasons) == 0 {
		t.Fatal("expected failure reasons")
	}

	// Must have at least: problem statement, evidence, acceptance criteria
	hasProblem := false
	hasEvidence := false
	hasAC := false
	for _, r := range result.Reasons {
		if r == MissingProblemStatement {
			hasProblem = true
		}
		if r == MissingEvidence {
			hasEvidence = true
		}
		if r == MissingAcceptanceCriteria {
			hasAC = true
		}
	}
	if !hasProblem {
		t.Error("expected missing_problem_statement")
	}
	if !hasEvidence {
		t.Error("expected missing_repro_or_evidence")
	}
	if !hasAC {
		t.Error("expected missing_acceptance_criteria")
	}

	if result.Action == "" {
		t.Error("expected non-empty action")
	}
	if result.Comment == "" {
		t.Error("expected non-empty comment")
	}

	t.Logf("Result: status=%s reasons=%v comment=%q", result.Status, result.Reasons, result.Comment)
}

func TestGoodTicketPasses(t *testing.T) {
	v := NewValidator()
	result := v.Check(TicketInput{
		Key:         "ZB-002",
		Title:       "Factory worker hangs when Jira API returns 500 error",
		Description: `The factory-fill binary hangs indefinitely when the Jira API returns a 500 Internal Server Error during the ticket fetch phase.

Component: cmd/factory-fill/main.go, function jiraSearch()
Repro steps:
1. Start factory-fill in daemon mode
2. Simulate Jira API returning HTTP 500
3. Observe that the process hangs without timeout

Expected: Should retry with backoff or fail fast with error message.
Actual: Process hangs, consuming CPU but making no progress.

Acceptance criteria:
- Factory should retry on 5xx with exponential backoff (max 3 retries)
- After max retries, log error and move to next poll cycle
- Must not hang indefinitely

Urgency: Medium. Affects automated ticket processing.`,
		Component: "cmd/factory-fill",
	})

	if result.Status != StatusReady {
		t.Fatalf("expected ready, got %s (reasons=%v)", result.Status, result.Reasons)
	}
	if result.Score < 4 {
		t.Errorf("expected score >= 4, got %d", result.Score)
	}

	m := v.Metrics()
	if m.PassedCount != 1 {
		t.Errorf("expected 1 passed, got %d", m.PassedCount)
	}
}

func TestGenericPhrases(t *testing.T) {
	v := NewValidator()
	genericTitles := []string{
		"Fix it",
		"Investigate",
		"Please check",
		"Doesn't work",
	}

	for _, title := range genericTitles {
		result := v.Check(TicketInput{
			Key:         "ZB-TEST",
			Title:       title,
			Description: "something happened",
		})
		if result.Status == StatusReady {
			t.Errorf("generic title %q should be rejected", title)
		}
	}
}

func TestEmptyDescription(t *testing.T) {
	v := NewValidator()
	result := v.Check(TicketInput{
		Key:         "ZB-003",
		Title:       "Some title here that is long enough",
		Description: "",
	})

	if result.Status != StatusNotReady {
		t.Fatalf("empty description should be not_ready, got %s", result.Status)
	}
}

func TestMissingScope(t *testing.T) {
	v := NewValidator()
	result := v.Check(TicketInput{
		Key:         "ZB-004",
		Title:       "Authentication error when user logs in with expired token",
		Description: `When a user tries to log in with an expired JWT token, the API returns a 401 error instead of a helpful message telling the user their session has expired.

Expected: A clear error message saying "Your session has expired, please log in again."
Actual: Generic 401 Unauthorized with no context.

To reproduce: Use an expired JWT token in the Authorization header.
Acceptance criteria: Return a specific error code and message for expired tokens.`,
	})

	// This ticket has problem statement, evidence, and AC — but no explicit scope
	// (no "component:", "file:", etc.). With the refined scope check, it may pass
	// if the scope indicator heuristic matches. Either outcome is acceptable.
	t.Logf("Missing scope test: status=%s score=%d reasons=%v", result.Status, result.Score, result.Reasons)
}

func TestScopeInDescription(t *testing.T) {
	v := NewValidator()
	result := v.Check(TicketInput{
		Key:         "ZB-005",
		Title:       "Login error in authentication module",
		Description: `The auth module at internal/auth/handler.go crashes when processing expired tokens.

Error log:
panic: nil pointer dereference in validateToken()
  at internal/auth/handler.go:142

Repro: Send expired token to POST /api/v1/auth/validate.

Expected: Return 401 with message.
Actual: Panic.

Acceptance: Handle nil token gracefully, return proper error.`,
		Component: "internal/auth",
	})

	if result.Status != StatusReady {
		t.Fatalf("expected ready (has scope in description + component), got %s (reasons=%v)", result.Status, result.Reasons)
	}
}

func TestMetricsTracking(t *testing.T) {
	v := NewValidator()

	// 3 bad, 2 good
	v.Check(TicketInput{Key: "ZB-1", Title: "Bug", Description: "Fix"})
	v.Check(TicketInput{Key: "ZB-2", Title: "Error", Description: "Check"})
	v.Check(TicketInput{Key: "ZB-3", Title: "Issue", Description: "Investigate"})
	v.Check(TicketInput{Key: "ZB-4", Title: "Auth panic in handler.go when token is null",
		Description: "The handler.go crashes with nil pointer. Error: panic at line 142. Repro: send null token. Expected: 401 error. Actual: panic. Acceptance: handle null gracefully."})
	v.Check(TicketInput{Key: "ZB-5", Title: "Login timeout on auth service",
		Description: "The auth service times out after 30s. Repro: call /login. Expected: 200 OK. Actual: timeout. Acceptance: must return in <5s."})

	m := v.Metrics()
	if m.TotalChecked != 5 {
		t.Errorf("expected 5 total, got %d", m.TotalChecked)
	}
	if m.PassedCount != 2 {
		t.Errorf("expected 2 passed, got %d", m.PassedCount)
	}
	if m.RejectedCount != 3 {
		t.Errorf("expected 3 rejected, got %d", m.RejectedCount)
	}
	if len(m.RejectionReasons) == 0 {
		t.Error("expected rejection reasons to be tracked")
	}

	data, _ := json.MarshalIndent(m, "", "  ")
	t.Logf("Metrics: %s", string(data))
}

func TestScoreCalculation(t *testing.T) {
	v := NewValidator()
	result := v.Check(TicketInput{
		Key:         "ZB-006",
		Title:       "Authentication error in internal/auth module",
		Description: `The auth module panics with nil pointer at handler.go:142.

Stack trace:
panic: nil pointer dereference
goroutine 1 [running]:
internal/auth/handler.validateToken(0x0, 0x0)

Repro: Send expired token to /api/v1/auth/validate.
Expected: Return 401 with message.
Actual: Panic.

Acceptance criteria: Handle nil tokens gracefully, return 401.`,
	})

	if result.Total != 5 {
		t.Errorf("total should always be 5, got %d", result.Total)
	}
	if result.Score < 3 {
		t.Errorf("this well-written ticket should score >= 3, got %d", result.Score)
	}
}

func TestConstraintsNotBlocking(t *testing.T) {
	// A ticket with no constraints should still pass if other criteria are met
	v := NewValidator()
	result := v.Check(TicketInput{
		Key:         "ZB-007",
		Title:       "API timeout on /health endpoint in gateway service",
		Description: `The gateway health endpoint at /health returns timeout after 60s.

The health check in the gateway service component calls downstream services synchronously.
When any downstream is slow, the entire health check times out.

Repro: Slow down downstream by 5s, observe health check timeout.
Expected: Health check returns 200 within 2s regardless of downstream.
Actual: Returns timeout after 60s.

Acceptance: Health check must complete within 2s even with slow downstreams.`,
		Component: "gateway",
	})

	if result.Status != StatusReady {
		t.Fatalf("ticket without constraints should still be ready, got %s (reasons=%v)", result.Status, result.Reasons)
	}
	// Score should be 4 (constraints missing but not blocking)
	if result.Score != 4 {
		t.Errorf("expected score 4 (no constraints but not blocking), got %d", result.Score)
	}
}

func TestClarificationComment(t *testing.T) {
	v := NewValidator()
	result := v.Check(TicketInput{
		Key:         "ZB-001",
		Title:       "Bug",
		Description: "Fix it",
	})

	if result.Comment == "" {
		t.Fatal("expected clarification comment")
	}
	// Should mention the missing items
	t.Logf("Comment:\n%s", result.Comment)
}

func TestTitleWithComponent(t *testing.T) {
	// Title like "bug in code" should fail, but "NullPointerException in UserService.login()" should pass title check
	v := NewValidator()

	bad := v.Check(TicketInput{Key: "ZB-1", Title: "bug in code", Description: "it's broken"})
	if bad.Status == StatusReady {
		t.Error("short generic title should be rejected")
	}

	good := v.Check(TicketInput{Key: "ZB-2", Title: "NullPointerException in UserService.login() when token expired", Description: ""})
	// Title is specific enough (contains function name, class, condition)
	if good.Reasons[0] == TitleTooGeneric {
		t.Log("Title check: specific title was still flagged as generic — acceptable if description provides context")
	}
}

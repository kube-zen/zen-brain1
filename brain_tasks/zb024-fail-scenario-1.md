---
apiVersion: v1alpha1
kind: BrainTask
metadata:
  name: zb024-fail-scenario-1
  labels:
    tranche: "ZB-024"
    simulation: "fail-scenario"
    task-class: validation
spec:
  description: |
    Controlled failure scenario for ZB-024 PHASE 5: Exercise retry and escalation intentionally.

    This task uses an intentionally invalid output path to verify retry behavior:
    - Invalid output path: /nonexistent/directory
    - Expected: Task should fail validation cleanly
    - Should NOT silently fall back to static templates
    - Should be retried bounded number of times
    - Should log each attempt clearly
    - Should NOT poison the queue

    This is a bounded validation task.

  template: "fail-scenario-invalid-path"

  # Override output validation to inject controlled failure
  outputValidation:
    enabled: true
    # Intentionally fail with specific error message for verification
    failIfContains: "FAIL-CLOSED: Invalid output path detected"

  # Simulate transient failure on first attempt
  failureSimulation:
    enabled: true
    failOnAttempt: 1  # Fail only on first attempt
    transient: true  # Simulate transient issue

  context:
    files: []
    instructions: |
      This task intentionally uses an invalid output path.

      Expected behavior:
      1. Validation fails with clear error
      2. Task status = "Failed" (not "Stuck")
      3. Error logged in Foreman logs
      4. Retry attempt 1-2-3
      5. After max retries, status = "Failed"
      6. Other tasks in queue continue processing
      7. No silent fallback to static templates
      8. Queue depth decreases as task moves out

      Verification:
      - Check Foreman logs for "FAIL-CLOSED: Invalid output path"
      - Check task status history for retry attempts
      - Verify queue not blocked by failed task
      - Verify no "static template" fallback occurred

  allowedOutputs: []

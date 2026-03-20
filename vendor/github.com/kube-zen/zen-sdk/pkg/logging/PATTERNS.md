# Component-Specific Logging Patterns

This document provides component-specific logging patterns and examples for common scenarios across different types of `zen` components.

## HTTP API Components (zen-back, zen-bff)

### Request Logging Pattern

```go
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    logger := logging.NewLogger("zen-back")
    start := time.Now()
    
    // Extract context values from request
    requestID := r.Header.Get("X-Request-ID")
    if requestID != "" {
        ctx = logging.WithRequestID(ctx, requestID)
    }
    
    // Process request
    result, err := h.process(ctx)
    
    duration := time.Since(start)
    statusCode := http.StatusOK
    if err != nil {
        statusCode = http.StatusInternalServerError
    }
    
    // Log with performance metrics
    perfLogger := logging.NewPerformanceLogger(logger)
    perfLogger.LogRequestProcessed(ctx, "handle_request",
        duration,
        statusCode,
        r.ContentLength,
        int64(len(result)),
        logging.HTTPMethod(r.Method),
        logging.HTTPPath(r.URL.Path),
    )
}
```

### Error Handling Pattern

```go
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    logger := logging.NewLogger("zen-back").WithContext(ctx)
    
    result, err := h.process(ctx)
    if err != nil {
        // Automatic error categorization and stack traces (in debug)
        logger.Error(err, "Request processing failed",
            logging.Operation("handle_request"),
            logging.ErrorCode("PROCESSING_FAILED"),
            logging.HTTPMethod(r.Method),
            logging.HTTPPath(r.URL.Path),
        )
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
    
    // Success logging with sampling for high-volume endpoints
    sampledLogger := logging.NewSampledLogger(logger,
        logging.DefaultSamplerConfig(),
        logging.DefaultRateLimiterConfig(),
    )
    sampledLogger.Info("Request processed successfully", true, "http_request",
        logging.HTTPStatus(http.StatusOK),
    )
}
```

## Controller Components (zen-lock, zen-flow, zen-gc)

### Reconcile Pattern

```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    logger := logging.NewLogger("zen-lock").WithContext(ctx)
    
    // Extract resource context
    ctx = logging.WithNamespace(ctx, req.Namespace)
    ctx = logging.WithName(ctx, req.Name)
    ctx = logging.WithKind(ctx, "ZenLock")
    
    logger.Info("Reconciling resource",
        logging.Operation("reconcile"),
        logging.Namespace(req.Namespace),
        logging.Name(req.Name),
    )
    
    // Reconcile logic
    resource, err := r.GetResource(ctx, req.Namespace, req.Name)
    if err != nil {
        logger.Error(err, "Failed to get resource",
            logging.Operation("get_resource"),
            logging.ErrorCode("RESOURCE_NOT_FOUND"),
        )
        return ctrl.Result{}, err
    }
    
    // Process
    err = r.process(ctx, resource)
    if err != nil {
        logger.Error(err, "Failed to reconcile resource",
            logging.Operation("reconcile"),
            logging.ErrorCode("RECONCILE_FAILED"),
        )
        return ctrl.Result{Requeue: true}, err
    }
    
    logger.Info("Resource reconciled successfully",
        logging.Operation("reconcile"),
        logging.ResourceID(string(resource.UID)),
    )
    
    return ctrl.Result{}, nil
}
```

### Watch/Event Pattern

```go
func (r *Reconciler) handleEvent(ctx context.Context, event watch.Event) {
    logger := logging.NewLogger("zen-lock").WithContext(ctx)
    
    logger.Info("Processing watch event",
        logging.Operation("watch_event"),
        logging.String("event_type", string(event.Type)),
        logging.Kind(event.Object.GetObjectKind().GroupVersionKind().Kind),
        logging.Namespace(event.Object.GetNamespace()),
        logging.Name(event.Object.GetName()),
    )
    
    // Process event
    err := r.processEvent(ctx, event)
    if err != nil {
        logger.Error(err, "Failed to process watch event",
            logging.Operation("process_event"),
            logging.ErrorCode("EVENT_PROCESSING_FAILED"),
        )
    }
}
```

## Cluster Components

### Event Processing Pattern

```go
func (p *Processor) ProcessEvent(ctx context.Context, event *Event) error {
    logger := logging.NewLogger("component-name").WithContext(ctx)
    
    // Extract cluster context
    ctx = logging.WithClusterID(ctx, event.ClusterID)
    // Note: For platform-specific adapter/instance ID, use github.com/kube-zen/shared/logctx
    
    logger.Info("Processing event",
        logging.Operation("process_event"),
        logging.ResourceType("event"),
        logging.ResourceID(event.ID),
    )
    
    // Measure processing duration
    perfLogger := logging.NewPerformanceLogger(logger)
    err := perfLogger.MeasureDuration(ctx, "process_event", func() error {
        return p.doProcess(ctx, event)
    })
    
    if err != nil {
        logger.Error(err, "Event processing failed",
            logging.Operation("process_event"),
            logging.ErrorCode("PROCESSING_FAILED"),
            logging.ResourceID(event.ID),
        )
    }
    
    return err
}
```

### Batch Processing Pattern

```go
func (p *Processor) ProcessBatch(ctx context.Context, events []*Event) error {
    logger := logging.NewLogger("component-name").WithContext(ctx)
    
    logger.Info("Processing batch",
        logging.Operation("process_batch"),
        logging.Int("batch_size", len(events)),
    )
    
    start := time.Now()
    processed := 0
    failed := 0
    
    for _, event := range events {
        err := p.ProcessEvent(ctx, event)
        if err != nil {
            failed++
        } else {
            processed++
        }
    }
    
    duration := time.Since(start)
    perfLogger := logging.NewPerformanceLogger(logger)
    perfLogger.LogMessageQueueOperation(ctx, "process_batch", "event-queue",
        len(events),
        duration,
        nil,
        logging.Int("processed", processed),
        logging.Int("failed", failed),
    )
    
    return nil
}
```

## Worker Components (zen-back-workers)

### Job Processing Pattern

```go
func (w *Worker) ProcessJob(ctx context.Context, job *Job) error {
    logger := logging.NewLogger("zen-back-workers").WithContext(ctx)
    
    logger.Info("Starting job",
        logging.Operation("process_job"),
        logging.ResourceID(job.ID),
        logging.String("job_type", job.Type),
    )
    
    // Measure duration
    perfLogger := logging.NewPerformanceLogger(logger)
    err := perfLogger.MeasureDuration(ctx, "process_job", func() error {
        return w.executeJob(ctx, job)
    })
    
    if err != nil {
        logger.Error(err, "Job processing failed",
            logging.Operation("process_job"),
            logging.ErrorCode("JOB_FAILED"),
            logging.ResourceID(job.ID),
            logging.Bool("retryable", logging.IsRetryableError(err)),
        )
    } else {
        logger.Info("Job completed successfully",
            logging.Operation("process_job"),
            logging.ResourceID(job.ID),
        )
    }
    
    return err
}
```

### Retry Pattern

```go
func (w *Worker) ProcessWithRetry(ctx context.Context, job *Job, maxRetries int) error {
    logger := logging.NewLogger("zen-back-workers").WithContext(ctx)
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        err := w.ProcessJob(ctx, job)
        if err == nil {
            return nil
        }
        
        if !logging.IsRetryableError(err) {
            logger.Error(err, "Non-retryable error, stopping",
                logging.Operation("process_job"),
                logging.ErrorCode("NON_RETRYABLE_ERROR"),
                logging.RetryCount(attempt),
            )
            return err
        }
        
        logger.Warn("Retryable error, retrying",
            logging.Operation("process_job_retry"),
            logging.RetryCount(attempt),
            logging.ErrorCode(logging.ExtractErrorCode(err)),
        )
        
        time.Sleep(time.Duration(attempt+1) * time.Second)
    }
    
    return fmt.Errorf("max retries exceeded")
}
```

## Database Operations Pattern

```go
func (s *Service) QueryUsers(ctx context.Context, tenantID string) ([]User, error) {
    logger := logging.NewLogger("zen-back").WithContext(ctx)
    perfLogger := logging.NewPerformanceLogger(logger)
    
    query := "SELECT * FROM users WHERE tenant_id = $1"
    start := time.Now()
    
    rows, err := s.db.QueryContext(ctx, query, tenantID)
    if err != nil {
        perfLogger.LogDBCall(ctx, "select_users", query,
            time.Since(start),
            0,
            err,
            logging.TenantID(tenantID, true),
        )
        return nil, err
    }
    defer rows.Close()
    
    var users []User
    rowCount := 0
    for rows.Next() {
        // Scan row
        rowCount++
    }
    
    duration := time.Since(start)
    perfLogger.LogDBCall(ctx, "select_users", query,
        duration,
        int64(rowCount),
        nil,
        logging.TenantID(tenantID, true),
        logging.Int("rows_returned", rowCount),
    )
    
    return users, nil
}
```

## Cache Operations Pattern

```go
func (s *Service) GetUser(ctx context.Context, userID string) (*User, error) {
    logger := logging.NewLogger("zen-back").WithContext(ctx)
    perfLogger := logging.NewPerformanceLogger(logger)
    
    cacheKey := fmt.Sprintf("user:%s", userID)
    start := time.Now()
    
    // Try cache first
    cached, err := s.cache.Get(ctx, cacheKey)
    hit := err == nil && cached != nil
    cacheDuration := time.Since(start)
    
    perfLogger.LogCacheOperation(ctx, "get", cacheKey, hit, cacheDuration, err,
        logging.UserID(userID, true),
    )
    
    if hit {
        return cached.(*User), nil
    }
    
    // Cache miss - fetch from DB
    user, err := s.db.GetUser(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // Store in cache (async, don't block)
    go func() {
        setStart := time.Now()
        setErr := s.cache.Set(ctx, cacheKey, user, time.Hour)
        setDuration := time.Since(setStart)
        
        perfLogger.LogCacheOperation(ctx, "set", cacheKey, false, setDuration, setErr,
            logging.UserID(userID, true),
        )
    }()
    
    return user, nil
}
```

## Authentication/Authorization Pattern

```go
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    logger := logging.NewLogger("zen-auth").WithContext(ctx)
    auditLogger := logging.NewAuditLogger(logger)
    
    // Extract credentials
    username := r.FormValue("username")
    ipAddress := r.RemoteAddr
    userAgent := r.UserAgent()
    
    // Authenticate
    user, err := h.authService.Authenticate(ctx, username, password)
    
    if err != nil {
        auditLogger.LogLogin(ctx, logging.AuditResultFailure, ipAddress, userAgent,
            logging.String("username", username), // Note: In production, may want to mask
            logging.ErrorCode("AUTH_FAILED"),
        )
        http.Error(w, "Authentication failed", http.StatusUnauthorized)
        return
    }
    
    // Success
    ctx = logging.WithUserID(ctx, user.ID)
    auditLogger.LogLogin(ctx, logging.AuditResultSuccess, ipAddress, userAgent,
        logging.UserID(user.ID, true),
    )
    
    // Set session, etc.
}
```

## Configuration Change Pattern

```go
func (s *Service) UpdateConfig(ctx context.Context, key string, newValue string) error {
    logger := logging.NewLogger("zen-back").WithContext(ctx)
    auditLogger := logging.NewAuditLogger(logger)
    
    // Get old value
    oldValue, err := s.config.Get(key)
    if err != nil {
        return err
    }
    
    // Update
    err = s.config.Set(key, newValue)
    if err != nil {
        logger.Error(err, "Failed to update configuration",
            logging.Operation("update_config"),
            logging.ErrorCode("CONFIG_UPDATE_FAILED"),
            logging.ConfigKeyField(key),
        )
        return err
    }
    
    // Audit log
    auditLogger.LogConfigChange(ctx, key, oldValue, newValue,
        logging.Operation("update_config"),
    )
    
    return nil
}
```

## Best Practices Summary

1. **Always use context** - `logger.WithContext(ctx)` for all logs
2. **Use structured fields** - Never format strings manually
3. **Use PerformanceLogger** - For all timing/performance metrics
4. **Use AuditLogger** - For all security-sensitive operations
5. **Use sampled logging** - For high-volume success logs
6. **Extract context early** - Set request_id, tenant_id, etc. at request start
7. **Log errors with context** - Include operation, error_code, resource info
8. **Measure durations** - Use PerformanceLogger.MeasureDuration for timing
9. **Mask sensitive data** - Always use masking for UUIDs, tokens, IPs
10. **Use appropriate log levels** - Debug for dev, Info/Warn/Error for production


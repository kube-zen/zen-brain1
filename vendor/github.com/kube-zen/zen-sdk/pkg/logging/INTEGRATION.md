# Logging Integration Guide

This document describes how to integrate `zen-sdk/pkg/logging` with monitoring systems, log aggregation tools, and other observability infrastructure.

## Log Aggregation

### Fluentd / Fluent Bit

The logger outputs structured JSON in production mode, which is compatible with Fluentd/Fluent Bit.

**Fluentd Configuration Example:**

```ruby
<source>
  @type tail
  path /var/log/zen/*.log
  pos_file /var/log/fluentd/zen.log.pos
  tag zen.logs
  format json
</source>

<match zen.logs>
  @type elasticsearch
  host elasticsearch.zen-system.svc.cluster.local
  port 9200
  index_name zen-logs
  type_name _doc
</match>
```

### Loki

Loki works seamlessly with JSON logs:

```yaml
# Promtail configuration
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki.zen-system.svc.cluster.local:3100/loki/api/v1/push

scrape_configs:
  - job_name: zen-logs
    static_configs:
      - targets:
          - localhost
        labels:
          job: zen-logs
          __path__: /var/log/zen/*.log
```

### ELK Stack (Elasticsearch, Logstash, Kibana)

**Logstash Configuration:**

```ruby
input {
  file {
    path => "/var/log/zen/*.log"
    codec => json
  }
}

filter {
  # Parse structured fields
  if [component] {
    mutate { add_field => { "service" => "%{component}" } }
  }
  
  # Extract trace context
  if [trace_id] {
    mutate { add_field => { "trace.id" => "%{trace_id}" } }
  }
  
  # Categorize errors
  if [error_category] {
    mutate { add_field => { "error.category" => "%{error_category}" } }
  }
}

output {
  elasticsearch {
    hosts => ["elasticsearch:9200"]
    index => "zen-logs-%{+YYYY.MM.dd}"
  }
}
```

## Monitoring Integration

### Prometheus Metrics from Logs

Use log-based metrics extraction (via Promtail, Fluentd, or similar):

**Example: Error Rate Metric**

```yaml
# Promtail pipeline stage
- match:
    selector: '{job="zen-logs"}'
    stages:
      - json:
          expressions:
            level: level
            component: component
            error_category: error_category
      - metrics:
          error_rate:
            type: Counter
            description: "Error rate by component and category"
            source: error_category
            config:
              action: inc
              match_all: true
              match: level=~"ERROR|error"
              labels:
                component: component
                category: error_category
```

### Grafana Dashboards

Create dashboards using log-based metrics:

1. **Error Rate by Component**
   - Query: `sum(rate(error_rate[5m])) by (component)`

2. **Request Latency P99**
   - Query: `histogram_quantile(0.99, sum(rate(latency_ms_bucket[5m])) by (le, component))`

3. **Error Categories**
   - Query: `sum(error_rate) by (category)`

## Correlation with Traces

### OpenTelemetry Integration

The logger automatically extracts trace and span IDs from OpenTelemetry context. To correlate logs with traces:

1. Ensure OpenTelemetry is initialized:
   ```go
   import "github.com/kube-zen/zen-sdk/pkg/observability"
   observability.InitWithDefaults(ctx, "my-service")
   ```

2. Logs automatically include `trace_id` and `span_id` fields

3. In your observability platform (Jaeger, Tempo, etc.), search traces by `trace_id` from logs

### Distributed Tracing Best Practices

```go
// Start span
tracer := observability.GetTracer("my-component")
ctx, span := tracer.Start(ctx, "my-operation")
defer span.End()

// Log within span context - automatically includes trace_id and span_id
logger.WithContext(ctx).Info("Operation started", logging.Operation("my-operation"))
```

## Alerting

### Log-Based Alerts

Create alerts based on log patterns:

**Example: High Error Rate Alert**

```yaml
groups:
  - name: zen_logs
    interval: 30s
    rules:
      - alert: HighErrorRate
        expr: |
          sum(rate(error_rate[5m])) by (component) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate in {{ $labels.component }}"
          description: "Component {{ $labels.component }} has error rate of {{ $value }} errors/sec"
```

## Performance Monitoring

### Request Latency Tracking

Use the PerformanceLogger for consistent latency logging:

```go
perfLogger.LogRequestProcessed(ctx, "user_list", duration, 200, reqSize, respSize)
```

This creates logs with `latency_ms` field that can be aggregated:

```promql
# P95 latency by component
histogram_quantile(0.95, 
  sum(rate(latency_ms_bucket[5m])) by (le, component)
)
```

### Database Query Performance

```go
perfLogger.LogDBCall(ctx, "select_users", query, duration, rowsAffected, err)
```

Query logs include `db_duration_ms` and `rows_affected` for performance analysis.

## Security and Compliance

### Audit Log Forwarding

Audit logs should be forwarded to a secure, immutable log store:

```yaml
# Separate audit log stream
<match zen.audit>
  @type secure_forward
  secure true
  self_hostname audit-forwarder
  shared_key "SECRET_KEY"
  <server>
    host audit-store.zen-system.svc.cluster.local
    port 24284
  </server>
</match>
```

### Compliance Log Retention

Ensure audit logs meet retention requirements:

- **SOC 2**: 90 days minimum
- **ISO 27001**: 3 years recommended
- **PCI-DSS**: 1 year minimum

Configure log retention in your log aggregation system accordingly.

## Troubleshooting

### Debug Mode

Enable debug logging for troubleshooting:

```bash
export LOG_LEVEL=debug
export DEVELOPMENT=true
```

This enables:
- Stack traces for all errors
- Debug-level logs
- Pretty console output

### Log Sampling Issues

If logs are being sampled out, adjust sampling configuration:

```go
samplerConfig := logging.SamplerConfig{
    InfoSamplingRate:    1.0,  // 100% (disable sampling)
    SuccessSamplingRate: 0.1,  // 10% for success logs
    ErrorSamplingRate:   1.0,  // 100% (always log errors)
}
```

### Missing Context

If context values are missing from logs:

1. Ensure context is propagated through call chains
2. Use `logging.WithRequestID(ctx, id)` to add values
3. Check that middleware is setting context values correctly

## Best Practices

1. **Use structured fields** - Don't format strings manually
2. **Always include context** - Use `WithContext(ctx)` for all logs
3. **Use appropriate log levels** - Debug for development, Info/Warn/Error for production
4. **Sample high-volume logs** - Use SampledLogger for HTTP access logs
5. **Audit security events** - Use AuditLogger for all security-sensitive operations
6. **Mask sensitive data** - Always mask UUIDs, tokens, IPs in logs
7. **Correlate with traces** - Use OpenTelemetry for distributed tracing


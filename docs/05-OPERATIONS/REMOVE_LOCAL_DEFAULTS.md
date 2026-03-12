# Remove Dev-Local Defaults from Production Commands

## Issue

Production-facing commands and runtime helpers have hardcoded local defaults that should fail closed:
- `localhost:6379` for Redis
- `localhost:9000` for S3
- `minioadmin` for S3 credentials
- `/tmp/zen-brain-factory` for Factory workspaces

## Current State

### cmd/zen-brain/main.go

#### Line 367 (vertical-slice message bus setup):
```go
redisURL := os.Getenv("REDIS_URL")
if redisURL == "" {
    redisURL = "redis://localhost:6379"  // DEFAULT - SHOULD FAIL CLOSED
}
```

#### Lines 1166-1198 (createRealZenContext):
```go
redisAddr := os.Getenv("REDIS_URL")
if redisAddr == "" {
    redisAddr = "localhost:6379"  // DEFAULT - SHOULD FAIL CLOSED
}
// ...
s3Endpoint := os.Getenv("S3_ENDPOINT")
if s3Endpoint == "" {
    s3Endpoint = "http://localhost:9000"  // DEFAULT - SHOULD FAIL CLOSED
}
s3AccessKey := os.Getenv("S3_ACCESS_KEY_ID")
if s3AccessKey == "" {
    s3AccessKey = "minioadmin"  // DEFAULT - SHOULD FAIL CLOSED
}
s3SecretKey := os.Getenv("S3_SECRET_ACCESS_KEY")
if s3SecretKey == "" {
    s3SecretKey = "minioadmin"  // DEFAULT - SHOULD FAIL CLOSED
}
```

### cmd/foreman/main.go

#### Lines 54-55 (Factory defaults):
```go
flag.StringVar(&runtimeDir, "factory-runtime-dir", envStr("ZEN_FOREMAN_RUNTIME_DIR", "/tmp/zen-brain-factory"), "Runtime dir...")
flag.StringVar(&workspaceHome, "factory-workspace-home", envStr("ZEN_FOREMAN_WORKSPACE_HOME", "/tmp/zen-brain-factory"), "Workspace home...")
```

### internal/context/tier1/redis_client.go

#### Default in RedisConfig struct:
```go
Addr: "localhost:6379",  // DEFAULT - SHOULD FAIL CLOSED
```

### internal/messagebus/redis/redis.go

#### Default in RedisConfig struct:
```go
RedisURL: "redis://localhost:6379",  // DEFAULT - SHOULD FAIL CLOSED
```

### internal/foreman/factory_runner.go

#### Default in FactoryTaskRunner struct:
```go
cfg.RuntimeDir = "/tmp/zen-brain-factory"  // DEFAULT - SHOULD FAIL CLOSED
```

## Proposed Fix

All local defaults should be removed and replaced with fail-closed behavior:
1. Check if environment variable is set
2. If not set → return error or skip initialization
3. Never use hardcoded local defaults

### Fix Pattern

```go
// BEFORE (has default):
redisURL := os.Getenv("REDIS_URL")
if redisURL == "" {
    redisURL = "redis://localhost:6379"  // DEFAULT
}

// AFTER (fails closed):
redisURL := os.Getenv("REDIS_URL")
if redisURL == "" {
    return fmt.Errorf("REDIS_URL not set (cannot use default localhost:6379)")
}
```

## Files to Fix

1. `cmd/zen-brain/main.go`:
   - Line 367: Remove `redis://localhost:6379` default
   - Lines 1174-1198: Remove all local defaults in `createRealZenContext()`

2. `cmd/foreman/main.go`:
   - Lines 54-55: Remove `/tmp/zen-brain-factory` defaults

3. `internal/context/tier1/redis_client.go`:
   - Remove default `Addr` value

4. `internal/messagebus/redis/redis.go`:
   - Remove default `RedisURL` value

5. `internal/foreman/factory_runner.go`:
   - Remove default `RuntimeDir` assignment

## Impact

After fixing:
- Production commands will fail fast when required config is missing
- No silent fallback to localhost/defaults
- Operators must explicitly set all required environment variables
- Better production deployment safety

## Testing

For local dev, operators can:
- Set environment variables explicitly
- Use a dev config/profile with local values
- Use the `--mock` flag where available

## References

- Block 3 strict mode enforcement (commit ce82d34)
- Block 2 office bootstrap (commit 54ad9f8)
- COMPLETENESS_MATRIX.md

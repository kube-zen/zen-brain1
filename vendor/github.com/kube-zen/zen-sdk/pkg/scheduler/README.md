# Scheduler

Cron-like job scheduling with persistence support. Designed for reuse across zen components.

## Features

- **Cron expressions**: Standard 6-field cron syntax (with seconds)
- **Intervals**: `@every 5m`, `@hourly`, `@daily`, etc.
- **One-time jobs**: Schedule a job to run at a specific time
- **Persistence**: Memory store (default) or file-based store
- **Callbacks**: Hook into job start/end events
- **Stats**: Track run counts, error counts, last run times
- **Thread-safe**: All operations are safe for concurrent use

## Usage

```go
import "github.com/kube-zen/zen-sdk/pkg/scheduler"

// Create scheduler
sched := scheduler.New(&scheduler.Config{
    Store:  scheduler.NewMemoryStore(), // or NewFileStore("/path/to/jobs.json")
    Logger: log.Default(),
})

// Register handlers
sched.RegisterHandler("my-task", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
    // Do work
    return "done", nil
})

// Add jobs
sched.AddJob(&scheduler.Job{
    ID:       "job-1",
    Name:     "My Task",
    Schedule: "0 */5 * * * *", // Every 5 minutes
    Handler:  "my-task",
    Args:     map[string]interface{}{"key": "value"},
    Enabled:  true,
})

// Start scheduler
sched.Start()
defer sched.Stop()
```

## Schedule Syntax

### Cron expressions (6 fields)
```
┌──────────── second (0-59)
│ ┌────────── minute (0-59)
│ │ ┌──────── hour (0-23)
│ │ │ ┌────── day of month (1-31)
│ │ │ │ ┌──── month (1-12)
│ │ │ │ │ ┌── day of week (0-6, Sunday=0)
│ │ │ │ │ │
* * * * * *
```

### Predefined schedules
- `@yearly` / `@annually` - Run once a year at midnight of Jan 1
- `@monthly` - Run once a month at midnight of the first day
- `@weekly` - Run once a week at midnight on Sunday
- `@daily` / `@midnight` - Run once a day at midnight
- `@hourly` - Run once an hour at the beginning of the hour
- `@every <duration>` - Run at fixed intervals (e.g., `@every 5m`, `@every 1h30m`)

### One-time jobs
Set `RunAt` instead of `Schedule`:
```go
runAt := time.Now().Add(10 * time.Minute)
sched.AddJob(&scheduler.Job{
    ID:      "reminder",
    Name:    "One-time reminder",
    RunAt:   &runAt,
    Handler: "notify",
})
```

## Persistence

### Memory Store (default)
Jobs are lost on restart.

### File Store
Jobs persist to a JSON file:
```go
store, err := scheduler.NewFileStore("/var/lib/myapp/jobs.json")
sched := scheduler.New(&scheduler.Config{Store: store})
```

## API

### Scheduler methods
- `RegisterHandler(name, fn)` - Register a job handler
- `AddJob(job)` - Add a new job
- `RemoveJob(id)` - Remove a job
- `EnableJob(id)` - Enable a disabled job
- `DisableJob(id)` - Disable a job (keeps it but stops scheduling)
- `GetJob(id)` - Get a job by ID
- `ListJobs()` - List all jobs
- `RunNow(id)` - Run a job immediately (outside schedule)
- `Start()` - Start the scheduler
- `Stop()` - Stop the scheduler
- `Stats()` - Get scheduler statistics

### Callbacks
```go
sched := scheduler.New(&scheduler.Config{
    OnJobStart: func(job *scheduler.Job) {
        log.Printf("Job %s starting", job.ID)
    },
    OnJobEnd: func(job *scheduler.Job, result *scheduler.JobResult) {
        if result.Success {
            log.Printf("Job %s completed in %v", job.ID, result.Duration)
        } else {
            log.Printf("Job %s failed: %s", job.ID, result.Error)
        }
    },
})
```

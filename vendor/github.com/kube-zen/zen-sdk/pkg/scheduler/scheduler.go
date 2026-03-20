// Package scheduler provides cron-like job scheduling with persistence support.
// Designed for reuse across zen components (zen-brain, zen-claw, etc).
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Job represents a scheduled job
type Job struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schedule    string                 `json:"schedule"`         // Cron expression, "@every 5m", or "@once"
	RunAt       *time.Time             `json:"run_at,omitempty"` // For one-time jobs
	Handler     string                 `json:"handler"`          // Handler name (resolved at runtime)
	Args        map[string]interface{} `json:"args,omitempty"`   // Arguments passed to handler
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	LastRun     *time.Time             `json:"last_run,omitempty"`
	NextRun     *time.Time             `json:"next_run,omitempty"`
	RunCount    int64                  `json:"run_count"`
	ErrorCount  int64                  `json:"error_count"`
	LastError   string                 `json:"last_error,omitempty"`
}

// JobResult represents the result of a job execution
type JobResult struct {
	JobID     string        `json:"job_id"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration_ms"`
	Output    interface{}   `json:"output,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// Handler is a function that executes a job
type Handler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// Store interface for job persistence
type Store interface {
	Save(job *Job) error
	Get(id string) (*Job, error)
	List() ([]*Job, error)
	Delete(id string) error
	UpdateLastRun(id string, lastRun time.Time, nextRun *time.Time, err error) error
}

// Scheduler manages scheduled jobs
type Scheduler struct {
	mu       sync.RWMutex
	cron     *cron.Cron
	jobs     map[string]*Job
	handlers map[string]Handler
	entries  map[string]cron.EntryID // job ID -> cron entry ID
	store    Store
	logger   *log.Logger
	running  bool

	// Callbacks
	onJobStart func(job *Job)
	onJobEnd   func(job *Job, result *JobResult)
}

// Config holds scheduler configuration
type Config struct {
	Store      Store
	Logger     *log.Logger
	Location   *time.Location // Timezone for cron expressions
	OnJobStart func(job *Job)
	OnJobEnd   func(job *Job, result *JobResult)
}

// New creates a new scheduler
func New(cfg *Config) *Scheduler {
	if cfg == nil {
		cfg = &Config{}
	}

	location := cfg.Location
	if location == nil {
		location = time.Local
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	s := &Scheduler{
		cron:       cron.New(cron.WithLocation(location), cron.WithSeconds()),
		jobs:       make(map[string]*Job),
		handlers:   make(map[string]Handler),
		entries:    make(map[string]cron.EntryID),
		store:      cfg.Store,
		logger:     logger,
		onJobStart: cfg.OnJobStart,
		onJobEnd:   cfg.OnJobEnd,
	}

	return s
}

// RegisterHandler registers a handler for jobs
func (s *Scheduler) RegisterHandler(name string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[name] = handler
	s.logger.Printf("[scheduler] registered handler: %s", name)
}

// AddJob adds a new job
func (s *Scheduler) AddJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		return fmt.Errorf("job ID is required")
	}
	if job.Handler == "" {
		return fmt.Errorf("job handler is required")
	}
	if job.Schedule == "" && job.RunAt == nil {
		return fmt.Errorf("job schedule or run_at is required")
	}

	job.CreatedAt = time.Now()
	job.UpdatedAt = job.CreatedAt
	if job.Enabled {
		job.Enabled = true
	}

	// Store job
	s.jobs[job.ID] = job

	// Persist if store available
	if s.store != nil {
		if err := s.store.Save(job); err != nil {
			s.logger.Printf("[scheduler] failed to persist job %s: %v", job.ID, err)
		}
	}

	// Schedule if running
	if s.running && job.Enabled {
		if err := s.scheduleJob(job); err != nil {
			return fmt.Errorf("failed to schedule job: %w", err)
		}
	}

	s.logger.Printf("[scheduler] added job: %s (%s)", job.ID, job.Name)
	return nil
}

// RemoveJob removes a job
func (s *Scheduler) RemoveJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entries[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, id)
	}

	delete(s.jobs, id)

	if s.store != nil {
		if err := s.store.Delete(id); err != nil {
			s.logger.Printf("[scheduler] failed to delete job %s from store: %v", id, err)
		}
	}

	s.logger.Printf("[scheduler] removed job: %s", id)
	return nil
}

// EnableJob enables a job
func (s *Scheduler) EnableJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Enabled = true
	job.UpdatedAt = time.Now()

	if s.running {
		if err := s.scheduleJob(job); err != nil {
			return err
		}
	}

	if s.store != nil {
		s.store.Save(job)
	}

	return nil
}

// DisableJob disables a job
func (s *Scheduler) DisableJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Enabled = false
	job.UpdatedAt = time.Now()

	if entryID, ok := s.entries[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, id)
	}

	if s.store != nil {
		s.store.Save(job)
	}

	return nil
}

// GetJob returns a job by ID
func (s *Scheduler) GetJob(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	// Return copy
	copy := *job
	return &copy, true
}

// ListJobs returns all jobs
func (s *Scheduler) ListJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		copy := *job
		jobs = append(jobs, &copy)
	}
	return jobs
}

// Start starts the scheduler
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// Load jobs from store
	if s.store != nil {
		jobs, err := s.store.List()
		if err != nil {
			s.logger.Printf("[scheduler] failed to load jobs from store: %v", err)
		} else {
			for _, job := range jobs {
				s.jobs[job.ID] = job
			}
			s.logger.Printf("[scheduler] loaded %d jobs from store", len(jobs))
		}
	}

	// Schedule all enabled jobs
	for _, job := range s.jobs {
		if job.Enabled {
			if err := s.scheduleJob(job); err != nil {
				s.logger.Printf("[scheduler] failed to schedule job %s: %v", job.ID, err)
			}
		}
	}

	s.cron.Start()
	s.running = true
	s.logger.Printf("[scheduler] started with %d jobs", len(s.jobs))
	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()
	s.running = false
	s.logger.Printf("[scheduler] stopped")
}

// scheduleJob adds a job to the cron scheduler (must hold lock)
func (s *Scheduler) scheduleJob(job *Job) error {
	// Remove existing entry if any
	if entryID, ok := s.entries[job.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, job.ID)
	}

	// Handle one-time jobs
	if job.RunAt != nil {
		return s.scheduleOnce(job)
	}

	// Parse schedule
	schedule := job.Schedule

	// Handle @every syntax
	if len(schedule) > 0 && schedule[0] != '@' && schedule[0] != '*' && schedule[0] != '0' {
		// Might be missing seconds field, add "0 " prefix
		schedule = "0 " + schedule
	}

	entryID, err := s.cron.AddFunc(schedule, func() {
		s.runJob(job)
	})
	if err != nil {
		return fmt.Errorf("invalid schedule %q: %w", job.Schedule, err)
	}

	s.entries[job.ID] = entryID

	// Update next run time
	entry := s.cron.Entry(entryID)
	nextRun := entry.Next
	job.NextRun = &nextRun

	return nil
}

// scheduleOnce schedules a one-time job
func (s *Scheduler) scheduleOnce(job *Job) error {
	if job.RunAt == nil {
		return fmt.Errorf("run_at is required for one-time jobs")
	}

	delay := time.Until(*job.RunAt)
	if delay < 0 {
		// Already past, run immediately
		delay = 0
	}

	// Use time.AfterFunc for one-time execution
	time.AfterFunc(delay, func() {
		s.runJob(job)
		// Remove job after execution
		s.RemoveJob(job.ID)
	})

	s.logger.Printf("[scheduler] scheduled one-time job %s to run at %v", job.ID, job.RunAt)
	return nil
}

// runJob executes a job
func (s *Scheduler) runJob(job *Job) {
	s.mu.RLock()
	handler, ok := s.handlers[job.Handler]
	s.mu.RUnlock()

	if !ok {
		s.logger.Printf("[scheduler] handler not found for job %s: %s", job.ID, job.Handler)
		return
	}

	// Callback: job starting
	if s.onJobStart != nil {
		s.onJobStart(job)
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result := &JobResult{
		JobID:     job.ID,
		Timestamp: start,
	}

	// Execute handler
	output, err := handler(ctx, job.Args)
	result.Duration = time.Since(start)
	result.Output = output

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		s.logger.Printf("[scheduler] job %s failed: %v", job.ID, err)
	} else {
		result.Success = true
		s.logger.Printf("[scheduler] job %s completed in %v", job.ID, result.Duration)
	}

	// Update job stats
	s.mu.Lock()
	now := time.Now()
	job.LastRun = &now
	job.RunCount++
	if err != nil {
		job.ErrorCount++
		job.LastError = err.Error()
	} else {
		job.LastError = ""
	}

	// Update next run from cron entry
	if entryID, ok := s.entries[job.ID]; ok {
		entry := s.cron.Entry(entryID)
		nextRun := entry.Next
		job.NextRun = &nextRun
	}
	s.mu.Unlock()

	// Persist
	if s.store != nil {
		s.store.UpdateLastRun(job.ID, now, job.NextRun, err)
	}

	// Callback: job ended
	if s.onJobEnd != nil {
		s.onJobEnd(job, result)
	}
}

// RunNow runs a job immediately (outside of schedule)
func (s *Scheduler) RunNow(id string) error {
	s.mu.RLock()
	job, ok := s.jobs[id]
	enabled := ok && job.Enabled
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	if !enabled {
		return fmt.Errorf("job %s is disabled", id)
	}

	go s.runJob(job)
	return nil
}

// Stats returns scheduler statistics
func (s *Scheduler) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalRuns, totalErrors int64
	for _, job := range s.jobs {
		totalRuns += job.RunCount
		totalErrors += job.ErrorCount
	}

	return map[string]interface{}{
		"running":      s.running,
		"total_jobs":   len(s.jobs),
		"enabled_jobs": len(s.entries),
		"handlers":     len(s.handlers),
		"total_runs":   totalRuns,
		"total_errors": totalErrors,
	}
}

// MarshalJSON implements json.Marshaler for Job
func (j *Job) MarshalJSON() ([]byte, error) {
	type Alias Job
	return json.Marshal(&struct {
		*Alias
		LastRun *string `json:"last_run,omitempty"`
		NextRun *string `json:"next_run,omitempty"`
		RunAt   *string `json:"run_at,omitempty"`
	}{
		Alias:   (*Alias)(j),
		LastRun: timeToString(j.LastRun),
		NextRun: timeToString(j.NextRun),
		RunAt:   timeToString(j.RunAt),
	})
}

func timeToString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

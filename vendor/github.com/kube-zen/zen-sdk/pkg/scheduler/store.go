package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemoryStore is an in-memory store (no persistence)
type MemoryStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs: make(map[string]*Job),
	}
}

func (s *MemoryStore) Save(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *job
	s.jobs[job.ID] = &copy
	return nil
}

func (s *MemoryStore) Get(id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	copy := *job
	return &copy, nil
}

func (s *MemoryStore) List() ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		copy := *job
		jobs = append(jobs, &copy)
	}
	return jobs, nil
}

func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	return nil
}

func (s *MemoryStore) UpdateLastRun(id string, lastRun time.Time, nextRun *time.Time, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	job.LastRun = &lastRun
	job.NextRun = nextRun
	job.RunCount++
	if err != nil {
		job.ErrorCount++
		job.LastError = err.Error()
	}
	return nil
}

// FileStore persists jobs to a JSON file
type FileStore struct {
	mu       sync.RWMutex
	path     string
	jobs     map[string]*Job
	modified bool
}

// NewFileStore creates a new file-based store
func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{
		path: path,
		jobs: make(map[string]*Job),
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Load existing data
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load jobs: %w", err)
	}

	return s, nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	var jobs []*Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return err
	}

	for _, job := range jobs {
		s.jobs[job.ID] = job
	}

	return nil
}

func (s *FileStore) save() error {
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.path)
}

func (s *FileStore) Save(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *job
	s.jobs[job.ID] = &copy
	return s.save()
}

func (s *FileStore) Get(id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	copy := *job
	return &copy, nil
}

func (s *FileStore) List() ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		copy := *job
		jobs = append(jobs, &copy)
	}
	return jobs, nil
}

func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	return s.save()
}

func (s *FileStore) UpdateLastRun(id string, lastRun time.Time, nextRun *time.Time, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	job.LastRun = &lastRun
	job.NextRun = nextRun
	job.RunCount++
	if err != nil {
		job.ErrorCount++
		job.LastError = err.Error()
	}
	return s.save()
}

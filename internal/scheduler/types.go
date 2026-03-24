// Package scheduler provides generic scheduling for zen-brain.
// Schedules are defined in YAML and support recurring, nights, weekends, catch-up.
package scheduler

import (
	"time"
)

// Schedule defines when work should run.
type Schedule struct {
	// Name is the schedule identifier
	Name string `yaml:"name"`

	// Description explains what this schedule does
	Description string `yaml:"description"`

	// Type is schedule type: recurring, once, cron
	Type string `yaml:"type"`

	// Interval is duration between runs (for recurring)
	Interval string `yaml:"interval,omitempty"`

	// CronExpr is cron expression (for cron type)
	CronExpr string `yaml:"cron_expr,omitempty"`

	// TimeWindow restricts execution to specific times
	TimeWindow *TimeWindow `yaml:"time_window,omitempty"`

	// RetryPolicy defines how to handle failures
	RetryPolicy *RetryPolicy `yaml:"retry_policy,omitempty"`

	// TaskTemplate is the template to execute
	TaskTemplate string `yaml:"task_template"`

	// Enabled controls whether schedule is active
	Enabled bool `yaml:"enabled"`
}

// TimeWindow restricts when a schedule can run.
type TimeWindow struct {
	// StartHour is the hour (0-23) when window opens
	StartHour int `yaml:"start_hour"`

	// EndHour is the hour (0-23) when window closes
	EndHour int `yaml:"end_hour"`

	// Days is list of days (0=Sunday, 6=Saturday), empty = all days
	Days []int `yaml:"days,omitempty"`

	// TimeZone for interpreting hours
	TimeZone string `yaml:"timezone,omitempty"`
}

// RetryPolicy defines how to retry failed scheduled work.
type RetryPolicy struct {
	// MaxRetries is maximum retry attempts
	MaxRetries int `yaml:"max_retries"`

	// Backoff is duration between retries
	Backoff string `yaml:"backoff"`

	// CatchUp enables running missed schedules after outage
	CatchUp bool `yaml:"catch_up"`
}

// IsInWindow checks if current time is within the schedule window.
func (tw *TimeWindow) IsInWindow(now time.Time) bool {
	if tw == nil {
		return true // No window restriction
	}

	hour := now.Hour()
	if hour < tw.StartHour || hour >= tw.EndHour {
		return false
	}

	if len(tw.Days) > 0 {
		weekday := int(now.Weekday())
		found := false
		for _, d := range tw.Days {
			if d == weekday {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// NextRun calculates the next run time for a schedule.
func (s *Schedule) NextRun(lastRun time.Time) (time.Time, error) {
	switch s.Type {
	case "recurring":
		interval, err := time.ParseDuration(s.Interval)
		if err != nil {
			return time.Time{}, err
		}
		return lastRun.Add(interval), nil

	case "cron":
		// TODO: implement cron parsing
		return time.Time{}, nil

	default:
		return time.Time{}, nil
	}
}

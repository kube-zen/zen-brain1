// Copyright 2025 The Zen SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dlq

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/kube-zen/zen-sdk/pkg/logging"
)

// API provides HTTP endpoints for DLQ management
// Reused from zen-ingester pattern for consistent DLQ API across components
type API struct {
	manager *Manager
	logger  *logging.Logger
}

// NewAPI creates a new DLQ API
func NewAPI(manager *Manager, logger *logging.Logger) *API {
	return &API{
		manager: manager,
		logger:  logger,
	}
}

// HandleListFailedEvents handles GET /api/v1/dlq/events
func (a *API) HandleListFailedEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	filter := &Filter{}
	if source := r.URL.Query().Get("source"); source != "" {
		filter.Source = source
	}
	if destination := r.URL.Query().Get("destination"); destination != "" {
		filter.Destination = destination
	}
	if errorType := r.URL.Query().Get("error_type"); errorType != "" {
		filter.ErrorType = errorType
	}
	if errorCategory := r.URL.Query().Get("error_category"); errorCategory != "" {
		filter.ErrorCategory = errorCategory
	}
	if minRetryStr := r.URL.Query().Get("min_retry_count"); minRetryStr != "" {
		if minRetry, err := strconv.Atoi(minRetryStr); err == nil {
			filter.MinRetryCount = minRetry
		}
	}
	if maxRetryStr := r.URL.Query().Get("max_retry_count"); maxRetryStr != "" {
		if maxRetry, err := strconv.Atoi(maxRetryStr); err == nil {
			filter.MaxRetryCount = maxRetry
		}
	}
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if since, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			filter.Since = &since
		}
	}
	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		if until, err := time.Parse(time.RFC3339, untilStr); err == nil {
			filter.Until = &until
		}
	}

	events := a.manager.ListFailedEvents(filter)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"count":  len(events),
	}); err != nil {
		a.logger.WithContext(ctx).Warn("Failed to encode DLQ events response",
			logging.Operation("dlq_list"),
			logging.Error(err),
		)
	}
}

// HandleGetFailedEvent handles GET /api/v1/dlq/events/:id
func (a *API) HandleGetFailedEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := r.URL.Path
	prefix := "/api/v1/dlq/events/"
	if len(path) <= len(prefix) {
		http.Error(w, "Invalid event ID", http.StatusBadRequest)
		return
	}
	eventID := path[len(prefix):]

	event, exists := a.manager.GetFailedEvent(eventID)
	if !exists {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(event); err != nil {
		a.logger.WithContext(ctx).Warn("Failed to encode DLQ event response",
			logging.Operation("dlq_get"),
			logging.String("event_id", eventID),
			logging.Error(err),
		)
	}
}

// HandleReplayFailedEvent handles POST /api/v1/dlq/events/:id/replay
func (a *API) HandleReplayFailedEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Extract event ID from path (e.g., /api/v1/dlq/events/abc123/replay)
	path := r.URL.Path
	prefix := "/api/v1/dlq/events/"
	suffix := "/replay"
	if len(path) <= len(prefix)+len(suffix) {
		http.Error(w, "Invalid event ID", http.StatusBadRequest)
		return
	}
	eventID := path[len(prefix) : len(path)-len(suffix)]

	event, exists := a.manager.ReplayFailedEvent(eventID)
	if !exists {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"event":   event,
		"message": "Event marked for replay",
	}); err != nil {
		a.logger.WithContext(ctx).Warn("Failed to encode DLQ replay response",
			logging.Operation("dlq_replay"),
			logging.String("event_id", eventID),
			logging.Error(err),
		)
	}
}

// HandleDeleteFailedEvent handles DELETE /api/v1/dlq/events/:id
func (a *API) HandleDeleteFailedEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Extract event ID from path
	path := r.URL.Path
	prefix := "/api/v1/dlq/events/"
	if len(path) <= len(prefix) {
		http.Error(w, "Invalid event ID", http.StatusBadRequest)
		return
	}
	eventID := path[len(prefix):]

	if !a.manager.RemoveFailedEvent(eventID) {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Event removed from DLQ",
	}); err != nil {
		a.logger.WithContext(ctx).Warn("Failed to encode DLQ delete response",
			logging.Operation("dlq_delete"),
			logging.String("event_id", eventID),
			logging.Error(err),
		)
	}
}

// HandleGetStats handles GET /api/v1/dlq/stats
func (a *API) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := a.manager.GetStats()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		ctx := r.Context()
		a.logger.WithContext(ctx).Warn("Failed to encode DLQ stats response",
			logging.Operation("dlq_stats"),
			logging.Error(err),
		)
	}
}

package session

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/journal"
	"github.com/kube-zen/zen-brain1/pkg/messagebus"
)

const defaultEventStream = "zen-brain.events"

func eventStream(cfg *Config) string {
	if cfg != nil && cfg.EventStream != "" {
		return cfg.EventStream
	}
	return defaultEventStream
}

// EmitSessionCreated records session.created to journal and message bus when configured (non-fatal on failure).
func EmitSessionCreated(ctx context.Context, cfg *Config, session *contracts.Session, workItemID string) {
	stream := eventStream(cfg)
	now := time.Now()
	payload := map[string]interface{}{
		"session_id":   session.ID,
		"work_item_id": workItemID,
		"state":        string(session.State),
		"actor":        "session-manager",
		"timestamp":    now.Format(time.RFC3339),
	}
	if cfg != nil && cfg.Journal != nil {
		entry := journal.Entry{
			EventType:     journal.EventSessionCreated,
			Actor:         "session-manager",
			CorrelationID: session.ID,
			TaskID:        workItemID,
			SessionID:     session.ID,
			ClusterID:     "default",
			Payload:       payload,
			Timestamp:     now,
		}
		if _, err := cfg.Journal.Record(ctx, entry); err != nil {
			log.Printf("Warning: journal Record(session.created) failed: %v", err)
		}
	}
	if cfg != nil && cfg.EventBus != nil {
		payloadBytes, _ := json.Marshal(payload)
		ev := &messagebus.Event{
			Type:        "session.created",
			Source:      "session-manager",
			Correlation: session.ID,
			Payload:     payloadBytes,
			Timestamp:   now,
		}
		if err := cfg.EventBus.Publish(ctx, stream, ev); err != nil {
			log.Printf("Warning: event bus Publish(session.created) failed: %v", err)
		}
	}
}

// EmitSessionTransitioned records session.transitioned to journal and message bus when configured.
func EmitSessionTransitioned(ctx context.Context, cfg *Config, sessionID, workItemID, fromState, toState, reason, agent string) {
	stream := eventStream(cfg)
	now := time.Now()
	payload := map[string]interface{}{
		"session_id":   sessionID,
		"work_item_id": workItemID,
		"from_state":   fromState,
		"to_state":     toState,
		"reason":       reason,
		"actor":        agent,
		"timestamp":    now.Format(time.RFC3339),
	}
	if cfg != nil && cfg.Journal != nil {
		entry := journal.Entry{
			EventType:     journal.EventSessionTransitioned,
			Actor:         agent,
			CorrelationID: sessionID,
			TaskID:        workItemID,
			SessionID:     sessionID,
			ClusterID:     "default",
			Payload:       payload,
			Timestamp:     now,
		}
		if _, err := cfg.Journal.Record(ctx, entry); err != nil {
			log.Printf("Warning: journal Record(session.transitioned) failed: %v", err)
		}
	}
	if cfg != nil && cfg.EventBus != nil {
		payloadBytes, _ := json.Marshal(payload)
		ev := &messagebus.Event{
			Type:        "session.transitioned",
			Source:      "session-manager",
			Correlation: sessionID,
			Payload:     payloadBytes,
			Timestamp:   now,
		}
		if err := cfg.EventBus.Publish(ctx, stream, ev); err != nil {
			log.Printf("Warning: event bus Publish(session.transitioned) failed: %v", err)
		}
	}
}

// EmitSessionEvidenceAdded records session.evidence_added to journal and message bus when configured.
func EmitSessionEvidenceAdded(ctx context.Context, cfg *Config, sessionID, workItemID string, evidence contracts.EvidenceItem) {
	stream := eventStream(cfg)
	now := time.Now()
	payload := map[string]interface{}{
		"session_id":    sessionID,
		"work_item_id":  workItemID,
		"evidence_id":   evidence.ID,
		"evidence_type":  string(evidence.Type),
		"actor":         evidence.CollectedBy,
		"timestamp":     now.Format(time.RFC3339),
	}
	if cfg != nil && cfg.Journal != nil {
		entry := journal.Entry{
			EventType:     journal.EventSessionEvidenceAdded,
			Actor:         evidence.CollectedBy,
			CorrelationID: sessionID,
			TaskID:        workItemID,
			SessionID:     sessionID,
			ClusterID:     "default",
			Payload:       payload,
			Timestamp:     now,
		}
		if _, err := cfg.Journal.Record(ctx, entry); err != nil {
			log.Printf("Warning: journal Record(session.evidence_added) failed: %v", err)
		}
	}
	if cfg != nil && cfg.EventBus != nil {
		payloadBytes, _ := json.Marshal(payload)
		ev := &messagebus.Event{
			Type:        "session.evidence_added",
			Source:      "session-manager",
			Correlation: sessionID,
			Payload:     payloadBytes,
			Timestamp:   now,
		}
		if err := cfg.EventBus.Publish(ctx, stream, ev); err != nil {
			log.Printf("Warning: event bus Publish(session.evidence_added) failed: %v", err)
		}
	}
}

// EmitSessionCheckpointUpdated records session.checkpoint_updated to journal and message bus when configured.
func EmitSessionCheckpointUpdated(ctx context.Context, cfg *Config, sessionID, workItemID, stage string) {
	stream := eventStream(cfg)
	now := time.Now()
	payload := map[string]interface{}{
		"session_id":    sessionID,
		"work_item_id":  workItemID,
		"checkpoint_stage": stage,
		"actor":         "session-manager",
		"timestamp":     now.Format(time.RFC3339),
	}
	if cfg != nil && cfg.Journal != nil {
		entry := journal.Entry{
			EventType:     journal.EventSessionCheckpointUpdated,
			Actor:         "session-manager",
			CorrelationID: sessionID,
			TaskID:        workItemID,
			SessionID:     sessionID,
			ClusterID:     "default",
			Payload:       payload,
			Timestamp:     now,
		}
		if _, err := cfg.Journal.Record(ctx, entry); err != nil {
			log.Printf("Warning: journal Record(session.checkpoint_updated) failed: %v", err)
		}
	}
	if cfg != nil && cfg.EventBus != nil {
		payloadBytes, _ := json.Marshal(payload)
		ev := &messagebus.Event{
			Type:        "session.checkpoint_updated",
			Source:      "session-manager",
			Correlation: sessionID,
			Payload:     payloadBytes,
			Timestamp:   now,
		}
		if err := cfg.EventBus.Publish(ctx, stream, ev); err != nil {
			log.Printf("Warning: event bus Publish(session.checkpoint_updated) failed: %v", err)
		}
	}
}

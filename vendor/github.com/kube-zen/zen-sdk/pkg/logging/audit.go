/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logging

import (
	"context"

	"go.uber.org/zap"
)

// AuditAction represents the type of action being audited
type AuditAction string

const (
	// AuditActionCreate represents a create operation
	AuditActionCreate AuditAction = "create"
	// AuditActionRead represents a read/access operation
	AuditActionRead AuditAction = "read"
	// AuditActionUpdate represents an update operation
	AuditActionUpdate AuditAction = "update"
	// AuditActionDelete represents a delete operation
	AuditActionDelete AuditAction = "delete"
	// AuditActionLogin represents a login/authentication operation
	AuditActionLogin AuditAction = "login"
	// AuditActionLogout represents a logout operation
	AuditActionLogout AuditAction = "logout"
	// AuditActionAuthorize represents an authorization decision
	AuditActionAuthorize AuditAction = "authorize"
	// AuditActionConfigChange represents a configuration change
	AuditActionConfigChange AuditAction = "config_change"
	// AuditActionDataAccess represents access to sensitive data
	AuditActionDataAccess AuditAction = "data_access"
)

// AuditResult represents the result of an audit action
type AuditResult string

const (
	// AuditResultSuccess represents a successful operation
	AuditResultSuccess AuditResult = "success"
	// AuditResultFailure represents a failed operation
	AuditResultFailure AuditResult = "failure"
	// AuditResultDenied represents a denied operation
	AuditResultDenied AuditResult = "denied"
)

// AuditLogger provides standardized audit logging helpers for security-sensitive operations
type AuditLogger struct {
	logger *Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger *Logger) *AuditLogger {
	return &AuditLogger{logger: logger}
}

// LogUserAction logs a user action for audit purposes
func (al *AuditLogger) LogUserAction(ctx context.Context, action AuditAction, resourceType, resourceID string, result AuditResult, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("audit_type", "user_action"),
		zap.String("audit_action", string(action)),
		zap.String("audit_result", string(result)),
		ResourceType(resourceType),
		ResourceID(resourceID),
	}
	allFields = append(allFields, fields...)

	// Extract user context
	if userID := GetUserID(ctx); userID != "" {
		allFields = append(allFields, UserID(userID, true))
	}

	al.logger.WithContext(ctx).Info("Audit: User action", allFields...)
}

// LogLogin logs a login/authentication event
func (al *AuditLogger) LogLogin(ctx context.Context, result AuditResult, ipAddress, userAgent string, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("audit_type", "authentication"),
		zap.String("audit_action", string(AuditActionLogin)),
		zap.String("audit_result", string(result)),
		RemoteAddr(MaskIP(ipAddress)), // Mask IP for security
		UserAgent(userAgent),
	}
	allFields = append(allFields, fields...)

	// Extract user context
	if userID := GetUserID(ctx); userID != "" {
		allFields = append(allFields, UserID(userID, true))
	}

	al.logger.WithContext(ctx).Info("Audit: Login attempt", allFields...)
}

// LogAuthorization logs an authorization decision
func (al *AuditLogger) LogAuthorization(ctx context.Context, resourceType, resourceID, permission string, result AuditResult, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("audit_type", "authorization"),
		zap.String("audit_action", string(AuditActionAuthorize)),
		zap.String("audit_result", string(result)),
		ResourceType(resourceType),
		ResourceID(resourceID),
		zap.String("permission", permission),
	}
	allFields = append(allFields, fields...)

	// Extract user context
	if userID := GetUserID(ctx); userID != "" {
		allFields = append(allFields, UserID(userID, true))
	}

	al.logger.WithContext(ctx).Info("Audit: Authorization decision", allFields...)
}

// LogConfigChange logs a configuration change
func (al *AuditLogger) LogConfigChange(ctx context.Context, configKey string, oldValue, newValue interface{}, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("audit_type", "config_change"),
		zap.String("audit_action", string(AuditActionConfigChange)),
		zap.String("audit_result", string(AuditResultSuccess)),
		zap.String("config_key", configKey),
	}
	// Only log non-sensitive config values (caller should redact sensitive values)
	if oldValue != nil {
		allFields = append(allFields, zap.Any("config_old_value", oldValue))
	}
	if newValue != nil {
		allFields = append(allFields, zap.Any("config_new_value", newValue))
	}
	allFields = append(allFields, fields...)

	// Extract user context
	if userID := GetUserID(ctx); userID != "" {
		allFields = append(allFields, UserID(userID, true))
	}

	al.logger.WithContext(ctx).Info("Audit: Configuration change", allFields...)
}

// LogDataAccess logs access to sensitive data (PII, etc.)
func (al *AuditLogger) LogDataAccess(ctx context.Context, dataType, resourceID string, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("audit_type", "data_access"),
		zap.String("audit_action", string(AuditActionDataAccess)),
		zap.String("audit_result", string(AuditResultSuccess)),
		zap.String("data_type", dataType),
		ResourceID(resourceID),
	}
	allFields = append(allFields, fields...)

	// Extract user context
	if userID := GetUserID(ctx); userID != "" {
		allFields = append(allFields, UserID(userID, true))
	}

	al.logger.WithContext(ctx).Info("Audit: Data access", allFields...)
}

// LogResourceOperation logs a create/update/delete operation on a resource
func (al *AuditLogger) LogResourceOperation(ctx context.Context, action AuditAction, resourceType, resourceID string, result AuditResult, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("audit_type", "resource_operation"),
		zap.String("audit_action", string(action)),
		zap.String("audit_result", string(result)),
		ResourceType(resourceType),
		ResourceID(resourceID),
	}
	allFields = append(allFields, fields...)

	// Extract user context
	if userID := GetUserID(ctx); userID != "" {
		allFields = append(allFields, UserID(userID, true))
	}

	al.logger.WithContext(ctx).Info("Audit: Resource operation", allFields...)
}

// Audit field helpers

// AuditActionField creates an audit_action field
func AuditActionField(action AuditAction) zap.Field {
	return zap.String("audit_action", string(action))
}

// AuditResultField creates an audit_result field
func AuditResultField(result AuditResult) zap.Field {
	return zap.String("audit_result", string(result))
}

// AuditTypeField creates an audit_type field
func AuditTypeField(auditType string) zap.Field {
	return zap.String("audit_type", auditType)
}

// PermissionField creates a permission field (for authorization audits)
func PermissionField(permission string) zap.Field {
	return zap.String("permission", permission)
}

// ConfigKeyField creates a config_key field (for config change audits)
func ConfigKeyField(key string) zap.Field {
	return zap.String("config_key", key)
}

// DataTypeField creates a data_type field (for data access audits)
func DataTypeField(dataType string) zap.Field {
	return zap.String("data_type", dataType)
}

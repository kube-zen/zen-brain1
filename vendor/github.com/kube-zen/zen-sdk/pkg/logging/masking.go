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
	"regexp"
	"strings"
)

// MaskUUID masks a UUID for logging (shows first 8 chars)
// Security: Prevents leaking full UUIDs in logs
func MaskUUID(uuid string) string {
	if uuid == "" {
		return ""
	}
	if len(uuid) <= 8 {
		return uuid
	}
	return uuid[:8] + "..."
}

// MaskToken masks a token/API key (shows first 4 chars)
// Security: Prevents leaking credentials in logs
func MaskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 4 {
		return "***"
	}
	return token[:4] + "..."
}

// MaskEmail masks an email address (shows first 3 chars + domain)
// Security: Protects PII in logs
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	if len(parts[0]) <= 3 {
		return "***@" + parts[1]
	}
	return parts[0][:3] + "..." + "@" + parts[1]
}

// MaskIP masks IP addresses (shows first octet)
// Security: Protects client IP addresses
func MaskIP(ip string) string {
	if ip == "" {
		return ""
	}
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + ".x.x.x"
	}
	return "***"
}

// SanitizeSQL sanitizes SQL queries for logging (removes values, keeps structure)
// Security: Prevents leaking sensitive data from SQL queries
func SanitizeSQL(query string) string {
	if query == "" {
		return ""
	}

	// Remove string literals
	re := regexp.MustCompile(`'[^']*'`)
	query = re.ReplaceAllString(query, "'?'")

	// Remove numeric literals (simple approach)
	re = regexp.MustCompile(`\b\d+\b`)
	query = re.ReplaceAllString(query, "?")

	// Remove UUIDs
	uuidRe := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	query = uuidRe.ReplaceAllString(query, "?")

	// Limit length
	if len(query) > 200 {
		query = query[:200] + "..."
	}

	return query
}

// RedactPassword ensures passwords are never logged
// Security: Hard redaction for passwords
func RedactPassword(password string) string {
	return "[REDACTED]"
}

// RedactSecret ensures secrets are never logged
// Security: Hard redaction for secrets
func RedactSecret(secret string) string {
	return "[REDACTED]"
}

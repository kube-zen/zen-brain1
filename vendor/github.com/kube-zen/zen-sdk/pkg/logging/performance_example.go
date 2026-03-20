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

// Example usage of performance logging helpers:
//
// // Create a performance logger
// baseLogger := logging.NewLogger("my-component")
// perfLogger := logging.NewPerformanceLogger(baseLogger)
//
// // Log HTTP request processing
// perfLogger.LogRequestProcessed(ctx, "user_list",
//     duration,
//     200,
//     requestSize,
//     responseSize,
//     logging.RequestID(requestID),
//     logging.TenantID(tenantID, true),
// )
//
// // Log database operation
// perfLogger.LogDBCall(ctx, "select_users", query, duration, rowsAffected, err,
//     logging.String("table", "users"),
// )
//
// // Log cache operation
// perfLogger.LogCacheOperation(ctx, "get", cacheKey, hit, duration, err,
//     logging.String("cache_type", "redis"),
// )
//
// // Log external API call
// perfLogger.LogExternalAPICall(ctx, "payment-service", "/process", "POST",
//     statusCode, duration, requestSize, responseSize, err,
//     logging.String("api_version", "v1"),
// )
//
// // Measure duration of an operation
// err := perfLogger.MeasureDuration(ctx, "complex_operation", func() error {
//     // Do work
//     return nil
// })
//
// // With additional fields
// err := perfLogger.MeasureDurationWithFields(ctx, "operation", []zap.Field{
//     logging.String("param1", value1),
//     logging.Int("param2", value2),
// }, func() error {
//     // Do work
//     return nil
// })

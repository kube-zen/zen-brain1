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

// Example usage of sampling and rate limiting:
//
// // Create a sampled logger
// baseLogger := logging.NewLogger("my-component")
// sampledLogger := logging.NewSampledLogger(
//     baseLogger,
//     logging.DefaultSamplerConfig(),
//     logging.DefaultRateLimiterConfig(),
// )
//
// // For high-volume success logs (e.g., 200 OK responses)
// sampledLogger.Info("Request completed", true, "http_request", // isSuccess=true, key="http_request"
//     logging.HTTPStatus(200),
//     logging.Latency(duration),
// )
//
// // For errors (always logged, but rate limited)
// sampledLogger.Error(err, "Request failed", "http_request_error", // key for rate limiting
//     logging.HTTPStatus(500),
// )
//
// // Custom sampling config for high-volume services
// customSamplerConfig := logging.SamplerConfig{
//     InfoSamplingRate:    0.01, // 1% of INFO logs
//     SuccessSamplingRate: 0.001, // 0.1% of success logs
//     ErrorSamplingRate:   1.0,   // 100% of errors
//     WarnSamplingRate:    1.0,   // 100% of warnings
// }
//
// // Custom rate limiter config
// customRateLimiterConfig := logging.RateLimiterConfig{
//     MaxLogsPerSecond: 5,              // More restrictive: 5 logs/sec
//     WindowSize:       time.Second,
//     CleanupInterval:  30 * time.Second,
// }

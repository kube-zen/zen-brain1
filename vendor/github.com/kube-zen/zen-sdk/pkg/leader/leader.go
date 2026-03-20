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

// Package leader provides a simple wrapper around controller-runtime's built-in leader election.
// It standardizes leader election configuration across all Zen tools, providing a consistent
// API for enabling high availability.
//
// Usage:
//
//	opts := leader.Options{
//	    LeaseName: "my-controller",
//	    Enable:    true,
//	}
//	mgr, err := ctrl.NewManager(cfg, ctrl.Options{}, leader.Setup(opts))
//
// All controllers use this function for consistency. zen-lead always passes enable=true (mandatory HA).
package leader

import (
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ApplyLeaderElection configures leader election for controller-runtime Manager.
// Leader election is enabled by default but can be disabled via the enable parameter.
// All controllers use this function for consistency. zen-lead always passes enable=true (mandatory HA).
// Other controllers expose a flag/env var that defaults to true but can be set to false
// if the user doesn't want HA or wants zen-lead to handle HA instead.
//
// Parameters:
//   - opts: Pointer to ctrl.Options to modify
//   - component: Component name (e.g., "zen-flow-controller", "zen-gc-controller")
//   - namespace: Namespace for leader election Lease (required if enabled)
//   - idOverride: Optional override for leader election ID. If empty, uses component-based ID.
//   - enable: Whether to enable leader election (default: true for all controllers except zen-lead which is always true)
func ApplyLeaderElection(opts *ctrl.Options, component string, namespace string, idOverride string, enable bool) {
	if !enable {
		opts.LeaderElection = false
		return
	}

	// Enable leader election
	opts.LeaderElection = true

	// Set leader election ID
	if idOverride != "" {
		opts.LeaderElectionID = idOverride
	} else {
		opts.LeaderElectionID = fmt.Sprintf("%s-leader-election", component)
	}

	// Set namespace (required when enabled)
	opts.LeaderElectionNamespace = namespace

	// Set ReleaseOnCancel to ensure clean shutdown
	opts.LeaderElectionReleaseOnCancel = true
}

// ApplyRequiredLeaderElection is a convenience wrapper for ApplyLeaderElection that always enables leader election.
// Use this for zen-lead which requires mandatory HA (no option to disable).
// This is equivalent to calling ApplyLeaderElection with enable=true.
func ApplyRequiredLeaderElection(opts *ctrl.Options, component string, namespace string, idOverride string) {
	ApplyLeaderElection(opts, component, namespace, idOverride, true)
}

// RequirePodNamespace returns the pod namespace from environment or service account file.
// This function hard-fails if namespace cannot be determined (required for leader election).
//
// Returns:
//   - namespace: Pod namespace
//   - error: If namespace cannot be determined
func RequirePodNamespace() (string, error) {
	// Try environment variable first (Downward API)
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns, nil
	}

	// Fallback to service account namespace file
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := string(data); ns != "" {
			return ns, nil
		}
	}

	return "", fmt.Errorf("POD_NAMESPACE environment variable must be set or service account namespace file must be readable")
}

// ApplyRestConfigDefaults sets recommended REST client defaults for controller-runtime.
// This ensures consistent QPS/Burst settings across all controllers using zen-sdk.
//
// Parameters:
//   - config: Pointer to rest.Config to modify
func ApplyRestConfigDefaults(config *rest.Config) {
	if config.QPS == 0 {
		config.QPS = 50 // Default is 20, increase for faster reconciliation
	}
	if config.Burst == 0 {
		config.Burst = 100 // Default is 30, increase for burst handling
	}
}

// Options configures leader election for controller-runtime Manager (legacy API, kept for compatibility)
type Options struct {
	// LeaseName is the name of the Lease resource used for leader election
	LeaseName string

	// Enable enables leader election (deprecated: use ApplyRequiredLeaderElection for mandatory LE)
	Enable bool

	// Namespace is the namespace where the Lease resource is created
	Namespace string

	// LeaseDuration is how long a leader holds the lease before it expires
	LeaseDuration time.Duration

	// RenewDeadline is the time to renew the lease before losing leadership
	RenewDeadline time.Duration

	// RetryPeriod is how often to retry acquiring leadership
	RetryPeriod time.Duration
}

// DefaultOptions returns default leader election options (legacy API)
func DefaultOptions(leaseName string) Options {
	return Options{
		LeaseName:     leaseName,
		Enable:        false,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
	}
}

// Setup configures leader election options for controller-runtime Manager (legacy API)
func Setup(opts Options) func(*ctrl.Options) {
	return func(managerOpts *ctrl.Options) {
		if !opts.Enable {
			return
		}

		if opts.LeaseName != "" {
			managerOpts.LeaderElectionID = opts.LeaseName
		}

		managerOpts.LeaderElection = true
		managerOpts.LeaderElectionReleaseOnCancel = true

		if opts.Namespace != "" {
			managerOpts.LeaderElectionNamespace = opts.Namespace
		}

		if opts.LeaseDuration > 0 {
			managerOpts.LeaseDuration = func() *time.Duration {
				d := opts.LeaseDuration
				return &d
			}()
		}

		if opts.RenewDeadline > 0 {
			managerOpts.RenewDeadline = func() *time.Duration {
				d := opts.RenewDeadline
				return &d
			}()
		}

		if opts.RetryPeriod > 0 {
			managerOpts.RetryPeriod = func() *time.Duration {
				d := opts.RetryPeriod
				return &d
			}()
		}
	}
}

// ManagerOptions returns ctrl.Options with leader election configured (legacy API)
func ManagerOptions(baseOpts *ctrl.Options, leaderOpts Options) ctrl.Options {
	setupFunc := Setup(leaderOpts)
	setupFunc(baseOpts)
	return *baseOpts
}

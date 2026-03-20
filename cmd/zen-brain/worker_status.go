// Package main provides worker status CLI for zen-brain1 (ZB-026).
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
)

func init() {
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func main() {
	cmd := &cobra.Command{
		Use:   "worker-status",
		Short: "Zen-Brain worker pool status CLI",
	}

	cmd.AddCommand(
		NewStatusCommand(),
		NewTasksCommand(),
		NewQueueCommand(),
		NewWorkersCommand(),
		NewFailuresCommand(),
	)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// NewStatusCommand creates the status command.
func NewStatusCommand() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get overall worker pool status",
		RunE: func(cmd *cobra.Command, args []string) error {
			k8sClient, err := getClient()
			if err != nil {
				return err
			}

			return showOverallStatus(k8sClient, namespace)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "zen-brain", "Namespace")

	return cmd
}

// NewTasksCommand creates the tasks command.
func NewTasksCommand() *cobra.Command {
	var namespace string
	var phase string
	var watch bool

	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "List BrainTasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			k8sClient, err := getClient()
			if err != nil {
				return err
			}

			if watch {
				return watchTasks(k8sClient, namespace, phase)
			}

			return listTasks(k8sClient, namespace, phase)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "zen-brain", "Namespace")
	cmd.Flags().StringVarP(&phase, "phase", "p", "", "Filter by phase (Pending/Running/Completed/Failed)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	return cmd
}

// NewQueueCommand creates the queue command.
func NewQueueCommand() *cobra.Command {
	var namespace string
	var queueName string

	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Get queue status",
		RunE: func(cmd *cobra.Command, args []string) error {
			k8sClient, err := getClient()
			if err != nil {
				return err
			}

			return showQueueStatus(k8sClient, namespace, queueName)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "zen-brain", "Namespace")
	cmd.Flags().StringVarP(&queueName, "name", "q", "dogfood", "Queue name")

	return cmd
}

// NewWorkersCommand creates the workers command.
func NewWorkersCommand() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "workers",
		Short: "List active workers",
		RunE: func(cmd *cobra.Command, args []string) error {
			k8sClient, err := getClient()
			if err != nil {
				return err
			}

			return listWorkers(k8sClient, namespace)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "zen-brain", "Namespace")

	return cmd
}

// NewFailuresCommand creates the failures command.
func NewFailuresCommand() *cobra.Command {
	var namespace string
	var detailed bool

	cmd := &cobra.Command{
		Use:   "failures",
		Short: "Show failed tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			k8sClient, err := getClient()
			if err != nil {
				return err
			}

			return showFailures(k8sClient, namespace, detailed)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "zen-brain", "Namespace")
	cmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Show detailed error messages")

	return cmd
}

// getClient creates a Kubernetes client.
func getClient() (client.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return k8sClient, nil
}

// showOverallStatus shows overall worker pool status.
func showOverallStatus(k8sClient client.Client, namespace string) error {
	ctx := context.Background()

	// Get all tasks
	taskList := &v1alpha1.BrainTaskList{}
	if err := k8sClient.List(ctx, taskList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	// Count by phase
	byPhase := make(map[v1alpha1.BrainTaskPhase]int)
	for _, task := range taskList.Items {
		byPhase[task.Status.Phase]++
	}

	// Get queue status
	queueList := &v1alpha1.BrainQueueList{}
	if err := k8sClient.List(ctx, queueList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list queues: %w", err)
	}

	// Print status
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PHASE\tCOUNT")
	fmt.Fprintf(w, "Pending\t%d\n", byPhase[v1alpha1.BrainTaskPhasePending])
	fmt.Fprintf(w, "Scheduled\t%d\n", byPhase[v1alpha1.BrainTaskPhaseScheduled])
	fmt.Fprintf(w, "Running\t%d\n", byPhase[v1alpha1.BrainTaskPhaseRunning])
	fmt.Fprintf(w, "Completed\t%d\n", byPhase[v1alpha1.BrainTaskPhaseCompleted])
	fmt.Fprintf(w, "Failed\t%d\n", byPhase[v1alpha1.BrainTaskPhaseFailed])
	fmt.Fprintf(w, "Canceled\t%d\n", byPhase[v1alpha1.BrainTaskPhaseCanceled])
	fmt.Fprintln(w)
	fmt.Fprintln(w, "QUEUE\tPHASE\tDEPTH\tIN-FLIGHT")
	for _, queue := range queueList.Items {
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\n",
			queue.Name, queue.Status.Phase, queue.Status.Depth, queue.Status.InFlight)
	}
	w.Flush()

	return nil
}

// listTasks lists BrainTasks.
func listTasks(k8sClient client.Client, namespace, phase string) error {
	ctx := context.Background()

	taskList := &v1alpha1.BrainTaskList{}
	if err := k8sClient.List(ctx, taskList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	// Filter by phase
	var filtered []v1alpha1.BrainTask
	for _, task := range taskList.Items {
		if phase == "" || string(task.Status.Phase) == phase {
			filtered = append(filtered, task)
		}
	}

	// Print tasks
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPHASE\tSESSION\tAGE\tASSIGNED")
	for _, task := range filtered {
		age := time.Since(task.CreationTimestamp.Time).Round(time.Second)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			task.Name, task.Status.Phase, task.Spec.SessionID, age, task.Status.AssignedAgent)
	}
	w.Flush()

	return nil
}

// watchTasks watches BrainTasks for changes.
func watchTasks(k8sClient client.Client, namespace, phase string) error {
	// TODO: Implement watch with controller-runtime
	return fmt.Errorf("watch mode not yet implemented")
}

// showQueueStatus shows queue status.
func showQueueStatus(k8sClient client.Client, namespace, queueName string) error {
	ctx := context.Background()

	queue := &v1alpha1.BrainQueue{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: queueName}, queue); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("queue %s not found in namespace %s", queueName, namespace)
		}
		return fmt.Errorf("failed to get queue: %w", err)
	}

	// Print queue details
	fmt.Printf("Queue: %s\n", queue.Name)
	fmt.Printf("Phase: %s\n", queue.Status.Phase)
	fmt.Printf("Priority: %d\n", queue.Spec.Priority)
	fmt.Printf("Max Concurrency: %d\n", queue.Spec.MaxConcurrency)
	fmt.Printf("Session Affinity: %v\n", queue.Spec.SessionAffinity)
	fmt.Printf("Depth (Pending): %d\n", queue.Status.Depth)
	fmt.Printf("In-Flight (Running): %d\n", queue.Status.InFlight)

	if len(queue.Status.Conditions) > 0 {
		fmt.Println("\nConditions:")
		for _, cond := range queue.Status.Conditions {
			fmt.Printf("  %s: %s (%s)\n", cond.Type, cond.Status, cond.Message)
		}
	}

	return nil
}

// listWorkers lists active workers.
func listWorkers(k8sClient client.Client, namespace string) error {
	ctx := context.Background()

	agentList := &v1alpha1.BrainAgentList{}
	if err := k8sClient.List(ctx, agentList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agentList.Items) == 0 {
		fmt.Println("No active workers found")
		return nil
	}

	// Print workers
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tROLE\tSTATUS\tTASKS\tAGE")
	for _, agent := range agentList.Items {
		age := time.Since(agent.CreationTimestamp.Time).Round(time.Second)
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			agent.Name, agent.Spec.Role, agent.Status.Phase, agent.Status.ActiveTasks, age)
	}
	w.Flush()

	return nil
}

// showFailures shows failed tasks.
func showFailures(k8sClient client.Client, namespace string, detailed bool) error {
	ctx := context.Background()

	taskList := &v1alpha1.BrainTaskList{}
	if err := k8sClient.List(ctx, taskList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	// Filter failed tasks
	var failed []v1alpha1.BrainTask
	for _, task := range taskList.Items {
		if task.Status.Phase == v1alpha1.BrainTaskPhaseFailed {
			failed = append(failed, task)
		}
	}

	if len(failed) == 0 {
		fmt.Println("No failed tasks found")
		return nil
	}

	// Print failures
	if detailed {
		for _, task := range failed {
			fmt.Printf("\n=== %s ===\n", task.Name)
			fmt.Printf("Phase: %s\n", task.Status.Phase)
			fmt.Printf("Message: %s\n", task.Status.Message)
			fmt.Printf("Assigned Agent: %s\n", task.Status.AssignedAgent)
			fmt.Printf("Source Key: %s\n", task.Spec.SourceKey)
			fmt.Printf("Work Type: %s\n", task.Spec.WorkType)
			fmt.Printf("Work Domain: %s\n", task.Spec.WorkDomain)
			fmt.Printf("Objective: %s\n", task.Spec.Objective)

			if len(task.Status.Conditions) > 0 {
				fmt.Println("\nConditions:")
				for _, cond := range task.Status.Conditions {
					fmt.Printf("  %s: %s (%s)\n", cond.Type, cond.Status, cond.Message)
				}
			}
		}
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tMESSAGE\tAGE")
		for _, task := range failed {
			age := time.Since(task.CreationTimestamp.Time).Round(time.Second)
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				task.Name, truncate(task.Status.Message, 50), age)
		}
		w.Flush()
	}

	return nil
}

// truncate truncates a string to max length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

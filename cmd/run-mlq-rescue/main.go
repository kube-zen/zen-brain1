package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

type BrainTaskStatus struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

type BrainTask struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status BrainTaskStatus `json:"status"`
}

func main() {
	log.Println("Creating MLQ rescue BrainTask with structured prompt...")

	// Apply BrainTask
	manifestPath := "mlq-rescue-structured-braintask.yaml"

	log.Println("Applying BrainTask with kubectl...")
	cmd := exec.Command("kubectl", "apply", "-f", manifestPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to apply BrainTask: %v", err)
	}

	log.Println("BrainTask created. Waiting for execution...")

	// Wait for task to complete
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	taskName := "mlq-rescue-structured"

	for {
		select {
		case <-ctx.Done():
			log.Println("Timeout waiting for task")
			return
		default:
			// Check task status
			cmd := exec.Command("kubectl", "get", "braintask", taskName, "-n", "zen-brain", "-o", "json")
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("Error checking status: %v, will retry...", err)
				time.Sleep(30 * time.Second)
				continue
			}

			var task BrainTask
			if err := yaml.Unmarshal(output, &task); err != nil {
				log.Printf("Error parsing status: %v, will retry...", err)
				time.Sleep(30 * time.Second)
				continue
			}

			log.Printf("Status: Phase=%s", task.Status.Phase)

			// Check for completion
			if task.Status.Phase == "Completed" {
				log.Println("✓ Task completed successfully!")
				evaluateResult()
				return
			}

			if task.Status.Phase == "Failed" {
				log.Println("✗ Task failed!")
				log.Printf("Message: %s", task.Status.Message)
				evaluateResult()
				return
			}

			time.Sleep(30 * time.Second)
		}
	}
}

func evaluateResult() {
	log.Println("\n=== EVALUATION ===")

	// Check workspace for output
	workspaceDir := filepath.Join("/tmp/zen-brain-factory/workspaces", "mlq-rescue-structured", "mlq-rescue-structured")
	log.Printf("Workspace: %s", workspaceDir)

	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		log.Println("No workspace directory found")
		return
	}

	// List generated files
	var goFiles []string
	filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			rel, _ := filepath.Rel(workspaceDir, path)
			goFiles = append(goFiles, rel)
			log.Printf("  Generated: %s (%d bytes)", rel, info.Size())
		}
		return nil
	})

	if len(goFiles) == 0 {
		log.Println("No Go files generated")
		return
	}

	// Compile check
	log.Println("\n=== COMPILE CHECK ===")
	repoDir := os.Getenv("HOME") + "/zen/zen-brain1"
	cmd := exec.Command("go", "build", "./internal/mlq/...")
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("✗ COMPILE FAILED: %v", err)
	} else {
		log.Println("✓ COMPILE SUCCESS")
	}

	// Check for fake imports
	log.Println("\n=== FAKE IMPORT CHECK ===")
	for _, f := range goFiles {
		fullPath := filepath.Join(workspaceDir, f)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		contentStr := string(content)

		// Check for known fake import patterns
		fakeImports := []string{
			"github.com/alexmiller/",
			"github.com/tidwall/",
			"github.com/stretchr/testify/mock",
			"github.com/example.com/",
		}

		for _, fake := range fakeImports {
			if strings.Contains(contentStr, fake) {
				log.Printf("✗ FAKE IMPORT DETECTED: %s in %s", fake, f)
			}
		}
	}

	log.Println("\n=== SUMMARY ===")
	log.Printf("Generated files: %d", len(goFiles))
	log.Printf("Workspace: %s", workspaceDir)
	log.Println("Manual review required to confirm output quality.")
}

package normalizer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ExecutionPacket represents the canonical bounded execution format
type ExecutionPacket struct {
	Version      string             `yaml:"version" json:"version"`
	TargetFiles  []TargetFile       `yaml:"target_files" json:"target_files"`
	Evidence     Evidence           `yaml:"evidence" json:"evidence"`
	Acceptance   []AcceptanceCriterion `yaml:"acceptance_criteria" json:"acceptance_criteria"`
	Validation   Validation         `yaml:"validation" json:"validation"`
	Scope        Scope              `yaml:"scope" json:"scope"`
	Metadata     InferenceMetadata  `yaml:"inference_metadata" json:"inference_metadata"`
}

type TargetFile struct {
	Path       string  `yaml:"path" json:"path"`
	Confidence float64 `yaml:"confidence" json:"confidence"`
	Reason     string  `yaml:"reason" json:"reason"`
	LineRange  []int   `yaml:"line_range,omitempty" json:"line_range,omitempty"`
}

type Evidence struct {
	Type       string  `yaml:"type" json:"type"`
	Content    string  `yaml:"content" json:"content"`
	Source     string  `yaml:"source" json:"source"`
	Confidence float64 `yaml:"confidence" json:"confidence"`
}

type AcceptanceCriterion struct {
	Description    string  `yaml:"description" json:"description"`
	ValidationCmd  string  `yaml:"validation_cmd,omitempty" json:"validation_cmd,omitempty"`
	ExpectedOutput string  `yaml:"expected_output,omitempty" json:"expected_output,omitempty"`
	Confidence     float64 `yaml:"confidence" json:"confidence"`
}

type Validation struct {
	BuildCmd      string   `yaml:"build_cmd,omitempty" json:"build_cmd,omitempty"`
	QualityGates  []string `yaml:"quality_gates,omitempty" json:"quality_gates,omitempty"`
	Confidence    float64  `yaml:"confidence" json:"confidence"`
}

type Scope struct {
	BlastRadius    string `yaml:"blast_radius" json:"blast_radius"`
	ExecutionClass string `yaml:"execution_class" json:"execution_class"`
	Bounded        bool   `yaml:"bounded" json:"bounded"`
}

type InferenceMetadata struct {
	GeneratedBy        string    `yaml:"generated_by" json:"generated_by"`
	GeneratedAt        string    `yaml:"generated_at" json:"generated_at"`
	InferenceSources   []string  `yaml:"inference_sources" json:"inference_sources"`
	OverallConfidence  float64   `yaml:"overall_confidence" json:"overall_confidence"`
}

// Normalizer infers execution packets from Jira tickets
type Normalizer struct {
	componentMappings map[string][]string
}

// NewNormalizer creates a new normalizer with default component mappings
func NewNormalizer() *Normalizer {
	return &Normalizer{
		componentMappings: map[string][]string{
			"scheduler":    {"internal/scheduler/*.go", "cmd/scheduler/*.go"},
			"factory-fill": {"cmd/factory-fill/*.go", "internal/factory/*.go"},
			"foreman":      {"internal/foreman/*.go", "cmd/foreman/*.go"},
			"worker":       {"cmd/zen-brain/*.go", "internal/worker/*.go"},
			"readiness":    {"internal/readiness/*.go"},
			"docs":         {"docs/**/*.md"},
			"config":       {"config/**/*.yaml", "config/**/*.yml"},
		},
	}
}

// InferPacket analyzes a Jira ticket and generates an execution packet
func (n *Normalizer) InferPacket(ticketKey, summary, description string) (*ExecutionPacket, error) {
	packet := &ExecutionPacket{
		Version: "1.0",
		Scope: Scope{
			BlastRadius:    "low",
			ExecutionClass: "unknown",
			Bounded:        false,
		},
		Metadata: InferenceMetadata{
			GeneratedBy:      "auto_normalizer_v1",
			InferenceSources: []string{},
		},
	}

	// Step 1: Infer target files
	targetFiles := n.inferTargetFiles(summary, description)
	packet.TargetFiles = targetFiles

	// Step 2: Infer evidence
	evidence := n.inferEvidence(description)
	packet.Evidence = evidence

	// Step 3: Infer acceptance criteria
	acceptance := n.inferAcceptanceCriteria(description)
	packet.Acceptance = acceptance

	// Step 4: Infer validation
	validation := n.inferValidation(targetFiles)
	packet.Validation = validation

	// Step 5: Classify scope and blast radius
	packet.Scope = n.classifyScope(targetFiles)

	// Step 6: Calculate overall confidence
	packet.Metadata.OverallConfidence = n.calculateOverallConfidence(packet)

	// Step 7: Determine if bounded
	packet.Scope.Bounded = packet.Metadata.OverallConfidence >= 0.70 && len(targetFiles) > 0

	return packet, nil
}

// inferTargetFiles extracts target file paths from ticket text
func (n *Normalizer) inferTargetFiles(summary, description string) []TargetFile {
	var files []TargetFile
	text := summary + " " + description

	// Pattern 1: Explicit file mentions
	// Matches: "File: path/to/file.go" or "In file.go"
	filePatterns := []string{
		`[Ff]ile:\s*([a-zA-Z0-9_\-/.]+\.(go|md|yaml|yml|json))`,
		`([a-zA-Z0-9_\-/]+/)+[a-zA-Z0-9_\-]+\.(go|md|yaml|yml|json)`,
		`\b(cmd|internal|pkg|docs|config)/[a-zA-Z0-9_\-/]+\.(go|md|yaml|yml|json)`,
	}

	for _, pattern := range filePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				filePath := match[1]
				files = append(files, TargetFile{
					Path:       filePath,
					Confidence: 0.95,
					Reason:     "Explicitly mentioned in ticket",
				})
			}
		}
	}

	// Pattern 2: Component mentions
	for component, paths := range n.componentMappings {
		if strings.Contains(strings.ToLower(text), strings.ToLower(component)) {
			for _, pathPattern := range paths {
				files = append(files, TargetFile{
					Path:       pathPattern,
					Confidence: 0.80,
					Reason:     fmt.Sprintf("Mapped from component: %s", component),
				})
			}
		}
	}

	// Deduplicate
	files = deduplicateFiles(files)

	return files
}

// inferEvidence extracts evidence from ticket description
func (n *Normalizer) inferEvidence(description string) Evidence {
	evidence := Evidence{
		Type:       "unknown",
		Content:    "No explicit evidence found",
		Source:     "ticket_description",
		Confidence: 0.50,
	}

	descLower := strings.ToLower(description)

	// Check for error messages
	if strings.Contains(descLower, "error:") || 
	   strings.Contains(descLower, "failed") ||
	   strings.Contains(descLower, "exception") {
		evidence.Type = "error_message"
		evidence.Content = "Error message found in ticket"
		evidence.Confidence = 0.85
	}

	// Check for logs
	if strings.Contains(descLower, "log") || 
	   strings.Contains(descLower, "output") {
		evidence.Type = "log_output"
		evidence.Content = "Log output found in ticket"
		evidence.Confidence = 0.80
	}

	// Check for repro steps
	if strings.Contains(descLower, "steps to reproduce") || 
	   strings.Contains(descLower, "repro:") ||
	   strings.Contains(descLower, "how to reproduce") {
		evidence.Type = "repro_steps"
		evidence.Content = "Reproduction steps provided"
		evidence.Confidence = 0.90
	}

	return evidence
}

// inferAcceptanceCriteria extracts acceptance criteria from ticket
func (n *Normalizer) inferAcceptanceCriteria(description string) []AcceptanceCriterion {
	var criteria []AcceptanceCriterion

	descLower := strings.ToLower(description)

	// Check if acceptance criteria already exists
	if strings.Contains(descLower, "acceptance criteria:") ||
	   strings.Contains(descLower, "acceptance:") ||
	   strings.Contains(descLower, "validation:") {
		
		criteria = append(criteria, AcceptanceCriterion{
			Description:    "Existing acceptance criteria found in ticket",
			Confidence:     0.85,
		})
	} else {
		// Generate default acceptance criteria
		criteria = append(criteria, AcceptanceCriterion{
			Description:    "File is modified and passes validation",
			ValidationCmd:  "go build ./...",
			Confidence:     0.70,
		})
	}

	return criteria
}

// inferValidation suggests validation commands
func (n *Normalizer) inferValidation(targetFiles []TargetFile) Validation {
	validation := Validation{
		Confidence: 0.75,
	}

	// Check file types
	hasGo := false
	hasDocs := false
	hasConfig := false

	for _, f := range targetFiles {
		if strings.HasSuffix(f.Path, ".go") {
			hasGo = true
		}
		if strings.HasSuffix(f.Path, ".md") {
			hasDocs = true
		}
		if strings.HasSuffix(f.Path, ".yaml") || strings.HasSuffix(f.Path, ".yml") {
			hasConfig = true
		}
	}

	if hasGo {
		validation.BuildCmd = "go build ./... && go test ./..."
		validation.QualityGates = []string{"go vet", "go fmt"}
		validation.Confidence = 0.90
	}

	if hasDocs {
		validation.QualityGates = append(validation.QualityGates, "markdown lint")
	}

	if hasConfig {
		validation.QualityGates = append(validation.QualityGates, "yaml validation")
	}

	return validation
}

// classifyScope determines blast radius and execution class
func (n *Normalizer) classifyScope(targetFiles []TargetFile) Scope {
	scope := Scope{
		BlastRadius:    "low",
		ExecutionClass: "unknown",
		Bounded:        false,
	}

	if len(targetFiles) == 0 {
		scope.BlastRadius = "unknown"
		return scope
	}

	// Count file types
	docsCount := 0
	configCount := 0
	codeCount := 0

	for _, f := range targetFiles {
		ext := strings.ToLower(f.Path)
		if strings.Contains(ext, "docs/") || strings.HasSuffix(ext, ".md") {
			docsCount++
		} else if strings.HasSuffix(ext, ".yaml") || strings.HasSuffix(ext, ".yml") || strings.HasSuffix(ext, ".json") {
			configCount++
		} else if strings.HasSuffix(ext, ".go") {
			codeCount++
		}
	}

	// Classify
	if docsCount > 0 && configCount == 0 && codeCount == 0 {
		scope.ExecutionClass = "docs"
		scope.BlastRadius = "low"
	} else if configCount > 0 && codeCount == 0 {
		scope.ExecutionClass = "config"
		scope.BlastRadius = "low"
	} else if len(targetFiles) == 1 {
		scope.ExecutionClass = "single_file"
		scope.BlastRadius = "low"
	} else if len(targetFiles) <= 3 {
		scope.ExecutionClass = "bounded_code"
		scope.BlastRadius = "medium"
	} else {
		scope.ExecutionClass = "broad_refactor"
		scope.BlastRadius = "high"
	}

	return scope
}

// calculateOverallConfidence computes overall packet confidence
func (n *Normalizer) calculateOverallConfidence(packet *ExecutionPacket) float64 {
	// Weighted average of component confidences
	weights := map[string]float64{
		"target_files": 0.40,
		"evidence":     0.25,
		"acceptance":   0.25,
		"validation":   0.10,
	}

	// Target files confidence
	targetConf := 0.0
	if len(packet.TargetFiles) > 0 {
		for _, f := range packet.TargetFiles {
			targetConf += f.Confidence
		}
		targetConf /= float64(len(packet.TargetFiles))
	}

	// Acceptance criteria confidence
	acceptConf := 0.0
	if len(packet.Acceptance) > 0 {
		for _, a := range packet.Acceptance {
			acceptConf += a.Confidence
		}
		acceptConf /= float64(len(packet.Acceptance))
	}

	overall := weights["target_files"]*targetConf +
		weights["evidence"]*packet.Evidence.Confidence +
		weights["acceptance"]*acceptConf +
		weights["validation"]*packet.Validation.Confidence

	return overall
}

// deduplicateFiles removes duplicate file paths
func deduplicateFiles(files []TargetFile) []TargetFile {
	seen := make(map[string]bool)
	var result []TargetFile

	for _, f := range files {
		if !seen[f.Path] {
			seen[f.Path] = true
			result = append(result, f)
		}
	}

	return result
}

// ToYAML converts packet to YAML string
func (p *ExecutionPacket) ToYAML() string {
	// Simple YAML serialization (in production, use yaml.v3)
	var sb strings.Builder
	
	sb.WriteString("BOUNDED_EXECUTION:\n")
	sb.WriteString(fmt.Sprintf("  version: %q\n", p.Version))
	sb.WriteString("  target_files:\n")
	
	for _, f := range p.TargetFiles {
		sb.WriteString(fmt.Sprintf("    - path: %s\n", f.Path))
		sb.WriteString(fmt.Sprintf("      confidence: %.2f\n", f.Confidence))
		sb.WriteString(fmt.Sprintf("      reason: %q\n", f.Reason))
	}
	
	sb.WriteString(fmt.Sprintf("  scope:\n"))
	sb.WriteString(fmt.Sprintf("    blast_radius: %s\n", p.Scope.BlastRadius))
	sb.WriteString(fmt.Sprintf("    execution_class: %s\n", p.Scope.ExecutionClass))
	sb.WriteString(fmt.Sprintf("    bounded: %v\n", p.Scope.Bounded))
	
	sb.WriteString(fmt.Sprintf("  inference_metadata:\n"))
	sb.WriteString(fmt.Sprintf("    overall_confidence: %.2f\n", p.Metadata.OverallConfidence))
	sb.WriteString(fmt.Sprintf("    generated_by: %s\n", p.Metadata.GeneratedBy))
	
	return sb.String()
}

// ToJSON converts packet to JSON string
func (p *ExecutionPacket) ToJSON() (string, error) {
	bytes, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

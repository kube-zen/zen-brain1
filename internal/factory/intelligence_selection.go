package factory

import (
/github.com/kube-zen/zen-brain1/internal/intelligence

	"context"
	"log"
	"strings"
)

// selectionResult holds the result of chooseTemplateAndConfig for use in createExecutionPlan.
type selectionResult struct {
	workType       string
	workDomain     string
	timeoutSeconds int64
	maxRetries     int
	source         string
	confidence     float64
	reasoning      string
	templateIdentity string // for proof-of-work: e.g. "implementation:real" or "default"
}

// chooseTemplateAndConfig selects template and configuration using the recommender when available,
// and persists intelligence metadata onto the spec.
func (f *FactoryImpl) chooseTemplateAndConfig(ctx context.Context, spec *FactoryTaskSpec) selectionResult {
	workType := string(spec.WorkType)
	workDomain := string(spec.WorkDomain)
	timeoutSeconds := spec.TimeoutSeconds
	maxRetries := spec.MaxRetries
	source := "static"
	confidence := 0.0
	reasoning := "Static selection (no recommender or fallback)"
	templateIdentity := "default"

	if f.recommender == nil {
		// Static: when domain empty prefer (workType, "real"); else exact (workType has this domain); else (workType, ""); else default
		var selWorkType, selWorkDomain string
		if workDomain == "" && f.templateManager.HasExactTemplate(workType, "real") {
			selWorkType, selWorkDomain = workType, "real"
			templateIdentity = workType + ":real"
		} else if f.templateManager.HasExactTemplate(workType, workDomain) {
			selWorkType, selWorkDomain = workType, workDomain
			templateIdentity = workType + ":" + workDomain
			if workDomain == "" {
				templateIdentity = workType
			}
		} else if f.templateManager.HasExactTemplate(workType, "") {
			selWorkType, selWorkDomain = workType, ""
			templateIdentity = workType
		} else {
			selWorkType, selWorkDomain = "default", ""
			templateIdentity = "default"
		}
		workType = selWorkType
		workDomain = selWorkDomain
		spec.SelectedTemplate = templateIdentity
		spec.TemplateKey = templateIdentity
		spec.SelectionSource = source
		spec.SelectionConfidence = confidence
		spec.SelectionReasoning = reasoning
		return selectionResult{
			workType:         workType,
			workDomain:       workDomain,
			timeoutSeconds:   timeoutSeconds,
			maxRetries:       maxRetries,
			source:           source,
			confidence:       confidence,
			reasoning:        reasoning,
			templateIdentity: templateIdentity,
		}
	}

	// Recommender exists: get template and config recommendations
	templateName, recSource, recConf, recReason, err := f.recommender.RecommendTemplateWithMetadata(ctx, spec.WorkType, spec.WorkDomain)
	if err != nil {
		log.Printf("[Factory] Recommender template error: %v; using static selection", err)
	} else {
		source = recSource
		confidence = recConf
		reasoning = recReason
		templateIdentity, workType, workDomain = f.interpretRecommendedTemplate(spec, templateName)
	}

	// Configuration recommendation
	if timeoutSeconds <= 0 || maxRetries <= 0 {
		to, ret, err := f.recommender.RecommendConfiguration(ctx, spec.WorkType, spec.WorkDomain)
		if err == nil {
			if timeoutSeconds <= 0 && to > 0 {
				timeoutSeconds = to
			}
			if maxRetries <= 0 && ret > 0 {
				maxRetries = ret
			}
		}
	}

	spec.SelectedTemplate = templateIdentity
	spec.TemplateKey = templateIdentity
	spec.SelectionSource = source
	spec.SelectionConfidence = confidence
	spec.SelectionReasoning = reasoning
	if spec.TimeoutSeconds <= 0 && timeoutSeconds > 0 {
		spec.TimeoutSeconds = timeoutSeconds
	}
	if spec.MaxRetries <= 0 && maxRetries > 0 {
		spec.MaxRetries = maxRetries
	}

	return selectionResult{
		workType:         workType,
		workDomain:       workDomain,
		timeoutSeconds:   timeoutSeconds,
		maxRetries:       maxRetries,
		source:           source,
		confidence:       confidence,
		reasoning:        reasoning,
		templateIdentity: templateIdentity,
	}
}

// interpretRecommendedTemplate parses recommended template name and validates against spec and registry.
// Supports: "default", "workType:workDomain", "workType/workDomain".
// Returns (templateIdentity, workType, workDomain) to use for GetTemplate; falls back to spec when invalid.
func (f *FactoryImpl) interpretRecommendedTemplate(spec *FactoryTaskSpec, templateName string) (identity, workType, workDomain string) {
	workType = string(spec.WorkType)
	workDomain = string(spec.WorkDomain)
	identity = templateName

	if templateName == "" || templateName == "default" {
		return "default", workType, workDomain
	}

	// Parse "workType:workDomain" or "workType/workDomain"
	var recWorkType, recWorkDomain string
	if idx := strings.Index(templateName, ":"); idx >= 0 {
		recWorkType = strings.TrimSpace(templateName[:idx])
		recWorkDomain = strings.TrimSpace(templateName[idx+1:])
	} else if idx := strings.Index(templateName, "/"); idx >= 0 {
		recWorkType = strings.TrimSpace(templateName[:idx])
		recWorkDomain = strings.TrimSpace(templateName[idx+1:])
	} else {
		recWorkType = templateName
	}

	// Only apply if it matches requested work type and registry has it
	if recWorkType != "" && recWorkType != string(spec.WorkType) {
		log.Printf("[Factory] Intelligence recommended template %s does not match work type %s; using static selection", templateName, spec.WorkType)
		return "default", workType, workDomain
	}
	if recWorkType != "" {
		workType = recWorkType
	}
	if recWorkDomain != "" {
		workDomain = recWorkDomain
	}

	_, err := f.templateManager.GetTemplate(workType, workDomain)
	if err != nil {
		log.Printf("[Factory] Registry has no template for recommended %s/%s: %v; using static selection", workType, workDomain, err)
		return "default", string(spec.WorkType), string(spec.WorkDomain)
	}

	if identity == "" {
		identity = workType + ":" + workDomain
		if workDomain == "" {
			identity = workType
		}
	}
	return identity, workType, workDomain
}

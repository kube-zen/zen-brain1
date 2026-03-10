// Package policyadapter converts BrainPolicy CRD resources into canonical policy.PolicyRule objects.
package policyadapter

import (
	"fmt"
	"strings"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/pkg/policy"
)

// ValidateBrainPolicySpec validates the BrainPolicy spec: unique rule names, Action required, MaxCostUSD >= 0,
// and when RequiresApproval is true with no per-rule approval level, DefaultApprovalLevel is used (valid if set or not required).
func ValidateBrainPolicySpec(spec *v1alpha1.BrainPolicySpec) error {
	if spec == nil {
		return fmt.Errorf("BrainPolicySpec is nil")
	}
	seen := make(map[string]bool)
	for i, r := range spec.Rules {
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("rules[%d]: name is required", i)
		}
		if seen[r.Name] {
			return fmt.Errorf("rules: duplicate rule name %q", r.Name)
		}
		seen[r.Name] = true
		if strings.TrimSpace(r.Action) == "" {
			return fmt.Errorf("rules[%d]: action is required", i)
		}
		if r.MaxCostUSD < 0 {
			return fmt.Errorf("rules[%d]: maxCostUSD cannot be negative", i)
		}
	}
	return nil
}

// ConvertBrainPolicyRule converts a single PolicyRuleSpec to a canonical PolicyRule.
// defaultApprovalLevel is used when RequiresApproval is true and no per-rule approval level is set (PolicyRuleSpec has no approval level field; use spec DefaultApprovalLevel when converting the whole policy).
func ConvertBrainPolicyRule(rule v1alpha1.PolicyRuleSpec, defaultApprovalLevel string) (policy.PolicyRule, error) {
	if strings.TrimSpace(rule.Name) == "" {
		return policy.PolicyRule{}, fmt.Errorf("rule name is required")
	}
	if strings.TrimSpace(rule.Action) == "" {
		return policy.PolicyRule{}, fmt.Errorf("rule action is required")
	}
	if rule.MaxCostUSD < 0 {
		return policy.PolicyRule{}, fmt.Errorf("maxCostUSD cannot be negative")
	}

	out := policy.PolicyRule{
		Name:        strings.TrimSpace(rule.Name),
		Description: fmt.Sprintf("BrainPolicy rule: %s", rule.Action),
		Version:     "v1",
		Priority:    0,
		Effect:      "allow",
		Conditions:  nil,
		Obligations: nil,
		Metadata:    map[string]interface{}{"action": rule.Action},
	}

	// Action: encode as condition so the engine can match requests by action.
	out.Conditions = append(out.Conditions, policy.Condition{
		Field:    "action",
		Operator: "equals",
		Value:    rule.Action,
	})

	// RequiresApproval -> effect + obligation
	if rule.RequiresApproval {
		out.Effect = "require_approval"
		level := strings.TrimSpace(defaultApprovalLevel)
		if level == "" {
			level = "default"
		}
		out.Obligations = append(out.Obligations, policy.Obligation{
			Type: "require_approval",
			Parameters: map[string]interface{}{
				"approval_level": level,
			},
		})
	}

	// MaxCostUSD > 0 -> condition on cost (context/resource cost field)
	if rule.MaxCostUSD > 0 {
		out.Conditions = append(out.Conditions, policy.Condition{
			Field:    "resource.attributes.estimated_cost_usd",
			Operator: "lte",
			Value:    rule.MaxCostUSD,
		})
	}

	// AllowedModels non-empty -> condition on model ID in request context
	if len(rule.AllowedModels) > 0 {
		out.Conditions = append(out.Conditions, policy.Condition{
			Field:    "context.data.model_id",
			Operator: "in",
			Value:    rule.AllowedModels,
		})
	}

	return out, nil
}

// ConvertBrainPolicy converts a BrainPolicy into a slice of canonical PolicyRules.
// ValidateBrainPolicySpec is not called here; call it first if you need validation.
func ConvertBrainPolicy(bp *v1alpha1.BrainPolicy) ([]policy.PolicyRule, error) {
	if bp == nil {
		return nil, fmt.Errorf("BrainPolicy is nil")
	}
	spec := &bp.Spec
	defaultApproval := strings.TrimSpace(spec.DefaultApprovalLevel)
	if defaultApproval == "" {
		defaultApproval = "default"
	}
	var rules []policy.PolicyRule
	for i := range spec.Rules {
		rule, err := ConvertBrainPolicyRule(spec.Rules[i], defaultApproval)
		if err != nil {
			return nil, fmt.Errorf("rules[%d] %q: %w", i, spec.Rules[i].Name, err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

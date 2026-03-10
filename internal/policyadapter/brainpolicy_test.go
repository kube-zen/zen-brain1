package policyadapter

import (
	"testing"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateBrainPolicySpec(t *testing.T) {
	if err := ValidateBrainPolicySpec(nil); err == nil {
		t.Error("expected error for nil spec")
	}
	if err := ValidateBrainPolicySpec(&v1alpha1.BrainPolicySpec{}); err != nil {
		t.Errorf("empty spec valid: %v", err)
	}
	// duplicate names
	if err := ValidateBrainPolicySpec(&v1alpha1.BrainPolicySpec{
		Rules: []v1alpha1.PolicyRuleSpec{
			{Name: "r1", Action: "execute_task"},
			{Name: "r1", Action: "call_llm"},
		},
	}); err == nil {
		t.Error("expected error for duplicate rule names")
	}
	// empty action
	if err := ValidateBrainPolicySpec(&v1alpha1.BrainPolicySpec{
		Rules: []v1alpha1.PolicyRuleSpec{{Name: "r1", Action: ""}},
	}); err == nil {
		t.Error("expected error for empty action")
	}
	// negative MaxCostUSD
	if err := ValidateBrainPolicySpec(&v1alpha1.BrainPolicySpec{
		Rules: []v1alpha1.PolicyRuleSpec{{Name: "r1", Action: "a", MaxCostUSD: -1}},
	}); err == nil {
		t.Error("expected error for negative MaxCostUSD")
	}
}

func TestConvertBrainPolicyRule_RequiresApproval(t *testing.T) {
	rule, err := ConvertBrainPolicyRule(v1alpha1.PolicyRuleSpec{
		Name:             "approve-high-cost",
		Action:           "execute_task",
		RequiresApproval: true,
	}, "team_lead")
	if err != nil {
		t.Fatalf("ConvertBrainPolicyRule: %v", err)
	}
	if rule.Effect != "require_approval" {
		t.Errorf("Effect: got %q", rule.Effect)
	}
	var found bool
	for _, o := range rule.Obligations {
		if o.Type == "require_approval" {
			found = true
			if o.Parameters["approval_level"] != "team_lead" {
				t.Errorf("approval_level: got %v", o.Parameters["approval_level"])
			}
			break
		}
	}
	if !found {
		t.Error("require_approval obligation not found")
	}
	// action condition
	var actionCond bool
	for _, c := range rule.Conditions {
		if c.Field == "action" && c.Value == "execute_task" {
			actionCond = true
			break
		}
	}
	if !actionCond {
		t.Error("action condition not found")
	}
}

func TestConvertBrainPolicyRule_AllowedModels(t *testing.T) {
	rule, err := ConvertBrainPolicyRule(v1alpha1.PolicyRuleSpec{
		Name:          "models",
		Action:        "call_llm",
		AllowedModels: []string{"gpt-4", "claude-3"},
	}, "")
	if err != nil {
		t.Fatalf("ConvertBrainPolicyRule: %v", err)
	}
	var found bool
	for _, c := range rule.Conditions {
		if c.Field == "context.data.model_id" && c.Operator == "in" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AllowedModels condition not found")
	}
}

func TestConvertBrainPolicyRule_MaxCostUSD(t *testing.T) {
	rule, err := ConvertBrainPolicyRule(v1alpha1.PolicyRuleSpec{
		Name:       "cost-cap",
		Action:     "execute_task",
		MaxCostUSD: 10.5,
	}, "")
	if err != nil {
		t.Fatalf("ConvertBrainPolicyRule: %v", err)
	}
	var found bool
	for _, c := range rule.Conditions {
		if c.Field == "resource.attributes.estimated_cost_usd" && c.Operator == "lte" {
			found = true
			if c.Value != 10.5 {
				t.Errorf("cost value: got %v", c.Value)
			}
			break
		}
	}
	if !found {
		t.Error("MaxCostUSD condition not found")
	}
}

func TestConvertBrainPolicy(t *testing.T) {
	bp := &v1alpha1.BrainPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1alpha1.BrainPolicySpec{
			DefaultApprovalLevel: "manager",
			Rules: []v1alpha1.PolicyRuleSpec{
				{Name: "r1", Action: "execute_task", RequiresApproval: true},
			},
		},
	}
	rules, err := ConvertBrainPolicy(bp)
	if err != nil {
		t.Fatalf("ConvertBrainPolicy: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Obligations[0].Parameters["approval_level"] != "manager" {
		t.Errorf("approval_level: got %v", rules[0].Obligations[0].Parameters["approval_level"])
	}
}

func TestConvertBrainPolicy_nil(t *testing.T) {
	_, err := ConvertBrainPolicy(nil)
	if err == nil {
		t.Error("expected error for nil BrainPolicy")
	}
}

func TestConvertBrainPolicyRule_invalid(t *testing.T) {
	_, err := ConvertBrainPolicyRule(v1alpha1.PolicyRuleSpec{Name: "x", Action: ""}, "")
	if err == nil {
		t.Error("expected error for empty action")
	}
	_, err = ConvertBrainPolicyRule(v1alpha1.PolicyRuleSpec{Name: "x", Action: "a", MaxCostUSD: -1}, "")
	if err == nil {
		t.Error("expected error for negative MaxCostUSD")
	}
}

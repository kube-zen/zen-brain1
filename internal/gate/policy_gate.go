// Package gate provides a ZenGate implementation that enforces BrainPolicy (Block 4.6).
// When no BrainPolicies exist, admission is allowed (permissive). When policies exist,
// rules are enforced: cost and model conditions can deny admission.
package gate

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	gatepkg "github.com/kube-zen/zen-brain1/pkg/gate"
	"github.com/kube-zen/zen-brain1/pkg/policy"
	"github.com/kube-zen/zen-brain1/internal/policyadapter"
)

// PolicyGate implements ZenGate by loading BrainPolicies from the cluster and
// evaluating admission requests against their rules. No policies → allow. Policies
// present → enforce (non-permissive).
type PolicyGate struct {
	client client.Client
}

// NewPolicyGate returns a ZenGate that enforces BrainPolicy CRDs. Requires a non-nil Client.
func NewPolicyGate(c client.Client) gatepkg.ZenGate {
	if c == nil {
		return NewStubGate()
	}
	return &PolicyGate{client: c}
}

// Admit evaluates the request against all BrainPolicies. When no policies exist, allows.
// When policies exist, denies if any matching rule's conditions fail.
func (g *PolicyGate) Admit(ctx context.Context, req gatepkg.AdmissionRequest) (*gatepkg.AdmissionResponse, error) {
	start := time.Now()
	resp := &gatepkg.AdmissionResponse{
		RequestID:   req.RequestID,
		Allowed:     true,
		EvaluatedAt: start,
	}

	var list v1alpha1.BrainPolicyList
	if err := g.client.List(ctx, &list); err != nil {
		resp.Allowed = false
		resp.Reason = fmt.Sprintf("policy list failed: %v", err)
		resp.EvaluationDuration = time.Since(start)
		return resp, nil
	}

	// No policies → permissive (allow)
	if len(list.Items) == 0 {
		resp.EvaluationDuration = time.Since(start)
		return resp, nil
	}

	// Collect all rules that apply to this action
	var rules []policy.PolicyRule
	for i := range list.Items {
		bp := &list.Items[i]
		if err := policyadapter.ValidateBrainPolicySpec(&bp.Spec); err != nil {
			continue
		}
		converted, err := policyadapter.ConvertBrainPolicy(bp)
		if err != nil {
			continue
		}
		for _, r := range converted {
			if ruleMatchesAction(r, req.Action) {
				rules = append(rules, r)
			}
		}
	}

	// No rules for this action → allow
	if len(rules) == 0 {
		resp.EvaluationDuration = time.Since(start)
		return resp, nil
	}

	// Evaluate each rule's conditions; deny if any fail
	for _, rule := range rules {
		if reason := evaluateRule(rule, req); reason != "" {
			resp.Allowed = false
			resp.Reason = reason
			resp.EvaluationDuration = time.Since(start)
			return resp, nil
		}
	}

	resp.EvaluationDuration = time.Since(start)
	return resp, nil
}

func ruleMatchesAction(r policy.PolicyRule, action policy.Action) bool {
	if a, ok := r.Metadata["action"]; ok {
		return a == string(action)
	}
	for _, c := range r.Conditions {
		if c.Field == "action" && c.Operator == "equals" {
			return c.Value == string(action)
		}
	}
	return false
}

// evaluateRule returns a non-empty reason string if the rule denies the request.
func evaluateRule(rule policy.PolicyRule, req gatepkg.AdmissionRequest) string {
	for _, cond := range rule.Conditions {
		if cond.Field == "action" {
			continue
		}
		reqVal := getRequestValue(req, cond.Field)
		if !conditionPasses(cond, reqVal) {
			return fmt.Sprintf("policy rule %q: %s %s %v (got %v)", rule.Name, cond.Field, cond.Operator, cond.Value, reqVal)
		}
	}
	return ""
}

func getRequestValue(req gatepkg.AdmissionRequest, field string) interface{} {
	// resource.attributes.estimated_cost_usd
	if strings.HasPrefix(field, "resource.attributes.") {
		key := strings.TrimPrefix(field, "resource.attributes.")
		if req.Resource.Attributes != nil {
			if v, ok := req.Resource.Attributes[key]; ok {
				return v
			}
		}
		return nil
	}
	// context.data.model_id
	if strings.HasPrefix(field, "context.data.") {
		key := strings.TrimPrefix(field, "context.data.")
		if req.Payload != nil {
			if v, ok := req.Payload[key]; ok {
				return v
			}
		}
		return nil
	}
	return nil
}

func conditionPasses(c policy.Condition, reqVal interface{}) bool {
	switch c.Operator {
	case "equals":
		return reflect.DeepEqual(reqVal, c.Value)
	case "lte":
		return numericCompare(reqVal, c.Value) <= 0
	case "in":
		return sliceContains(c.Value, reqVal)
	default:
		return true
	}
}

func numericCompare(a, b interface{}) int {
	af := toFloat(a)
	bf := toFloat(b)
	if af < bf {
		return -1
	}
	if af > bf {
		return 1
	}
	return 0
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return 0
	}
}

func sliceContains(slice, elem interface{}) bool {
	if slice == nil {
		return true
	}
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Slice {
		return false
	}
	for i := 0; i < rv.Len(); i++ {
		if reflect.DeepEqual(rv.Index(i).Interface(), elem) {
			return true
		}
	}
	// When request has no value (e.g. no model_id on task), treat as "no constraint" so we don't deny.
	if elem == nil || reflect.ValueOf(elem).IsZero() {
		return true
	}
	return false
}

// Validate runs the same policy check as Admit and returns validation errors if denied.
func (g *PolicyGate) Validate(ctx context.Context, req gatepkg.AdmissionRequest) ([]gatepkg.ValidationError, error) {
	resp, err := g.Admit(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp != nil && !resp.Allowed {
		return []gatepkg.ValidationError{{Field: "policy", Message: resp.Reason}}, nil
	}
	return nil, nil
}

// RegisterValidator is a no-op (policy is from CRDs).
func (g *PolicyGate) RegisterValidator(ctx context.Context, _ gatepkg.Validator) error {
	return nil
}

// RegisterPolicy is a no-op (policy is from CRDs).
func (g *PolicyGate) RegisterPolicy(ctx context.Context, _ policy.ZenPolicy) error {
	return nil
}

// Stats returns basic stats (policy count would require listing again).
func (g *PolicyGate) Stats(ctx context.Context) (map[string]interface{}, error) {
	var list v1alpha1.BrainPolicyList
	if err := g.client.List(ctx, &list); err != nil {
		return map[string]interface{}{"error": err.Error()}, nil
	}
	return map[string]interface{}{"brainpolicy_count": len(list.Items)}, nil
}

// Close is a no-op.
func (g *PolicyGate) Close() error {
	return nil
}

var _ gatepkg.ZenGate = (*PolicyGate)(nil)

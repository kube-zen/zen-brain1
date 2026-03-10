package gate

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	gatepkg "github.com/kube-zen/zen-brain1/pkg/gate"
	"github.com/kube-zen/zen-brain1/pkg/policy"
)

var testScheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(v1alpha1.AddToScheme(s))
	return s
}()

func TestPolicyGate_Admit_NoPolicies(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme).Build()
	g := NewPolicyGate(cl).(*PolicyGate)

	req := gatepkg.AdmissionRequest{
		RequestID: "req1",
		Action:    policy.ActionExecuteTask,
		Resource:  policy.Resource{Type: "task", ID: "t1", Attributes: map[string]interface{}{"estimated_cost_usd": 5.0}},
	}
	resp, err := g.Admit(context.Background(), req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if !resp.Allowed {
		t.Errorf("expected allowed when no policies, got denied: %s", resp.Reason)
	}
}

func TestPolicyGate_Admit_PolicyEnforced(t *testing.T) {
	bp := &v1alpha1.BrainPolicy{}
	bp.Name = "default"
	bp.Spec.Rules = []v1alpha1.PolicyRuleSpec{
		{Name: "cost-cap", Action: "execute_task", MaxCostUSD: 10},
	}
	cl := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(bp).Build()
	g := NewPolicyGate(cl).(*PolicyGate)

	// Under cap: allow
	req := gatepkg.AdmissionRequest{
		RequestID: "req1",
		Action:    policy.ActionExecuteTask,
		Resource:  policy.Resource{Type: "task", ID: "t1", Attributes: map[string]interface{}{"estimated_cost_usd": 5.0}},
	}
	resp, err := g.Admit(context.Background(), req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if !resp.Allowed {
		t.Errorf("expected allowed (under cap), got denied: %s", resp.Reason)
	}

	// Over cap: deny
	req2 := gatepkg.AdmissionRequest{
		RequestID: "req2",
		Action:    policy.ActionExecuteTask,
		Resource:  policy.Resource{Type: "task", ID: "t2", Attributes: map[string]interface{}{"estimated_cost_usd": 15.0}},
	}
	resp2, err := g.Admit(context.Background(), req2)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if resp2.Allowed {
		t.Error("expected denied when over maxCostUSD")
	}
	if resp2.Reason == "" {
		t.Error("expected reason when denied")
	}
}

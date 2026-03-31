package labels

import "testing"

func TestGetReportedToJira_NilLabels(t *testing.T) {
	result := GetReportedToJira(nil)
	if result {
		t.Error("expected false for nil labels")
	}
}

func TestGetReportedToJira_EmptyLabels(t *testing.T) {
	result := GetReportedToJira(map[string]string{})
	if result {
		t.Error("expected false for empty labels")
	}
}

func TestGetReportedToJira_NewKeyTrue(t *testing.T) {
	labels := map[string]string{
		LabelReportedToJira: "true",
	}
	if !GetReportedToJira(labels) {
		t.Error("expected true with new key set to true")
	}
}

func TestGetReportedToJira_NewKeyFalse(t *testing.T) {
	labels := map[string]string{
		LabelReportedToJira: "false",
	}
	if GetReportedToJira(labels) {
		t.Error("expected false with new key set to false")
	}
}

func TestGetReportedToJira_OldKeyTrue(t *testing.T) {
	labels := map[string]string{
		LabelReportedToJiraOld: "true",
	}
	if !GetReportedToJira(labels) {
		t.Error("expected true with old legacy key set to true (fallback)")
	}
}

func TestGetReportedToJira_OldKeyFalse(t *testing.T) {
	labels := map[string]string{
		LabelReportedToJiraOld: "false",
	}
	if GetReportedToJira(labels) {
		t.Error("expected false with old key set to false")
	}
}

func TestGetReportedToJira_BothPresent_NewWins(t *testing.T) {
	// New key says true, old key says false — new wins
	labels := map[string]string{
		LabelReportedToJira:    "true",
		LabelReportedToJiraOld: "false",
	}
	if !GetReportedToJira(labels) {
		t.Error("expected new key to take precedence (true)")
	}

	// New key says false, old key says true — new wins (even though old is true)
	labels = map[string]string{
		LabelReportedToJira:    "false",
		LabelReportedToJiraOld: "true",
	}
	if GetReportedToJira(labels) {
		t.Error("expected new key to take precedence (false)")
	}
}

func TestSetReportedToJira_NilLabels(t *testing.T) {
	var labels map[string]string
	labels = EnsureLabels(labels)
	SetReportedToJira(labels)
	if labels[LabelReportedToJira] != "true" {
		t.Error("expected new key to be set to true")
	}
	if _, ok := labels[LabelReportedToJiraOld]; ok {
		t.Error("old key should NOT be written by SetReportedToJira")
	}
}

func TestSetReportedToJira_ExistingLabels(t *testing.T) {
	labels := map[string]string{"other": "value"}
	SetReportedToJira(labels)
	if labels[LabelReportedToJira] != "true" {
		t.Error("expected new key to be set to true")
	}
	if _, ok := labels[LabelReportedToJiraOld]; ok {
		t.Error("old key should NOT be written by SetReportedToJira")
	}
	if labels["other"] != "value" {
		t.Error("existing labels should be preserved")
	}
}

func TestGetPlannedModel_NilAnnotations(t *testing.T) {
	result := GetPlannedModel(nil)
	if result != "" {
		t.Error("expected empty string for nil annotations")
	}
}

func TestGetPlannedModel_NewKey(t *testing.T) {
	annotations := map[string]string{
		AnnotationPlannedModel: "qwen3.5:0.8b",
	}
	result := GetPlannedModel(annotations)
	if result != "qwen3.5:0.8b" {
		t.Errorf("expected qwen3.5:0.8b, got %q", result)
	}
}

func TestGetPlannedModel_OldKeyFallback(t *testing.T) {
	annotations := map[string]string{
		AnnotationPlannedModelOld: "glm-4.7",
	}
	result := GetPlannedModel(annotations)
	if result != "glm-4.7" {
		t.Errorf("expected glm-4.7 from old key fallback, got %q", result)
	}
}

func TestGetPlannedModel_BothPresent_NewWins(t *testing.T) {
	annotations := map[string]string{
		AnnotationPlannedModel:    "new-model",
		AnnotationPlannedModelOld: "old-model",
	}
	result := GetPlannedModel(annotations)
	if result != "new-model" {
		t.Errorf("expected new-model (new key precedence), got %q", result)
	}
}

func TestGetPlannedModel_NeitherSet(t *testing.T) {
	annotations := map[string]string{"other": "value"}
	result := GetPlannedModel(annotations)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

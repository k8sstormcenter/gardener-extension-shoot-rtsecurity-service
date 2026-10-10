package v1alpha1

import "testing"

// The shoot declares its workload; a chart default naming one cluster's scale set is how a
// suite ends up aimed at the cluster it was authored against.
func TestTargetReachesTheChartValues(t *testing.T) {
	b, ns, pn, ss := true, "arc-systems", "arc-runner", "k3s-2"
	c := &SOCConfig{Target: &Target{
		Enabled: &b, Namespace: &ns, ProfileName: &pn, ScaleSetName: &ss,
		Containers: []string{"runner", "dind"}, InitContainers: []string{},
	}}
	got, _ := c.Values()["target"].(map[string]any)
	if got == nil {
		t.Fatal("no target section")
	}
	for k, want := range map[string]any{"enabled": true, "namespace": ns, "profileName": pn, "scaleSetName": ss} {
		if got[k] != want {
			t.Errorf("%s = %v, want %v", k, got[k], want)
		}
	}
	// The label and the pod-name prefix are different strings and neither may be derived
	// from the other.
	if got["profileName"] == got["scaleSetName"] {
		t.Error("profileName and scaleSetName collapsed into one value")
	}
	if cs, _ := got["containers"].([]string); len(cs) != 2 {
		t.Errorf("containers = %v", got["containers"])
	}
	// An explicitly empty init-container list must survive, or the chart default would
	// reintroduce containers whose profiles are not delivered.
	if ics, ok := got["initContainers"].([]string); !ok || len(ics) != 0 {
		t.Errorf("initContainers = %v, want an empty list that is present", got["initContainers"])
	}
}

// Nothing set means the chart stays the single owner of defaults.
func TestTargetAbsentSetsNothing(t *testing.T) {
	if _, ok := (&SOCConfig{}).Values()["target"]; ok {
		t.Error("an absent target must not write a target section")
	}
}

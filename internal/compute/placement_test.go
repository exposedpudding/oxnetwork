package compute

import (
	"slices"
	"testing"
)

func candidate(id string) Candidate {
	return Candidate{ID: id, State: "online", Freshness: "fresh", Architecture: "amd64", Capabilities: []string{"compute"}, Tags: []string{"trusted"}, TotalVCPU: 8, FreeVCPU: 6, TotalRAMGB: 32, FreeRAMGB: 24}
}

func TestPlacementFiltersAndExplains(t *testing.T) {
	stale := candidate("stale")
	stale.Freshness = "stale"
	drained := candidate("drained")
	drained.Drained = true
	arm := candidate("arm")
	arm.Architecture = "arm64"
	small := candidate("small")
	small.FreeVCPU = 1
	good := candidate("good")
	decision := Place(Request{VCPU: 2, RAMGB: 4, Architecture: "amd64", RequiredCapabilities: []string{"compute"}, RequiredTags: []string{"trusted"}, MinFreeVCPU: 1}, []Candidate{stale, drained, arm, small, good})
	if decision.SelectedWorkerID != "good" {
		t.Fatalf("selected %q", decision.SelectedWorkerID)
	}
	want := map[string]string{"stale": "not_fresh", "drained": "drained", "arm": "architecture_mismatch", "small": "insufficient_capacity"}
	for _, a := range decision.Assessments {
		if reason, ok := want[a.WorkerID]; ok && !slices.Contains(a.Reasons, reason) {
			t.Fatalf("%s missing %s: %+v", a.WorkerID, reason, a)
		}
	}
}

func TestPlacementScoreAndStableTieBreak(t *testing.T) {
	a := candidate("a")
	b := candidate("b")
	better := candidate("better")
	better.FreeVCPU = 8
	better.FreeRAMGB = 32
	req := Request{VCPU: 2, RAMGB: 4}
	if got := Place(req, []Candidate{b, a}).SelectedWorkerID; got != "a" {
		t.Fatalf("tie selected %q", got)
	}
	if got := Place(req, []Candidate{a, better, b}).SelectedWorkerID; got != "better" {
		t.Fatalf("score selected %q", got)
	}
}

func TestPlacementHardAffinityExclusionAndNoCandidate(t *testing.T) {
	a := candidate("a")
	b := candidate("b")
	decision := Place(Request{AllowedWorkerIDs: []string{"b"}, ExcludedWorkerIDs: []string{"b"}}, []Candidate{a, b})
	if decision.SelectedWorkerID != "" {
		t.Fatalf("unexpected selected worker %q", decision.SelectedWorkerID)
	}
	if !slices.Contains(decision.Assessments[0].Reasons, "outside_affinity") || !slices.Contains(decision.Assessments[1].Reasons, "excluded") {
		t.Fatalf("unexpected reasons: %+v", decision.Assessments)
	}
}

func TestPlacementRejectsInvalidCapacityAndDuplicateID(t *testing.T) {
	a := candidate("a")
	a.FreeVCPU = 20
	decision := Place(Request{}, []Candidate{a, a})
	if decision.SelectedWorkerID != "" || !slices.Contains(decision.Assessments[0].Reasons, "invalid_capacity") || !slices.Contains(decision.Assessments[1].Reasons, "invalid_worker_id") {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestPlacementRequiresGPUAndPrefersLowerLoad(t *testing.T) {
	busy := candidate("busy")
	idle := candidate("idle")
	busy.TotalGPU, busy.FreeGPU = 1, 1
	idle.TotalGPU, idle.FreeGPU = 1, 1
	busyLoad, idleLoad := 0.9, 0.1
	busy.Load, idle.Load = &busyLoad, &idleLoad
	decision := Place(Request{GPU: 1}, []Candidate{busy, idle})
	if decision.SelectedWorkerID != "idle" {
		t.Fatalf("selected %q", decision.SelectedWorkerID)
	}
	idle.FreeGPU = 0
	decision = Place(Request{GPU: 1}, []Candidate{busy, idle})
	if decision.SelectedWorkerID != "busy" || !slices.Contains(decision.Assessments[1].Reasons, "insufficient_capacity") {
		t.Fatalf("unexpected GPU decision: %+v", decision)
	}
}

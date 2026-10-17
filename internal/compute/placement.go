// Package compute provides deterministic, read-only placement decisions.
// A provider must confirm resource availability again before dispatching a job.
package compute

import (
	"math"
	"slices"
	"strings"
)

type Candidate struct {
	ID           string
	State        string // online, draining, offline
	Freshness    string // fresh, stale, unknown
	Drained      bool
	Architecture string
	Capabilities []string
	Tags         []string
	TotalVCPU    int
	FreeVCPU     int
	TotalRAMGB   float64
	FreeRAMGB    float64
	TotalGPU     int
	FreeGPU      int
	Load         *float64 // optional fraction in [0, 1]
}

type Request struct {
	VCPU                 int
	RAMGB                float64
	GPU                  int
	Architecture         string
	RequiredCapabilities []string
	RequiredTags         []string
	AllowedWorkerIDs     []string // hard affinity; empty means any worker
	ExcludedWorkerIDs    []string
	MinFreeVCPU          int // capacity to leave after placing the request
	MinFreeRAMGB         float64
}

type Assessment struct {
	WorkerID string   `json:"worker_id"`
	Eligible bool     `json:"eligible"`
	Score    float64  `json:"score,omitempty"`
	Reasons  []string `json:"reasons"`
}

type Decision struct {
	SelectedWorkerID string       `json:"selected_worker_id,omitempty"`
	Assessments      []Assessment `json:"assessments"`
}

// Place evaluates each candidate in worker ID order. Scores use the mean
// remaining free CPU and RAM ratios, an optional GPU ratio, and an optional
// load score. It never dispatches a job or reserves capacity.
func Place(req Request, candidates []Candidate) Decision {
	ordered := slices.Clone(candidates)
	slices.SortFunc(ordered, func(a, b Candidate) int { return strings.Compare(a.ID, b.ID) })
	decision := Decision{Assessments: make([]Assessment, 0, len(ordered))}
	bestScore := -1.0
	seen := map[string]bool{}
	for _, worker := range ordered {
		reasons := []string{}
		if worker.ID == "" || seen[worker.ID] {
			reasons = append(reasons, "invalid_worker_id")
		}
		seen[worker.ID] = true
		if worker.State != "online" {
			reasons = append(reasons, "not_online")
		}
		if worker.Drained {
			reasons = append(reasons, "drained")
		}
		if worker.Freshness != "fresh" {
			reasons = append(reasons, "not_fresh")
		}
		if req.Architecture != "" && worker.Architecture != req.Architecture {
			reasons = append(reasons, "architecture_mismatch")
		}
		if len(req.AllowedWorkerIDs) > 0 && !slices.Contains(req.AllowedWorkerIDs, worker.ID) {
			reasons = append(reasons, "outside_affinity")
		}
		if slices.Contains(req.ExcludedWorkerIDs, worker.ID) {
			reasons = append(reasons, "excluded")
		}
		for _, capability := range req.RequiredCapabilities {
			if !slices.Contains(worker.Capabilities, capability) {
				reasons = append(reasons, "missing_capability:"+capability)
			}
		}
		for _, tag := range req.RequiredTags {
			if !slices.Contains(worker.Tags, tag) {
				reasons = append(reasons, "missing_tag:"+tag)
			}
		}
		if req.VCPU < 0 || req.GPU < 0 || req.RAMGB < 0 || req.MinFreeVCPU < 0 || req.MinFreeRAMGB < 0 || math.IsNaN(req.RAMGB) || math.IsNaN(req.MinFreeRAMGB) || math.IsInf(req.RAMGB, 0) || math.IsInf(req.MinFreeRAMGB, 0) {
			reasons = append(reasons, "invalid_request")
		}
		if worker.TotalVCPU <= 0 || worker.TotalRAMGB <= 0 || worker.TotalGPU < 0 || worker.FreeVCPU < 0 || worker.FreeRAMGB < 0 || worker.FreeGPU < 0 || worker.FreeVCPU > worker.TotalVCPU || worker.FreeRAMGB > worker.TotalRAMGB || worker.FreeGPU > worker.TotalGPU || math.IsNaN(worker.TotalRAMGB) || math.IsNaN(worker.FreeRAMGB) || math.IsInf(worker.TotalRAMGB, 0) || math.IsInf(worker.FreeRAMGB, 0) {
			reasons = append(reasons, "invalid_capacity")
		}
		if worker.Load != nil && (math.IsNaN(*worker.Load) || *worker.Load < 0 || *worker.Load > 1) {
			reasons = append(reasons, "invalid_load")
		}
		if worker.FreeVCPU < req.VCPU || worker.FreeVCPU-req.VCPU < req.MinFreeVCPU || worker.FreeRAMGB < req.RAMGB || worker.FreeRAMGB-req.RAMGB < req.MinFreeRAMGB || worker.FreeGPU < req.GPU {
			reasons = append(reasons, "insufficient_capacity")
		}
		assessment := Assessment{WorkerID: worker.ID, Eligible: len(reasons) == 0, Reasons: reasons}
		if assessment.Eligible {
			parts := []float64{float64(worker.FreeVCPU-req.VCPU) / float64(worker.TotalVCPU), (worker.FreeRAMGB - req.RAMGB) / worker.TotalRAMGB}
			if req.GPU > 0 {
				parts = append(parts, float64(worker.FreeGPU-req.GPU)/float64(worker.TotalGPU))
			}
			if worker.Load != nil {
				parts = append(parts, 1-*worker.Load)
			}
			for _, part := range parts {
				assessment.Score += part
			}
			assessment.Score /= float64(len(parts))
			if assessment.Score > bestScore {
				bestScore = assessment.Score
				decision.SelectedWorkerID = worker.ID
			}
		}
		decision.Assessments = append(decision.Assessments, assessment)
	}
	return decision
}

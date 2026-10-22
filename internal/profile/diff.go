package profile

import (
	"errors"
	"math"
	"sort"

	pprof "github.com/google/pprof/profile"
	"github.com/kyyril/perfguard/internal/model"
)

// DiffOptions specifies parameters for filtering and ranking regression contributor candidates.
type DiffOptions struct {
	MinShare float64 // Minimum share of total base metric required (in percent, e.g. 1.0 for 1%)
	Top      int     // Maximum number of candidates to return (0 means unlimited)
}

// Diff compares base and head profiles, returning ranked contributor candidates.
func Diff(baseProf, headProf *pprof.Profile, sampleIdx int, totalOpsBase, totalOpsHead int64, opts DiffOptions) ([]model.Candidate, error) {
	if baseProf == nil || headProf == nil {
		return nil, errors.New("profile: both base and head profiles are required")
	}

	baseFns, err := ExtractFunctions(baseProf, sampleIdx, totalOpsBase)
	if err != nil {
		return nil, err
	}

	headFns, err := ExtractFunctions(headProf, sampleIdx, totalOpsHead)
	if err != nil {
		return nil, err
	}

	return DiffExtracted(baseFns, headFns, opts), nil
}

// DiffExtracted computes deltas between already extracted base and head function values.
func DiffExtracted(baseFns, headFns map[string]FunctionValue, opts DiffOptions) []model.Candidate {
	// Calculate total base flat value to evaluate min_share threshold
	var totalBaseFlat float64
	for _, fv := range baseFns {
		totalBaseFlat += fv.Flat
	}

	threshold := 0.0
	if opts.MinShare > 0 && totalBaseFlat > 0 {
		threshold = (opts.MinShare / 100.0) * totalBaseFlat
	}

	// Gather all function names across both sides
	allNames := make(map[string]struct{})
	for name := range baseFns {
		allNames[name] = struct{}{}
	}
	for name := range headFns {
		allNames[name] = struct{}{}
	}

	candidates := make([]model.Candidate, 0, len(allNames))

	for name := range allNames {
		b := baseFns[name]
		h := headFns[name]

		flatDelta := h.Flat - b.Flat
		cumDelta := h.Cum - b.Cum

		// If there is no change at all, omit
		if flatDelta == 0 && cumDelta == 0 {
			continue
		}

		// Filter out candidates where neither flat nor cum delta meets the min_share threshold
		if math.Abs(flatDelta) < threshold && math.Abs(cumDelta) < threshold {
			continue
		}

		candidates = append(candidates, model.Candidate{
			Function:  name,
			FlatDelta: flatDelta,
			CumDelta:  cumDelta,
			Touched:   false,
		})
	}

	// Sort candidates:
	// 1. Absolute flat delta descending
	// 2. Absolute cum delta descending
	// 3. Function name ascending (deterministic tie-breaker)
	sort.Slice(candidates, func(i, j int) bool {
		absFlatI := math.Abs(candidates[i].FlatDelta)
		absFlatJ := math.Abs(candidates[j].FlatDelta)
		if !almostEqualDelta(absFlatI, absFlatJ, 1e-12) {
			return absFlatI > absFlatJ
		}

		absCumI := math.Abs(candidates[i].CumDelta)
		absCumJ := math.Abs(candidates[j].CumDelta)
		if !almostEqualDelta(absCumI, absCumJ, 1e-12) {
			return absCumI > absCumJ
		}

		return candidates[i].Function < candidates[j].Function
	})

	if opts.Top > 0 && len(candidates) > opts.Top {
		candidates = candidates[:opts.Top]
	}

	return candidates
}

func almostEqualDelta(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

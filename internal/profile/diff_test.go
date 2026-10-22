package profile_test

import (
	"math"
	"testing"

	pprof "github.com/google/pprof/profile"
	"github.com/kyyril/perfguard/internal/profile"
)

func makeDiffSyntheticProfile(fns map[string]int64, totalOps int64) *pprof.Profile {
	sampleTypes := []*pprof.ValueType{{Type: "samples", Unit: "count"}}
	prof := &pprof.Profile{
		SampleType: sampleTypes,
		PeriodType: sampleTypes[0],
	}

	var fnID uint64 = 1
	var locID uint64 = 1

	for name, val := range fns {
		f := &pprof.Function{ID: fnID, Name: name, Filename: name + ".go"}
		loc := &pprof.Location{ID: locID, Line: []pprof.Line{{Function: f}}}
		fnID++
		locID++

		prof.Function = append(prof.Function, f)
		prof.Location = append(prof.Location, loc)
		prof.Sample = append(prof.Sample, &pprof.Sample{
			Location: []*pprof.Location{loc},
			Value:    []int64{val},
		})
	}
	_ = totalOps
	return prof
}

func TestDiff_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		baseFns       map[string]int64
		headFns       map[string]int64
		baseOps       int64
		headOps       int64
		opts          profile.DiffOptions
		expectedNames []string
		expectedFlatD []float64
	}{
		{
			name: "clear regression in worker",
			baseFns: map[string]int64{
				"main.fast": 100, // per op = 1.0
				"main.slow": 100, // per op = 1.0
			},
			headFns: map[string]int64{
				"main.fast": 100, // per op = 1.0 (delta = 0)
				"main.slow": 500, // per op = 5.0 (delta = +4.0)
			},
			baseOps:       100,
			headOps:       100,
			opts:          profile.DiffOptions{MinShare: 1.0, Top: 10},
			expectedNames: []string{"main.slow"},
			expectedFlatD: []float64{4.0},
		},
		{
			name: "candidate ranking order by absolute delta",
			baseFns: map[string]int64{
				"fnA": 100, // 1.0
				"fnB": 100, // 1.0
				"fnC": 100, // 1.0
			},
			headFns: map[string]int64{
				"fnA": 300, // 3.0 -> delta +2.0
				"fnB": 700, // 7.0 -> delta +6.0
				"fnC": 50,  // 0.5 -> delta -0.5
			},
			baseOps:       100,
			headOps:       100,
			opts:          profile.DiffOptions{MinShare: 0.1, Top: 10},
			expectedNames: []string{"fnB", "fnA", "fnC"},
			expectedFlatD: []float64{6.0, 2.0, -0.5},
		},
		{
			name: "min_share filters out noise below threshold",
			baseFns: map[string]int64{
				"fnLarge": 10000, // per op = 100.0, total base flat = 100.0
				"fnNoise": 100,   // per op = 1.0
			},
			headFns: map[string]int64{
				"fnLarge": 15000, // per op = 150.0 -> delta +50.0 (50% of total)
				"fnNoise": 105,   // per op = 1.05 -> delta +0.05 (0.05% of total, below 2% min_share)
			},
			baseOps:       100,
			headOps:       100,
			opts:          profile.DiffOptions{MinShare: 2.0, Top: 10}, // threshold = 2% of 101.0 = 2.02
			expectedNames: []string{"fnLarge"},
			expectedFlatD: []float64{50.0},
		},
		{
			name: "top truncation limits candidates count",
			baseFns: map[string]int64{
				"fn1": 100,
				"fn2": 100,
				"fn3": 100,
				"fn4": 100,
			},
			headFns: map[string]int64{
				"fn1": 500, // delta +4
				"fn2": 400, // delta +3
				"fn3": 300, // delta +2
				"fn4": 200, // delta +1
			},
			baseOps:       100,
			headOps:       100,
			opts:          profile.DiffOptions{MinShare: 0.1, Top: 2},
			expectedNames: []string{"fn1", "fn2"},
			expectedFlatD: []float64{4.0, 3.0},
		},
		{
			name: "identical profiles produce zero candidates",
			baseFns: map[string]int64{
				"fn1": 100,
				"fn2": 200,
			},
			headFns: map[string]int64{
				"fn1": 100,
				"fn2": 200,
			},
			baseOps:       100,
			headOps:       100,
			opts:          profile.DiffOptions{MinShare: 1.0, Top: 10},
			expectedNames: nil,
			expectedFlatD: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseProf := makeDiffSyntheticProfile(tc.baseFns, tc.baseOps)
			headProf := makeDiffSyntheticProfile(tc.headFns, tc.headOps)

			candidates, err := profile.Diff(baseProf, headProf, 0, tc.baseOps, tc.headOps, tc.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(candidates) != len(tc.expectedNames) {
				t.Fatalf("expected %d candidates, got %d: %+v", len(tc.expectedNames), len(candidates), candidates)
			}

			for i, expName := range tc.expectedNames {
				c := candidates[i]
				if c.Function != expName {
					t.Errorf("candidate %d: expected %s, got %s", i, expName, c.Function)
				}
				if math.Abs(c.FlatDelta-tc.expectedFlatD[i]) > 1e-9 {
					t.Errorf("candidate %d (%s): expected FlatDelta %f, got %f", i, expName, tc.expectedFlatD[i], c.FlatDelta)
				}
			}
		})
	}
}

func TestDiff_NilProfileError(t *testing.T) {
	t.Parallel()

	dummy := &pprof.Profile{}
	opts := profile.DiffOptions{}

	if _, err := profile.Diff(nil, dummy, 0, 10, 10, opts); err == nil {
		t.Error("expected error for nil baseProf, got nil")
	}
	if _, err := profile.Diff(dummy, nil, 0, 10, 10, opts); err == nil {
		t.Error("expected error for nil headProf, got nil")
	}
}

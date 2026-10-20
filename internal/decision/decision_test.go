package decision_test

import (
	"math"
	"testing"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/decision"
	"github.com/kyyril/perfguard/internal/model"
)

func validSamples(count int, val float64) []float64 {
	s := make([]float64, count)
	for i := range s {
		s[i] = val
	}
	return s
}

func TestDecisionRules_All11Rules(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Thresholds.MinSamples = 6
	cfg.Thresholds.Alpha = 0.05
	cfg.Noise.MaxCV = 10.0
	cfg.Noise.OnNoisy = "inconclusive"
	cfg.Thresholds.ZeroBase = "warn"
	cfg.Thresholds.Metrics["sec/op"] = config.ThresholdValue{Warn: 5.0, Fail: 15.0}

	tests := []struct {
		name           string
		input          decision.MetricInput
		cfg            *config.Config
		expectedStatus model.Status
		expectedReason string
	}{
		{
			name: "Rule 1: Benchmark exists only in head",
			input: decision.MetricInput{
				Benchmark:  "BenchmarkNew",
				Metric:     "sec/op",
				BaseExists: false,
				HeadExists: true,
			},
			cfg:            cfg,
			expectedStatus: model.StatusNew,
			expectedReason: model.ReasonNewBenchmark,
		},
		{
			name: "Rule 2: Benchmark exists only in base",
			input: decision.MetricInput{
				Benchmark:  "BenchmarkOld",
				Metric:     "sec/op",
				BaseExists: true,
				HeadExists: false,
			},
			cfg:            cfg,
			expectedStatus: model.StatusRemoved,
			expectedReason: model.ReasonRemovedBenchmark,
		},
		{
			name: "Rule 3: Insufficient samples",
			input: decision.MetricInput{
				Benchmark:  "BenchmarkFew",
				Metric:     "sec/op",
				BaseExists: true,
				HeadExists: true,
				BaseValues: validSamples(4, 100),
				HeadValues: validSamples(6, 100),
			},
			cfg:            cfg,
			expectedStatus: model.StatusInconclusive,
			expectedReason: model.ReasonInsufficientSamples,
		},
		{
			name: "Rule 4: Environment mismatch in stored mode",
			input: decision.MetricInput{
				Benchmark:   "BenchmarkEnv",
				Metric:      "sec/op",
				BaseExists:  true,
				HeadExists:  true,
				BaseValues:  validSamples(6, 100),
				HeadValues:  validSamples(6, 100),
				EnvMismatch: true,
			},
			cfg: func() *config.Config {
				c := config.Default()
				c.Execution.Mode = "stored"
				return c
			}(),
			expectedStatus: model.StatusInconclusive,
			expectedReason: model.ReasonEnvMismatch,
		},
		{
			name: "Rule 5: Noisy CV exceeding max_cv",
			input: decision.MetricInput{
				Benchmark:  "BenchmarkNoisy",
				Metric:     "sec/op",
				BaseExists: true,
				HeadExists: true,
				BaseValues: validSamples(6, 100),
				HeadValues: validSamples(6, 100),
				CVBase:     15.0, // > 10.0
				CVHead:     2.0,
			},
			cfg:            cfg,
			expectedStatus: model.StatusInconclusive,
			expectedReason: model.ReasonNoisy,
		},
		{
			name: "Rule 6: Zero base median with positive head median",
			input: decision.MetricInput{
				Benchmark:  "BenchmarkZeroBase",
				Metric:     "allocs/op",
				BaseExists: true,
				HeadExists: true,
				BaseValues: validSamples(6, 0),
				HeadValues: validSamples(6, 5),
				BaseMedian: 0.0,
				HeadMedian: 5.0,
				PValue:     0.001, // significant
			},
			cfg:            cfg,
			expectedStatus: model.StatusWarn,
			expectedReason: model.ReasonZeroBase,
		},
		{
			name: "Rule 7: Not significant (p >= alpha)",
			input: decision.MetricInput{
				Benchmark:  "BenchmarkInsignif",
				Metric:     "sec/op",
				BaseExists: true,
				HeadExists: true,
				BaseValues: validSamples(6, 100),
				HeadValues: validSamples(6, 130),
				BaseMedian: 100,
				HeadMedian: 130,
				DeltaPct:   30.0, // Large shift but p >= 0.05
				PValue:     0.12,
			},
			cfg:            cfg,
			expectedStatus: model.StatusUnchanged,
			expectedReason: model.ReasonNotSignificant,
		},
		{
			name: "Rule 8: Significant worse exceeding fail threshold",
			input: decision.MetricInput{
				Benchmark:  "BenchmarkRegressFail",
				Metric:     "sec/op",
				BaseExists: true,
				HeadExists: true,
				BaseValues: validSamples(6, 100),
				HeadValues: validSamples(6, 120),
				BaseMedian: 100,
				HeadMedian: 120,
				DeltaPct:   20.0, // >= 15.0
				PValue:     0.001,
			},
			cfg:            cfg,
			expectedStatus: model.StatusFail,
			expectedReason: model.ReasonExceedsFail,
		},
		{
			name: "Rule 9: Significant worse exceeding warn threshold",
			input: decision.MetricInput{
				Benchmark:  "BenchmarkRegressWarn",
				Metric:     "sec/op",
				BaseExists: true,
				HeadExists: true,
				BaseValues: validSamples(6, 100),
				HeadValues: validSamples(6, 108),
				BaseMedian: 100,
				HeadMedian: 108,
				DeltaPct:   8.0, // >= 5.0, < 15.0
				PValue:     0.01,
			},
			cfg:            cfg,
			expectedStatus: model.StatusWarn,
			expectedReason: model.ReasonExceedsWarn,
		},
		{
			name: "Rule 10: Significant improvement",
			input: decision.MetricInput{
				Benchmark:  "BenchmarkSpeedup",
				Metric:     "sec/op",
				BaseExists: true,
				HeadExists: true,
				BaseValues: validSamples(6, 100),
				HeadValues: validSamples(6, 90),
				BaseMedian: 100,
				HeadMedian: 90,
				DeltaPct:   -10.0, // <= -5.0
				PValue:     0.005,
			},
			cfg:            cfg,
			expectedStatus: model.StatusImproved,
			expectedReason: model.ReasonImprovement,
		},
		{
			name: "Rule 11: Otherwise below threshold",
			input: decision.MetricInput{
				Benchmark:  "BenchmarkBelow",
				Metric:     "sec/op",
				BaseExists: true,
				HeadExists: true,
				BaseValues: validSamples(6, 100),
				HeadValues: validSamples(6, 102),
				BaseMedian: 100,
				HeadMedian: 102,
				DeltaPct:   2.0, // < 5.0
				PValue:     0.01,
			},
			cfg:            cfg,
			expectedStatus: model.StatusUnchanged,
			expectedReason: model.ReasonBelowThreshold,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason := decision.DecideRule(tt.input, tt.cfg, tt.cfg.Thresholds.Alpha)
			if status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, status)
			}
			if reason != tt.expectedReason {
				t.Errorf("expected reason %s, got %s", tt.expectedReason, reason)
			}
		})
	}
}

func TestRuleOrderPrecedence(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Thresholds.MinSamples = 6
	cfg.Noise.MaxCV = 10.0
	cfg.Execution.Mode = "stored"

	t.Run("Rule 3 takes precedence over Rule 5 and Rule 8", func(t *testing.T) {
		// Both insufficient samples (3 samples) AND high noise (CV 50%) AND huge delta (100%)
		in := decision.MetricInput{
			Benchmark:  "PrecedenceTest",
			Metric:     "sec/op",
			BaseExists: true,
			HeadExists: true,
			BaseValues: validSamples(3, 100),
			HeadValues: validSamples(3, 200),
			CVBase:     50.0,
			CVHead:     50.0,
			DeltaPct:   100.0,
			PValue:     0.001,
		}
		status, reason := decision.DecideRule(in, cfg, 0.05)
		if status != model.StatusInconclusive || reason != model.ReasonInsufficientSamples {
			t.Errorf("expected Rule 3 insufficient_samples, got %s/%s", status, reason)
		}
	})

	t.Run("Rule 5 takes precedence over Rule 8", func(t *testing.T) {
		// High noise (CV 25%) AND delta >= fail (50%)
		in := decision.MetricInput{
			Benchmark:  "NoiseOverFail",
			Metric:     "sec/op",
			BaseExists: true,
			HeadExists: true,
			BaseValues: validSamples(6, 100),
			HeadValues: validSamples(6, 150),
			CVBase:     25.0,
			DeltaPct:   50.0,
			PValue:     0.001,
		}
		status, reason := decision.DecideRule(in, cfg, 0.05)
		if status != model.StatusInconclusive || reason != model.ReasonNoisy {
			t.Errorf("expected Rule 5 noisy, got %s/%s", status, reason)
		}
	})

	t.Run("Rule 7 takes precedence over Rule 8", func(t *testing.T) {
		// Delta 50% >= fail, but p >= 0.05 (not significant)
		in := decision.MetricInput{
			Benchmark:  "NotSignifOverFail",
			Metric:     "sec/op",
			BaseExists: true,
			HeadExists: true,
			BaseValues: validSamples(6, 100),
			HeadValues: validSamples(6, 150),
			DeltaPct:   50.0,
			PValue:     0.08,
		}
		status, reason := decision.DecideRule(in, cfg, 0.05)
		if status != model.StatusUnchanged || reason != model.ReasonNotSignificant {
			t.Errorf("expected Rule 7 not_significant, got %s/%s", status, reason)
		}
	})
}

func TestWorstVerdict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		statuses []model.Status
		expected model.Verdict
	}{
		{
			name:     "clean pass with unchanged only",
			statuses: []model.Status{model.StatusUnchanged, model.StatusUnchanged},
			expected: model.VerdictPass,
		},
		{
			name:     "new and improved do not raise verdict",
			statuses: []model.Status{model.StatusNew, model.StatusImproved, model.StatusUnchanged, model.StatusRemoved},
			expected: model.VerdictPass,
		},
		{
			name:     "inconclusive raises verdict to inconclusive",
			statuses: []model.Status{model.StatusUnchanged, model.StatusInconclusive},
			expected: model.VerdictInconclusive,
		},
		{
			name:     "warn dominates inconclusive",
			statuses: []model.Status{model.StatusInconclusive, model.StatusWarn},
			expected: model.VerdictWarn,
		},
		{
			name:     "fail dominates warn",
			statuses: []model.Status{model.StatusWarn, model.StatusFail, model.StatusInconclusive},
			expected: model.VerdictFail,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var items []model.MetricComparison
			for _, st := range tc.statuses {
				items = append(items, model.MetricComparison{Status: st})
			}
			got := decision.ComputeOverallVerdict(items)
			if got != tc.expected {
				t.Errorf("expected verdict %s, got %s", tc.expected, got)
			}
		})
	}
}

func TestCustomMetricDirection(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Thresholds.Custom["throughput"] = config.CustomMetricThreshold{
		Warn:      5.0,
		Fail:      15.0,
		Direction: "higher_is_better",
	}

	// Throughput drops from 100 to 80 (-20% raw, so +20% worse)
	inWorse := decision.MetricInput{
		Benchmark:  "BenchmarkNet",
		Metric:     "throughput",
		BaseExists: true,
		HeadExists: true,
		BaseValues: validSamples(6, 100),
		HeadValues: validSamples(6, 80),
		BaseMedian: 100,
		HeadMedian: 80,
		DeltaPct:   -20.0,
		PValue:     0.001,
	}

	status, reason := decision.DecideRule(inWorse, cfg, 0.05)
	if status != model.StatusFail || reason != model.ReasonExceedsFail {
		t.Errorf("expected throughput drop to fail, got %s/%s", status, reason)
	}

	// Throughput increases from 100 to 120 (+20% raw, so -20% better)
	inBetter := decision.MetricInput{
		Benchmark:  "BenchmarkNet",
		Metric:     "throughput",
		BaseExists: true,
		HeadExists: true,
		BaseValues: validSamples(6, 100),
		HeadValues: validSamples(6, 120),
		BaseMedian: 100,
		HeadMedian: 120,
		DeltaPct:   20.0,
		PValue:     0.001,
	}

	statusB, reasonB := decision.DecideRule(inBetter, cfg, 0.05)
	if statusB != model.StatusImproved || reasonB != model.ReasonImprovement {
		t.Errorf("expected throughput gain to improve, got %s/%s", statusB, reasonB)
	}
}

func TestOverrides(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Thresholds.Metrics["sec/op"] = config.ThresholdValue{Warn: 5.0, Fail: 15.0}
	cfg.Thresholds.Overrides = []config.Override{
		{
			Match:  "^BenchmarkSlow",
			Metric: "sec/op",
			Warn:   10.0,
			Fail:   25.0,
		},
	}

	warn, fail, _ := decision.ResolveThresholds("BenchmarkSlowNetwork", "sec/op", cfg)
	if warn != 10.0 || fail != 25.0 {
		t.Errorf("expected override thresholds 10/25, got %v/%v", warn, fail)
	}

	warnStd, failStd, _ := decision.ResolveThresholds("BenchmarkFast", "sec/op", cfg)
	if warnStd != 5.0 || failStd != 15.0 {
		t.Errorf("expected standard thresholds 5/15, got %v/%v", warnStd, failStd)
	}
}

func TestCheckEnvironment(t *testing.T) {
	t.Parallel()

	baseEnv := model.Environment{
		OS:         "linux",
		Arch:       "amd64",
		CPUModel:   "Intel Core i7",
		GOMAXPROCS: 8,
		GoVersion:  "go1.24.0",
	}

	t.Run("identical environment matches", func(t *testing.T) {
		headEnv := baseEnv
		headEnv.GoVersion = "go1.24.6" // Patch release matches minor version
		compat, diffs := decision.CheckEnvironment(baseEnv, headEnv)
		if !compat || len(diffs) > 0 {
			t.Errorf("expected compatible environment, got diffs: %v", diffs)
		}
	})

	t.Run("OS or Go minor mismatch produces incompatibility", func(t *testing.T) {
		headEnv := baseEnv
		headEnv.OS = "darwin"
		headEnv.GoVersion = "go1.25.0"
		compat, diffs := decision.CheckEnvironment(baseEnv, headEnv)
		if compat || len(diffs) != 2 {
			t.Errorf("expected incompatible environment with 2 diffs, got %v diffs", diffs)
		}
	})
}

func TestAdjustHolmBonferroni(t *testing.T) {
	t.Parallel()

	rawP := []float64{0.01, 0.04, 0.02}
	adj := decision.AdjustHolmBonferroni(rawP)

	if len(adj) != 3 {
		t.Fatalf("expected 3 adjusted p-values, got %d", len(adj))
	}
	// Smallest p=0.01 multiplied by 3 -> 0.03
	if math.Abs(adj[0]-0.03) > 1e-8 {
		t.Errorf("expected adj[0] == 0.03, got %v", adj[0])
	}
}

func TestCompareRuns_FullFlow(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Thresholds.MinSamples = 2
	cfg.Noise.MaxCV = 100.0

	baseRun := model.Run{
		ID: "run-base",
		Benchmarks: []model.BenchmarkResult{
			{
				Name: "BenchmarkFast",
				Samples: []model.Sample{
					{Values: map[string]float64{"sec/op": 10.0}},
					{Values: map[string]float64{"sec/op": 10.2}},
				},
			},
			{
				Name: "BenchmarkRemoved",
				Samples: []model.Sample{
					{Values: map[string]float64{"sec/op": 50.0}},
				},
			},
		},
	}

	headRun := model.Run{
		ID: "run-head",
		Benchmarks: []model.BenchmarkResult{
			{
				Name: "BenchmarkFast",
				Samples: []model.Sample{
					{Values: map[string]float64{"sec/op": 15.0}},
					{Values: map[string]float64{"sec/op": 15.2}},
				},
			},
			{
				Name: "BenchmarkNew",
				Samples: []model.Sample{
					{Values: map[string]float64{"sec/op": 30.0}},
				},
			},
		},
	}

	cmp, err := decision.CompareRuns(baseRun, headRun, cfg)
	if err != nil {
		t.Fatalf("unexpected error comparing runs: %v", err)
	}

	if len(cmp.Items) != 3 {
		t.Fatalf("expected 3 metric comparisons, got %d", len(cmp.Items))
	}

	// Verify items contain BenchmarkFast, BenchmarkNew, BenchmarkRemoved
	foundMap := make(map[string]model.Status)
	for _, item := range cmp.Items {
		foundMap[item.Benchmark] = item.Status
	}

	if foundMap["BenchmarkNew"] != model.StatusNew {
		t.Errorf("expected BenchmarkNew to be new, got %s", foundMap["BenchmarkNew"])
	}
	if foundMap["BenchmarkRemoved"] != model.StatusRemoved {
		t.Errorf("expected BenchmarkRemoved to be removed, got %s", foundMap["BenchmarkRemoved"])
	}
}

func TestCheckEnvironment_AllMismatches(t *testing.T) {
	t.Parallel()

	base := model.Environment{
		OS:         "linux",
		Arch:       "amd64",
		CPUModel:   "AMD Ryzen",
		GOMAXPROCS: 4,
		GoVersion:  "devel",
	}

	head := model.Environment{
		OS:         "windows",
		Arch:       "arm64",
		CPUModel:   "Intel Core",
		GOMAXPROCS: 8,
		GoVersion:  "devel-other",
	}

	compat, diffs := decision.CheckEnvironment(base, head)
	if compat {
		t.Error("expected incompatible environment, got true")
	}
	if len(diffs) < 4 {
		t.Errorf("expected at least 4 diffs, got %d: %v", len(diffs), diffs)
	}
}

func TestAdjustHolmBonferroni_EdgeCases(t *testing.T) {
	t.Parallel()

	// Empty
	if len(decision.AdjustHolmBonferroni(nil)) != 0 {
		t.Error("expected empty result for nil input")
	}

	// Single element
	single := []float64{0.04}
	resSingle := decision.AdjustHolmBonferroni(single)
	if len(resSingle) != 1 || resSingle[0] != 0.04 {
		t.Errorf("expected [0.04], got %v", resSingle)
	}

	// Cap at 1.0
	largeP := []float64{0.6, 0.8}
	resCapped := decision.AdjustHolmBonferroni(largeP)
	for _, p := range resCapped {
		if p > 1.0 {
			t.Errorf("expected p <= 1.0, got %v", p)
		}
	}
}

func TestCompareRuns_StoredModeAndHolm(t *testing.T) {
	t.Parallel()

	baseRun := model.Run{
		ID:  "run-base",
		Env: model.Environment{OS: "linux", Arch: "amd64"},
		Benchmarks: []model.BenchmarkResult{
			{
				Name: "BenchmarkTest",
				Samples: []model.Sample{
					{Values: map[string]float64{"sec/op": 10.0, "unknown_metric": 5.0}},
					{Values: map[string]float64{"sec/op": 10.1, "unknown_metric": 5.1}},
				},
			},
		},
	}

	headRun := model.Run{
		ID:  "run-head",
		Env: model.Environment{OS: "darwin", Arch: "arm64"}, // Mismatched env
		Benchmarks: []model.BenchmarkResult{
			{
				Name: "BenchmarkTest",
				Samples: []model.Sample{
					{Values: map[string]float64{"sec/op": 12.0, "unknown_metric": 6.0}},
					{Values: map[string]float64{"sec/op": 12.1, "unknown_metric": 6.1}},
				},
			},
		},
	}

	// 1. nil config fallback
	cmpDefault, err := decision.CompareRuns(baseRun, headRun, nil)
	if err != nil {
		t.Fatalf("unexpected error with nil config: %v", err)
	}
	if cmpDefault.Verdict == "" {
		t.Error("expected non-empty verdict")
	}

	// 2. Stored mode with environment mismatch
	cfgStored := config.Default()
	cfgStored.Execution.Mode = "stored"
	cfgStored.Thresholds.MinSamples = 2
	cfgStored.Thresholds.Correction = "holm"

	cmpStored, err := decision.CompareRuns(baseRun, headRun, cfgStored)
	if err != nil {
		t.Fatalf("unexpected error in stored mode: %v", err)
	}
	if cmpStored.Verdict != model.VerdictInconclusive {
		t.Errorf("expected inconclusive verdict for stored mode env mismatch, got %s", cmpStored.Verdict)
	}
	if len(cmpStored.Notes) == 0 {
		t.Error("expected notes containing env mismatch diffs")
	}
}

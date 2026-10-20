// Package decision evaluates benchmark comparisons against configured rules,
// thresholds, noise limits, and statistical significance to assign statuses,
// fixed reason codes, and an overall verdict.
package decision

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/stats"
)

// MetricInput holds the statistical metrics and context for evaluating a single (benchmark, metric) pair.
type MetricInput struct {
	Benchmark   string
	Metric      string
	BaseExists  bool
	HeadExists  bool
	BaseValues  []float64
	HeadValues  []float64
	BaseMedian  float64
	HeadMedian  float64
	CVBase      float64
	CVHead      float64
	DeltaPct    float64
	CI          [2]float64
	PValue      float64
	EnvMismatch bool
}

// ResolveThresholds returns the warn threshold, fail threshold, and direction for a given benchmark and metric.
func ResolveThresholds(benchmark, metric string, cfg *config.Config) (warn, fail float64, direction string) {
	direction = "lower_is_better"

	// 1. Check custom metric defaults
	if cm, ok := cfg.Thresholds.Custom[metric]; ok {
		warn = cm.Warn
		fail = cm.Fail
		if cm.Direction != "" {
			direction = cm.Direction
		}
	} else if std, ok := cfg.Thresholds.Metrics[metric]; ok {
		// 2. Check standard metrics
		warn = std.Warn
		fail = std.Fail
	} else {
		// Default fallback
		warn = 5.0
		fail = 15.0
	}

	// 3. Check overrides (last matching override takes precedence)
	for _, ov := range cfg.Thresholds.Overrides {
		if ov.Metric != metric {
			continue
		}
		if re, err := regexp.Compile(ov.Match); err == nil && re.MatchString(benchmark) {
			warn = ov.Warn
			fail = ov.Fail
		}
	}

	return warn, fail, direction
}

// AdjustDelta adjusts delta such that positive values always indicate performance degradation (worse).
func AdjustDelta(rawDelta float64, direction string) float64 {
	if direction == "higher_is_better" {
		return -rawDelta
	}
	return rawDelta
}

// DecideRule evaluates the 11 decision rules in the exact order documented in ARCHITECTURE.md.
func DecideRule(in MetricInput, cfg *config.Config, alpha float64) (model.Status, string) {
	// Rule 1: Benchmark exists only in head
	if !in.BaseExists && in.HeadExists {
		return model.StatusNew, model.ReasonNewBenchmark
	}

	// Rule 2: Benchmark exists only in base
	if in.BaseExists && !in.HeadExists {
		return model.StatusRemoved, model.ReasonRemovedBenchmark
	}

	// Rule 3: N base or head is less than thresholds.min_samples
	nBase := len(in.BaseValues)
	nHead := len(in.HeadValues)
	if nBase < cfg.Thresholds.MinSamples || nHead < cfg.Thresholds.MinSamples {
		return model.StatusInconclusive, model.ReasonInsufficientSamples
	}

	// Rule 4: Environment does not match (stored mode)
	if cfg.Execution.Mode == "stored" && in.EnvMismatch {
		return model.StatusInconclusive, model.ReasonEnvMismatch
	}

	// Rule 5: CV on either side exceeds noise.max_cv after extra samples are exhausted
	if in.CVBase > cfg.Noise.MaxCV || in.CVHead > cfg.Noise.MaxCV {
		status := model.StatusInconclusive
		if cfg.Noise.OnNoisy == "warn" {
			status = model.StatusWarn
		}
		return status, model.ReasonNoisy
	}

	warnThresh, failThresh, direction := ResolveThresholds(in.Benchmark, in.Metric, cfg)
	isSignificant := in.PValue < alpha
	adjDelta := AdjustDelta(in.DeltaPct, direction)

	// Rule 6: Base median is 0, head median is greater than 0, and significant
	if in.BaseMedian == 0 && in.HeadMedian > 0 && isSignificant {
		status := model.StatusWarn
		if cfg.Thresholds.ZeroBase == "fail" {
			status = model.StatusFail
		}
		return status, model.ReasonZeroBase
	}

	// Rule 7: Not significant (p >= alpha)
	if !isSignificant {
		return model.StatusUnchanged, model.ReasonNotSignificant
	}

	// Rule 8: Significant, worse, delta >= fail
	if adjDelta >= failThresh {
		return model.StatusFail, model.ReasonExceedsFail
	}

	// Rule 9: Significant, worse, delta >= warn
	if adjDelta >= warnThresh {
		return model.StatusWarn, model.ReasonExceedsWarn
	}

	// Rule 10: Significant, better, delta <= -warn (absolute value of improvement >= warn)
	if adjDelta <= -warnThresh {
		return model.StatusImproved, model.ReasonImprovement
	}

	// Rule 11: Otherwise
	return model.StatusUnchanged, model.ReasonBelowThreshold
}

// ComputeOverallVerdict computes the worst verdict from all metric comparison statuses.
// Fail > Warn > Inconclusive > Pass. Statuses new, removed, improved, and unchanged do not raise the verdict.
func ComputeOverallVerdict(items []model.MetricComparison) model.Verdict {
	verdict := model.VerdictPass
	for _, item := range items {
		switch item.Status {
		case model.StatusFail:
			return model.VerdictFail
		case model.StatusWarn:
			verdict = model.WorstVerdict(verdict, model.VerdictWarn)
		case model.StatusInconclusive:
			verdict = model.WorstVerdict(verdict, model.VerdictInconclusive)
		}
	}
	return verdict
}

// CheckEnvironment verifies if base and head runtime environments are compatible in stored mode.
// It checks OS, Arch, CPU model, GOMAXPROCS, and Go minor version.
func CheckEnvironment(baseEnv, headEnv model.Environment) (bool, []string) {
	var diffs []string

	if baseEnv.OS != headEnv.OS {
		diffs = append(diffs, fmt.Sprintf("OS mismatch: base %s vs head %s", baseEnv.OS, headEnv.OS))
	}
	if baseEnv.Arch != headEnv.Arch {
		diffs = append(diffs, fmt.Sprintf("Arch mismatch: base %s vs head %s", baseEnv.Arch, headEnv.Arch))
	}
	if baseEnv.CPUModel != "" && headEnv.CPUModel != "" && baseEnv.CPUModel != headEnv.CPUModel {
		diffs = append(diffs, fmt.Sprintf("CPU model mismatch: base %s vs head %s", baseEnv.CPUModel, headEnv.CPUModel))
	}
	if baseEnv.GOMAXPROCS != headEnv.GOMAXPROCS && baseEnv.GOMAXPROCS > 0 && headEnv.GOMAXPROCS > 0 {
		diffs = append(diffs, fmt.Sprintf("GOMAXPROCS mismatch: base %d vs head %d", baseEnv.GOMAXPROCS, headEnv.GOMAXPROCS))
	}

	baseMinor := parseGoMinorVersion(baseEnv.GoVersion)
	headMinor := parseGoMinorVersion(headEnv.GoVersion)
	if baseMinor != "" && headMinor != "" && baseMinor != headMinor {
		diffs = append(diffs, fmt.Sprintf("Go minor version mismatch: base %s vs head %s", baseMinor, headMinor))
	}

	return len(diffs) == 0, diffs
}

func parseGoMinorVersion(v string) string {
	// Example: "go1.24.0" -> "1.24"
	v = strings.TrimPrefix(v, "go")
	parts := strings.Split(v, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}

type pIndexed struct {
	index int
	pVal  float64
}

// AdjustHolmBonferroni adjusts p-values using the step-down Holm-Bonferroni method.
func AdjustHolmBonferroni(rawP []float64) []float64 {
	m := len(rawP)
	if m <= 1 {
		res := make([]float64, len(rawP))
		copy(res, rawP)
		return res
	}

	indexed := make([]pIndexed, m)
	for i, p := range rawP {
		indexed[i] = pIndexed{index: i, pVal: p}
	}

	sort.Slice(indexed, func(i, j int) bool {
		return indexed[i].pVal < indexed[j].pVal
	})

	adj := make([]float64, m)
	runningMax := 0.0

	for k, item := range indexed {
		// multiplier is m - k
		factor := float64(m - k)
		curr := item.pVal * factor
		if curr > 1.0 {
			curr = 1.0
		}
		if curr > runningMax {
			runningMax = curr
		}
		adj[item.index] = runningMax
	}

	return adj
}

// CompareRuns performs complete pairwise statistical analysis and rule evaluation between two Runs.
func CompareRuns(baseRun, headRun model.Run, cfg *config.Config) (model.Comparison, error) {
	if cfg == nil {
		cfg = config.Default()
	}

	envMatch, envDiffs := true, []string(nil)
	if cfg.Execution.Mode == "stored" {
		envMatch, envDiffs = CheckEnvironment(baseRun.Env, headRun.Env)
	}

	baseMap := make(map[string]model.BenchmarkResult)
	for _, b := range baseRun.Benchmarks {
		baseMap[b.Name] = b
	}

	headMap := make(map[string]model.BenchmarkResult)
	for _, b := range headRun.Benchmarks {
		headMap[b.Name] = b
	}

	// Gather union of all benchmark names in deterministic order
	nameSet := make(map[string]struct{})
	for name := range baseMap {
		nameSet[name] = struct{}{}
	}
	for name := range headMap {
		nameSet[name] = struct{}{}
	}

	allNames := make([]string, 0, len(nameSet))
	for name := range nameSet {
		allNames = append(allNames, name)
	}
	sort.Strings(allNames)

	var inputs []MetricInput

	for _, name := range allNames {
		bRes, bExists := baseMap[name]
		hRes, hExists := headMap[name]

		// Gather metrics present in either side
		metricSet := make(map[string]struct{})
		if bExists {
			for _, s := range bRes.Samples {
				for m := range s.Values {
					metricSet[m] = struct{}{}
				}
			}
		}
		if hExists {
			for _, s := range hRes.Samples {
				for m := range s.Values {
					metricSet[m] = struct{}{}
				}
			}
		}

		metrics := make([]string, 0, len(metricSet))
		for m := range metricSet {
			metrics = append(metrics, m)
		}
		sort.Strings(metrics)

		for _, m := range metrics {
			var bVals, hVals []float64
			if bExists {
				for _, s := range bRes.Samples {
					if val, ok := s.Values[m]; ok {
						bVals = append(bVals, val)
					}
				}
			}
			if hExists {
				for _, s := range hRes.Samples {
					if val, ok := s.Values[m]; ok {
						hVals = append(hVals, val)
					}
				}
			}

			in := MetricInput{
				Benchmark:   name,
				Metric:      m,
				BaseExists:  bExists,
				HeadExists:  hExists,
				BaseValues:  bVals,
				HeadValues:  hVals,
				EnvMismatch: !envMatch,
			}

			if bExists && len(bVals) > 0 {
				in.BaseMedian, _ = stats.Median(bVals)
				in.CVBase, _ = stats.CV(bVals)
			}
			if hExists && len(hVals) > 0 {
				in.HeadMedian, _ = stats.Median(hVals)
				in.CVHead, _ = stats.CV(hVals)
			}

			if bExists && hExists && len(bVals) > 0 && len(hVals) > 0 {
				in.DeltaPct, _ = stats.DeltaPct(in.BaseMedian, in.HeadMedian)
				ci, _ := stats.BootstrapCI(bVals, hVals, 0.95, 1000, 42)
				in.CI = ci
				_, p, _ := stats.MannWhitneyU(bVals, hVals)
				in.PValue = p
			}

			inputs = append(inputs, in)
		}
	}

	// Apply multiple testing correction if configured
	if cfg.Thresholds.Correction == "holm" && len(inputs) > 0 {
		rawP := make([]float64, len(inputs))
		for i, in := range inputs {
			rawP[i] = in.PValue
		}
		adjP := AdjustHolmBonferroni(rawP)
		for i := range inputs {
			inputs[i].PValue = adjP[i]
		}
	}

	alpha := cfg.Thresholds.Alpha
	if alpha <= 0 {
		alpha = 0.05
	}

	var items []model.MetricComparison
	for _, in := range inputs {
		status, reason := DecideRule(in, cfg, alpha)
		items = append(items, model.MetricComparison{
			Benchmark:  in.Benchmark,
			Metric:     in.Metric,
			BaseMedian: in.BaseMedian,
			HeadMedian: in.HeadMedian,
			DeltaPct:   in.DeltaPct,
			CI:         in.CI,
			PValue:     in.PValue,
			CVBase:     in.CVBase,
			CVHead:     in.CVHead,
			NBase:      len(in.BaseValues),
			NHead:      len(in.HeadValues),
			Status:     status,
			Reason:     reason,
		})
	}

	verdict := ComputeOverallVerdict(items)

	var notes []string
	if len(envDiffs) > 0 {
		notes = append(notes, envDiffs...)
	}

	return model.Comparison{
		SchemaVersion: model.CurrentSchemaVersion,
		Base:          baseRun.Summary(),
		Head:          headRun.Summary(),
		Items:         items,
		Verdict:       verdict,
		Notes:         notes,
	}, nil
}

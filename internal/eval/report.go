package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"perfguard/internal/model"
)

// ScenarioResult aggregates all trials for a single scenario.
type ScenarioResult struct {
	Scenario        Scenario
	Metrics         Metrics
	FailureExamples []string
}

// ReportData holds aggregated results for all scenarios.
type ReportData struct {
	Scenarios   []ScenarioResult
	Environment EnvironmentFingerprint
}

// EnvironmentFingerprint captures system information.
type EnvironmentFingerprint struct {
	OS      string
	Arch    string
	GoVer   string
	NumCPU  int
	Hostnam string
}

// AggregateResults reads all trial results for a scenario and computes metrics.
func AggregateResults(ctx context.Context, scenario Scenario, outputDir string) (ScenarioResult, error) {
	scenarioDir := filepath.Join(outputDir, scenario.ID)
	entries, err := os.ReadDir(scenarioDir)
	if err != nil {
		return ScenarioResult{}, fmt.Errorf("read scenario output: %w", err)
	}

	var trials []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "trial_") {
			trials = append(trials, e.Name())
		}
	}
	sort.Strings(trials)

	if len(trials) == 0 {
		return ScenarioResult{}, fmt.Errorf("no trial results found in %s", scenarioDir)
	}

	metrics := Metrics{
		TotalTrials: len(trials),
	}
	var failureExamples []string

	for _, trial := range trials {
		compFile := filepath.Join(scenarioDir, trial, "comparison.json")
		data, err := os.ReadFile(compFile)
		if err != nil {
			return ScenarioResult{}, fmt.Errorf("read %s: %w", compFile, err)
		}

		var comp model.Comparison
		if err := json.Unmarshal(data, &comp); err != nil {
			return ScenarioResult{}, fmt.Errorf("unmarshal %s: %w", compFile, err)
		}

		expectOutcome := scenario.Expect
		actualVerdict := comp.Verdict

		// Check if this trial was successful
		if isSuccessfulTrial(expectOutcome, actualVerdict) {
			metrics.SuccessfulRuns++
		} else if len(failureExamples) < 3 {
			// Collect first few failures for report
			failureExamples = append(failureExamples, fmt.Sprintf("%s: expected %s, got %s",
				trial, expectOutcome.Verdict, actualVerdict))
		}

		// Check diagnosis hit
		if expectOutcome.Verdict == VerdictFail && actualVerdict == model.VerdictFail {
			metrics.DiagnosisTrials++
			if diagnosisMatches(expectOutcome.ExpectedFunction, comp.Regressions) {
				metrics.DiagnosisHits++
			}
		}
	}

	return ScenarioResult{
		Scenario:        scenario,
		Metrics:         metrics,
		FailureExamples: failureExamples,
	}, nil
}

// isSuccessfulTrial checks if the actual verdict matches the expected outcome.
func isSuccessfulTrial(expect ExpectedOutcome, actual model.Verdict) bool {
	switch expect.Verdict {
	case VerdictPass:
		return actual == model.VerdictPass
	case VerdictFail:
		return actual == model.VerdictFail
	case VerdictWarn:
		return actual == model.VerdictWarn
	default:
		return false
	}
}

// diagnosisMatches checks if the expected function appears in top K candidates.
func diagnosisMatches(expectedFunc string, regressions []model.Regression) bool {
	if expectedFunc == "" {
		return false
	}

	const topK = 3
	for _, reg := range regressions {
		if reg.Diagnosis == nil || len(reg.Diagnosis.ContributorCandidates) == 0 {
			continue
		}
		for i, cand := range reg.Diagnosis.ContributorCandidates {
			if i >= topK {
				break
			}
			if strings.Contains(cand.FunctionName, expectedFunc) {
				return true
			}
		}
	}
	return false
}

// GenerateReport creates a markdown report from aggregated results.
func GenerateReport(data ReportData) string {
	var sb strings.Builder

	sb.WriteString("# PerfGuard Evaluation Results\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", data.Environment.Timestamp()))

	// Environment section
	sb.WriteString("## Environment\n\n")
	sb.WriteString(fmt.Sprintf("- OS: %s\n", data.Environment.OS))
	sb.WriteString(fmt.Sprintf("- Arch: %s\n", data.Environment.Arch))
	sb.WriteString(fmt.Sprintf("- Go: %s\n", data.Environment.GoVer))
	sb.WriteString(fmt.Sprintf("- CPU cores: %d\n", data.Environment.NumCPU))
	sb.WriteString(fmt.Sprintf("- Hostname: %s\n\n", data.Environment.Hostnam))

	// Summary section
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("Total scenarios: %d\n\n", len(data.Scenarios)))

	// Scenario results
	sb.WriteString("## Scenario Results\n\n")
	for _, sr := range data.Scenarios {
		sb.WriteString(fmt.Sprintf("### %s\n\n", sr.Scenario.ID))
		sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", sr.Scenario.Description))
		sb.WriteString(fmt.Sprintf("**Expected verdict:** %s\n\n", sr.Scenario.Expect.Verdict))
		sb.WriteString(fmt.Sprintf("**Trials:** %d\n\n", sr.Metrics.TotalTrials))

		detRate := sr.Metrics.DetectionRate()
		lower, upper := WilsonInterval(sr.Metrics.SuccessfulRuns, sr.Metrics.TotalTrials, 0.95)
		sb.WriteString(fmt.Sprintf("**Detection rate:** %s %s\n\n",
			FormatPercent(detRate),
			FormatInterval(lower, upper)))

		if sr.Metrics.DiagnosisTrials > 0 {
			diagRate := sr.Metrics.DiagnosisHitRate()
			dLower, dUpper := WilsonInterval(sr.Metrics.DiagnosisHits, sr.Metrics.DiagnosisTrials, 0.95)
			sb.WriteString(fmt.Sprintf("**Diagnosis hit rate:** %s %s (K=3)\n\n",
				FormatPercent(diagRate),
				FormatInterval(dLower, dUpper)))
		}

		if len(sr.FailureExamples) > 0 {
			sb.WriteString("**Failure examples:**\n\n")
			for _, ex := range sr.FailureExamples {
				sb.WriteString(fmt.Sprintf("- %s\n", ex))
			}
			sb.WriteString("\n")
		}
	}

	// False positive analysis
	sb.WriteString("## False Positive Analysis\n\n")
	var totalNegTrials, totalFails, totalWarns int
	for _, sr := range data.Scenarios {
		if sr.Scenario.Expect.Verdict == VerdictPass {
			totalNegTrials += sr.Metrics.TotalTrials
			for i := 0; i < sr.Metrics.TotalTrials; i++ {
				if i >= sr.Metrics.SuccessfulRuns {
					// These trials did not result in pass
					// Count them as false positives
					totalFails++
				}
			}
		}
	}

	if totalNegTrials > 0 {
		fpFail, fpWarn := FalsePositiveRate(totalFails, totalWarns, totalNegTrials)
		fLower, fUpper := WilsonInterval(totalFails, totalNegTrials, 0.95)
		sb.WriteString(fmt.Sprintf("**Fail false positive rate:** %s %s\n\n",
			FormatPercent(fpFail),
			FormatInterval(fLower, fUpper)))
		sb.WriteString(fmt.Sprintf("**Warn false positive rate:** %s %s\n\n",
			FormatPercent(fpWarn),
			FormatInterval(0, 0)))
	} else {
		sb.WriteString("No negative scenarios in this run.\n\n")
	}

	return sb.String()
}

// CaptureEnvironment returns current system information.
func CaptureEnvironment() EnvironmentFingerprint {
	hostname, _ := os.Hostname()
	return EnvironmentFingerprint{
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		GoVer:   runtime.Version(),
		NumCPU:  runtime.NumCPU(),
		Hostnam: hostname,
	}
}

// Timestamp returns a human-readable timestamp for the environment.
func (e EnvironmentFingerprint) Timestamp() string {
	return fmt.Sprintf("%s/%s, Go %s, %d cores",
		e.OS, e.Arch, e.GoVer, e.NumCPU)
}

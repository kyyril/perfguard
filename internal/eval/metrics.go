package eval

import (
	"fmt"
	"math"
)

// Metrics aggregates evaluation results across trials.
type Metrics struct {
	ScenarioID      string
	TotalTrials     int
	SuccessfulRuns  int
	FailedRuns      int
	VerdictCounts   map[string]int // verdict -> count
	StatusCounts    map[string]int // status -> count (for expected metrics)
	DiagnosisHits   int            // diagnosis matches if applicable
	DiagnosisTrials int            // trials where diagnosis was expected
}

// DetectionRate computes the proportion of trials that detected the expected outcome.
// For positive scenarios, this is the rate at which the expected verdict occurred.
func (m *Metrics) DetectionRate() float64 {
	if m.SuccessfulRuns == 0 {
		return 0.0
	}
	// Detection means the trial succeeded and produced a verdict
	return float64(m.SuccessfulRuns) / float64(m.TotalTrials)
}

// FalsePositiveRate computes the proportion of trials that incorrectly reported regression.
// For negative scenarios, this is the rate of fail or warn verdicts.
func FalsePositiveRate(failCount, warnCount, totalTrials int) (failRate, warnRate float64) {
	if totalTrials == 0 {
		return 0.0, 0.0
	}
	failRate = float64(failCount) / float64(totalTrials)
	warnRate = float64(warnCount) / float64(totalTrials)
	return
}

// DiagnosisHitRate computes the proportion of trials where the expected function
// appeared in the top K candidates.
func (m *Metrics) DiagnosisHitRate() float64 {
	if m.DiagnosisTrials == 0 {
		return 0.0
	}
	return float64(m.DiagnosisHits) / float64(m.DiagnosisTrials)
}

// WilsonInterval computes the Wilson score confidence interval for a proportion.
// This is more accurate than the normal approximation for small sample sizes.
//
// Parameters:
//   - successes: number of successful trials
//   - trials: total number of trials
//   - confidence: confidence level (e.g., 0.95 for 95%)
//
// Returns: (lower bound, upper bound)
func WilsonInterval(successes, trials int, confidence float64) (float64, float64) {
	if trials == 0 {
		return 0.0, 0.0
	}

	p := float64(successes) / float64(trials)
	n := float64(trials)

	// Z-score for desired confidence level
	// For 95% confidence: z = 1.96
	var z float64
	switch {
	case confidence >= 0.99:
		z = 2.576 // 99%
	case confidence >= 0.95:
		z = 1.96 // 95%
	case confidence >= 0.90:
		z = 1.645 // 90%
	default:
		z = 1.96 // default to 95%
	}

	// Wilson score interval formula
	denominator := 1 + (z*z)/n
	center := p + (z*z)/(2*n)
	margin := z * math.Sqrt((p*(1-p))/n+(z*z)/(4*n*n))

	lower := (center - margin) / denominator
	upper := (center + margin) / denominator

	// Clamp to [0, 1]
	if lower < 0 {
		lower = 0
	}
	if upper > 1 {
		upper = 1
	}

	return lower, upper
}

// FormatPercent formats a proportion as a percentage string.
func FormatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value*100)
}

// FormatInterval formats a confidence interval as a string.
func FormatInterval(lower, upper float64) string {
	return fmt.Sprintf("[%.1f%%, %.1f%%]", lower*100, upper*100)
}

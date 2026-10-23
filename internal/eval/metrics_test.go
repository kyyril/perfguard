package eval

import (
	"math"
	"testing"
)

func TestMetrics_DetectionRate(t *testing.T) {
	tests := []struct {
		name           string
		successfulRuns int
		totalTrials    int
		want           float64
	}{
		{
			name:           "all successful",
			successfulRuns: 20,
			totalTrials:    20,
			want:           1.0,
		},
		{
			name:           "half successful",
			successfulRuns: 10,
			totalTrials:    20,
			want:           0.5,
		},
		{
			name:           "none successful",
			successfulRuns: 0,
			totalTrials:    20,
			want:           0.0,
		},
		{
			name:           "90% detection",
			successfulRuns: 18,
			totalTrials:    20,
			want:           0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metrics{
				SuccessfulRuns: tt.successfulRuns,
				TotalTrials:    tt.totalTrials,
			}
			got := m.DetectionRate()
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("DetectionRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFalsePositiveRate(t *testing.T) {
	tests := []struct {
		name        string
		failCount   int
		warnCount   int
		totalTrials int
		wantFail    float64
		wantWarn    float64
	}{
		{
			name:        "no false positives",
			failCount:   0,
			warnCount:   0,
			totalTrials: 30,
			wantFail:    0.0,
			wantWarn:    0.0,
		},
		{
			name:        "1% fail rate",
			failCount:   1,
			warnCount:   0,
			totalTrials: 100,
			wantFail:    0.01,
			wantWarn:    0.0,
		},
		{
			name:        "5% warn rate",
			failCount:   0,
			warnCount:   5,
			totalTrials: 100,
			wantFail:    0.0,
			wantWarn:    0.05,
		},
		{
			name:        "both present",
			failCount:   1,
			warnCount:   2,
			totalTrials: 30,
			wantFail:    1.0 / 30.0,
			wantWarn:    2.0 / 30.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFail, gotWarn := FalsePositiveRate(tt.failCount, tt.warnCount, tt.totalTrials)
			if math.Abs(gotFail-tt.wantFail) > 0.001 {
				t.Errorf("FalsePositiveRate() fail = %v, want %v", gotFail, tt.wantFail)
			}
			if math.Abs(gotWarn-tt.wantWarn) > 0.001 {
				t.Errorf("FalsePositiveRate() warn = %v, want %v", gotWarn, tt.wantWarn)
			}
		})
	}
}

func TestMetrics_DiagnosisHitRate(t *testing.T) {
	tests := []struct {
		name            string
		diagnosisHits   int
		diagnosisTrials int
		want            float64
	}{
		{
			name:            "all hits",
			diagnosisHits:   15,
			diagnosisTrials: 15,
			want:            1.0,
		},
		{
			name:            "80% hit rate",
			diagnosisHits:   12,
			diagnosisTrials: 15,
			want:            0.8,
		},
		{
			name:            "no hits",
			diagnosisHits:   0,
			diagnosisTrials: 15,
			want:            0.0,
		},
		{
			name:            "no diagnosis trials",
			diagnosisHits:   0,
			diagnosisTrials: 0,
			want:            0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metrics{
				DiagnosisHits:   tt.diagnosisHits,
				DiagnosisTrials: tt.diagnosisTrials,
			}
			got := m.DiagnosisHitRate()
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("DiagnosisHitRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWilsonInterval(t *testing.T) {
	tests := []struct {
		name       string
		successes  int
		trials     int
		confidence float64
		wantLower  float64
		wantUpper  float64
		tolerance  float64
	}{
		{
			name:       "perfect detection 20/20",
			successes:  20,
			trials:     20,
			confidence: 0.95,
			wantLower:  0.83,
			wantUpper:  1.0,
			tolerance:  0.02,
		},
		{
			name:       "90% detection 18/20",
			successes:  18,
			trials:     20,
			confidence: 0.95,
			wantLower:  0.68,
			wantUpper:  0.99,
			tolerance:  0.02,
		},
		{
			name:       "50% detection 10/20",
			successes:  10,
			trials:     20,
			confidence: 0.95,
			wantLower:  0.28,
			wantUpper:  0.72,
			tolerance:  0.02,
		},
		{
			name:       "no detection 0/20",
			successes:  0,
			trials:     20,
			confidence: 0.95,
			wantLower:  0.0,
			wantUpper:  0.17,
			tolerance:  0.02,
		},
		{
			name:       "1% false positive 1/100",
			successes:  1,
			trials:     100,
			confidence: 0.95,
			wantLower:  0.0,
			wantUpper:  0.05,
			tolerance:  0.02,
		},
		{
			name:       "zero trials",
			successes:  0,
			trials:     0,
			confidence: 0.95,
			wantLower:  0.0,
			wantUpper:  0.0,
			tolerance:  0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lower, upper := WilsonInterval(tt.successes, tt.trials, tt.confidence)
			if math.Abs(lower-tt.wantLower) > tt.tolerance {
				t.Errorf("WilsonInterval() lower = %v, want %v (±%v)", lower, tt.wantLower, tt.tolerance)
			}
			if math.Abs(upper-tt.wantUpper) > tt.tolerance {
				t.Errorf("WilsonInterval() upper = %v, want %v (±%v)", upper, tt.wantUpper, tt.tolerance)
			}

			// Sanity checks
			if lower < 0 || lower > 1 {
				t.Errorf("lower bound %v out of range [0, 1]", lower)
			}
			if upper < 0 || upper > 1 {
				t.Errorf("upper bound %v out of range [0, 1]", upper)
			}
			if lower > upper {
				t.Errorf("lower bound %v > upper bound %v", lower, upper)
			}
		})
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{0.0, "0.0%"},
		{0.01, "1.0%"},
		{0.05, "5.0%"},
		{0.5, "50.0%"},
		{0.9, "90.0%"},
		{1.0, "100.0%"},
		{0.123, "12.3%"},
		{0.956, "95.6%"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatPercent(tt.value)
			if got != tt.want {
				t.Errorf("FormatPercent(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestFormatInterval(t *testing.T) {
	tests := []struct {
		lower float64
		upper float64
		want  string
	}{
		{0.0, 0.0, "[0.0%, 0.0%]"},
		{0.01, 0.05, "[1.0%, 5.0%]"},
		{0.68, 0.99, "[68.0%, 99.0%]"},
		{0.83, 1.0, "[83.0%, 100.0%]"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatInterval(tt.lower, tt.upper)
			if got != tt.want {
				t.Errorf("FormatInterval(%v, %v) = %q, want %q", tt.lower, tt.upper, got, tt.want)
			}
		})
	}
}

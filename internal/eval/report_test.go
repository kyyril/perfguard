package eval

import (
	"strings"
	"testing"

	"github.com/kyyril/perfguard/internal/model"
)

func TestIsSuccessfulTrial(t *testing.T) {
	tests := []struct {
		name   string
		expect ExpectedOutcome
		actual model.Verdict
		want   bool
	}{
		{
			name:   "pass matches pass",
			expect: ExpectedOutcome{Verdict: VerdictPass},
			actual: model.VerdictPass,
			want:   true,
		},
		{
			name:   "fail matches fail",
			expect: ExpectedOutcome{Verdict: VerdictFail},
			actual: model.VerdictFail,
			want:   true,
		},
		{
			name:   "warn matches warn",
			expect: ExpectedOutcome{Verdict: VerdictWarn},
			actual: model.VerdictWarn,
			want:   true,
		},
		{
			name:   "pass does not match fail",
			expect: ExpectedOutcome{Verdict: VerdictPass},
			actual: model.VerdictFail,
			want:   false,
		},
		{
			name:   "fail does not match pass",
			expect: ExpectedOutcome{Verdict: VerdictFail},
			actual: model.VerdictPass,
			want:   false,
		},
		{
			name:   "pass does not match warn",
			expect: ExpectedOutcome{Verdict: VerdictPass},
			actual: model.VerdictWarn,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSuccessfulTrial(tt.expect, tt.actual)
			if got != tt.want {
				t.Errorf("isSuccessfulTrial() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiagnosisMatches(t *testing.T) {
	tests := []struct {
		name         string
		expectedFunc string
		regressions  []model.Regression
		want         bool
	}{
		{
			name:         "empty expected function",
			expectedFunc: "",
			regressions:  []model.Regression{},
			want:         false,
		},
		{
			name:         "no regressions",
			expectedFunc: "slowFunction",
			regressions:  []model.Regression{},
			want:         false,
		},
		{
			name:         "match in first candidate",
			expectedFunc: "slowFunction",
			regressions: []model.Regression{
				{
					Diagnosis: &model.Diagnosis{
						ContributorCandidates: []model.Candidate{
							{FunctionName: "main.slowFunction"},
							{FunctionName: "main.otherFunc"},
						},
					},
				},
			},
			want: true,
		},
		{
			name:         "match in third candidate",
			expectedFunc: "targetFunc",
			regressions: []model.Regression{
				{
					Diagnosis: &model.Diagnosis{
						ContributorCandidates: []model.Candidate{
							{FunctionName: "main.fastFunc"},
							{FunctionName: "main.mediumFunc"},
							{FunctionName: "pkg.targetFunc"},
						},
					},
				},
			},
			want: true,
		},
		{
			name:         "no match in top 3",
			expectedFunc: "targetFunc",
			regressions: []model.Regression{
				{
					Diagnosis: &model.Diagnosis{
						ContributorCandidates: []model.Candidate{
							{FunctionName: "main.func1"},
							{FunctionName: "main.func2"},
							{FunctionName: "main.func3"},
							{FunctionName: "main.targetFunc"},
						},
					},
				},
			},
			want: false,
		},
		{
			name:         "nil diagnosis",
			expectedFunc: "slowFunction",
			regressions: []model.Regression{
				{
					Diagnosis: nil,
				},
			},
			want: false,
		},
		{
			name:         "empty candidates",
			expectedFunc: "slowFunction",
			regressions: []model.Regression{
				{
					Diagnosis: &model.Diagnosis{
						ContributorCandidates: []model.Candidate{},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagnosisMatches(tt.expectedFunc, tt.regressions)
			if got != tt.want {
				t.Errorf("diagnosisMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateReport(t *testing.T) {
	data := ReportData{
		Scenarios: []ScenarioResult{
			{
				Scenario: Scenario{
					ID:          "test-scenario",
					Description: "Test scenario for unit testing",
					Trials:      20,
					Expect: ExpectedOutcome{
						Verdict:          VerdictFail,
						ExpectedFunction: "slowFunc",
					},
				},
				Metrics: Metrics{
					TotalTrials:     20,
					SuccessfulRuns:  18,
					DiagnosisTrials: 18,
					DiagnosisHits:   15,
				},
				FailureExamples: []string{
					"trial_01: expected fail, got pass",
					"trial_05: expected fail, got warn",
				},
			},
		},
		Environment: EnvironmentFingerprint{
			OS:      "linux",
			Arch:    "amd64",
			GoVer:   "go1.21.0",
			NumCPU:  8,
			Hostnam: "test-host",
		},
	}

	report := GenerateReport(data)

	// Check that report contains key sections
	requiredSections := []string{
		"# PerfGuard Evaluation Results",
		"## Environment",
		"## Summary",
		"## Scenario Results",
		"### test-scenario",
		"**Detection rate:**",
		"**Diagnosis hit rate:**",
		"**Failure examples:**",
		"## False Positive Analysis",
	}

	for _, section := range requiredSections {
		if !strings.Contains(report, section) {
			t.Errorf("Report missing section: %s", section)
		}
	}

	// Check environment details
	if !strings.Contains(report, "OS: linux") {
		t.Error("Report missing OS information")
	}
	if !strings.Contains(report, "Arch: amd64") {
		t.Error("Report missing Arch information")
	}
	if !strings.Contains(report, "Go: go1.21.0") {
		t.Error("Report missing Go version")
	}

	// Check metrics are formatted
	if !strings.Contains(report, "90.0%") {
		t.Error("Report missing detection rate percentage")
	}
}

func TestCaptureEnvironment(t *testing.T) {
	env := CaptureEnvironment()

	if env.OS == "" {
		t.Error("OS not captured")
	}
	if env.Arch == "" {
		t.Error("Arch not captured")
	}
	if env.GoVer == "" {
		t.Error("Go version not captured")
	}
	if env.NumCPU <= 0 {
		t.Error("NumCPU not captured or invalid")
	}
}

func TestEnvironmentFingerprint_Timestamp(t *testing.T) {
	env := EnvironmentFingerprint{
		OS:      "linux",
		Arch:    "amd64",
		GoVer:   "go1.21.0",
		NumCPU:  8,
		Hostnam: "test-machine",
	}

	ts := env.Timestamp()
	expected := "linux/amd64, Go go1.21.0, 8 cores"

	if ts != expected {
		t.Errorf("Timestamp() = %q, want %q", ts, expected)
	}
}

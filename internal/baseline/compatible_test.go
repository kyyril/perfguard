package baseline

import (
	"strings"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/model"
)

func makeBaselineRun() model.Run {
	return model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            "base-run",
		Side:          model.SideBase,
		CreatedAt:     time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC),
		Git: model.GitInfo{
			SHA: "aaaa1111",
			Ref: "refs/heads/main",
		},
		Env: model.Environment{
			OS:         "linux",
			Arch:       "amd64",
			GoVersion:  "go1.24.0",
			CPUModel:   "AMD EPYC 7763",
			GOMAXPROCS: 16,
		},
		Params: model.RunParams{
			Count:     10,
			Benchtime: "1s",
			CPU:       16,
			Include:   "BenchmarkCompute",
			Mode:      "stored",
		},
	}
}

func TestCompatible_Table(t *testing.T) {
	tests := []struct {
		name          string
		mutateHead    func(r *model.Run)
		wantCompat    bool
		wantReason    string
		expectSubstr  string
		expectedDiffs int
	}{
		{
			name:          "identical environment and parameters",
			mutateHead:    func(_ *model.Run) {},
			wantCompat:    true,
			wantReason:    "",
			expectedDiffs: 0,
		},
		{
			name: "OS mismatch",
			mutateHead: func(r *model.Run) {
				r.Env.OS = "darwin"
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "OS mismatch: base linux vs head darwin",
			expectedDiffs: 1,
		},
		{
			name: "Arch mismatch",
			mutateHead: func(r *model.Run) {
				r.Env.Arch = "arm64"
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "Arch mismatch: base amd64 vs head arm64",
			expectedDiffs: 1,
		},
		{
			name: "CPU model mismatch",
			mutateHead: func(r *model.Run) {
				r.Env.CPUModel = "Intel Xeon Platinum"
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "CPU model mismatch",
			expectedDiffs: 1,
		},
		{
			name: "GOMAXPROCS mismatch",
			mutateHead: func(r *model.Run) {
				r.Env.GOMAXPROCS = 8
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "GOMAXPROCS mismatch: base 16 vs head 8",
			expectedDiffs: 1,
		},
		{
			name: "Go minor version mismatch",
			mutateHead: func(r *model.Run) {
				r.Env.GoVersion = "go1.25.0"
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "Go minor version mismatch: base 1.24 vs head 1.25",
			expectedDiffs: 1,
		},
		{
			name: "Go patch version match",
			mutateHead: func(r *model.Run) {
				r.Env.GoVersion = "go1.24.2"
			},
			wantCompat:    true,
			wantReason:    "",
			expectedDiffs: 0,
		},
		{
			name: "Benchtime mismatch",
			mutateHead: func(r *model.Run) {
				r.Params.Benchtime = "2s"
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "Benchtime mismatch: base 1s vs head 2s",
			expectedDiffs: 1,
		},
		{
			name: "Count mismatch",
			mutateHead: func(r *model.Run) {
				r.Params.Count = 20
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "Count mismatch: base 10 vs head 20",
			expectedDiffs: 1,
		},
		{
			name: "CPU procs flag mismatch",
			mutateHead: func(r *model.Run) {
				r.Params.CPU = 4
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "CPU procs mismatch: base 16 vs head 4",
			expectedDiffs: 1,
		},
		{
			name: "Include regex filter mismatch",
			mutateHead: func(r *model.Run) {
				r.Params.Include = "BenchmarkOther"
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "Include filter mismatch",
			expectedDiffs: 1,
		},
		{
			name: "Exclude list difference produces parameter hash mismatch",
			mutateHead: func(r *model.Run) {
				r.Params.Exclude = []string{"Slow"}
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "Parameter hash mismatch",
			expectedDiffs: 1,
		},
		{
			name: "Multiple mismatches reported together",
			mutateHead: func(r *model.Run) {
				r.Env.OS = "windows"
				r.Env.GOMAXPROCS = 32
				r.Params.Count = 5
			},
			wantCompat:    false,
			wantReason:    ReasonEnvMismatch,
			expectSubstr:  "OS mismatch",
			expectedDiffs: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := makeBaselineRun()
			head := makeBaselineRun()
			head.ID = "head-run"
			head.Side = model.SideHead
			head.Git.SHA = "bbbb2222"
			tt.mutateHead(&head)

			res := Compatible(base, head)
			if res.Compatible != tt.wantCompat {
				t.Errorf("Compatible = %v, want %v", res.Compatible, tt.wantCompat)
			}
			if res.Reason != tt.wantReason {
				t.Errorf("Reason = %q, want %q", res.Reason, tt.wantReason)
			}
			if len(res.Diffs) != tt.expectedDiffs {
				t.Errorf("Diffs count = %d, want %d (diffs: %v)", len(res.Diffs), tt.expectedDiffs, res.Diffs)
			}
			if tt.expectSubstr != "" {
				found := false
				for _, d := range res.Diffs {
					if strings.Contains(d, tt.expectSubstr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected diff containing %q, got: %v", tt.expectSubstr, res.Diffs)
				}
			}
		})
	}
}

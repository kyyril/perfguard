package profile_test

import (
	"path/filepath"
	"testing"

	pprof "github.com/google/pprof/profile"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/profile"
)

func TestMatchesChangedFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sourceFile string
		changed    []string
		want       bool
	}{
		{
			name:       "exact match",
			sourceFile: "pkg/work.go",
			changed:    []string{"pkg/work.go", "pkg/other.go"},
			want:       true,
		},
		{
			name:       "suffix match with unix path",
			sourceFile: "/home/runner/work/perfguard/internal/stats/stats.go",
			changed:    []string{"internal/stats/stats.go"},
			want:       true,
		},
		{
			name:       "suffix match with windows path",
			sourceFile: "D:\\repo\\perfguard\\internal\\stats\\stats.go",
			changed:    []string{"internal/stats/stats.go"},
			want:       true,
		},
		{
			name:       "different file not matched",
			sourceFile: "/repo/internal/stats/stats.go",
			changed:    []string{"internal/runner/execute.go", "README.md"},
			want:       false,
		},
		{
			name:       "partial filename collision should not match without slash",
			sourceFile: "/repo/my_stats.go",
			changed:    []string{"stats.go"},
			want:       false,
		},
		{
			name:       "empty source file",
			sourceFile: "",
			changed:    []string{"stats.go"},
			want:       false,
		},
		{
			name:       "empty changed files",
			sourceFile: "stats.go",
			changed:    nil,
			want:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Skip Windows-specific path test on non-Windows platforms
			if tc.name == "suffix match with windows path" && filepath.Separator != '\\' {
				t.Skip("Windows path test skipped on non-Windows platform")
			}
			got := profile.MatchesChangedFile(tc.sourceFile, tc.changed)
			if got != tc.want {
				t.Errorf("MatchesChangedFile(%q, %v) = %v; want %v", tc.sourceFile, tc.changed, got, tc.want)
			}
		})
	}
}

func TestExtractFunctionFiles(t *testing.T) {
	t.Parallel()

	prof := &pprof.Profile{
		Function: []*pprof.Function{
			{ID: 1, Name: "pkg.Foo", Filename: "pkg/foo.go"},
			{ID: 2, Name: "pkg.Bar", Filename: "pkg/bar.go"},
			{ID: 3, Name: "", Filename: "pkg/empty.go"},
		},
	}

	files := profile.ExtractFunctionFiles(prof)
	if files["pkg.Foo"] != "pkg/foo.go" {
		t.Errorf("expected pkg/foo.go, got %q", files["pkg.Foo"])
	}
	if files["pkg.Bar"] != "pkg/bar.go" {
		t.Errorf("expected pkg/bar.go, got %q", files["pkg.Bar"])
	}
	if _, ok := files[""]; ok {
		t.Errorf("expected empty function name to be skipped")
	}

	// nil profile
	nilFiles := profile.ExtractFunctionFiles(nil)
	if len(nilFiles) != 0 {
		t.Errorf("expected empty map for nil profile, got %v", nilFiles)
	}
}

func TestAnnotateTouched(t *testing.T) {
	t.Parallel()

	candidates := []model.Candidate{
		{Function: "codec.(*Encoder).grow", FlatDelta: 9.8, CumDelta: 14.1, Touched: false},
		{Function: "runtime.mallocgc", FlatDelta: 6.2, CumDelta: 8.4, Touched: false},
		{Function: "unknown.Func", FlatDelta: 1.0, CumDelta: 1.0, Touched: false},
	}

	fnFiles := map[string]string{
		"codec.(*Encoder).grow": "/home/runner/repo/codec/encoder.go",
		"runtime.mallocgc":      "/usr/local/go/src/runtime/malloc.go",
		"unknown.Func":          "",
	}

	changedFiles := []string{
		"codec/encoder.go",
		"docs/ARCHITECTURE.md",
	}

	annotated := profile.AnnotateTouched(candidates, fnFiles, changedFiles)

	if len(annotated) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(annotated))
	}

	// codec.(*Encoder).grow should be touched (changed by PR)
	if !annotated[0].Touched {
		t.Errorf("expected candidate 0 to be marked touched=true")
	}

	// runtime.mallocgc should NOT be touched
	if annotated[1].Touched {
		t.Errorf("expected candidate 1 to be marked touched=false")
	}

	// unknown.Func should NOT be touched
	if annotated[2].Touched {
		t.Errorf("expected candidate 2 to be marked touched=false")
	}

	// Original slice should not be mutated
	if candidates[0].Touched {
		t.Errorf("original candidates slice was mutated")
	}
}

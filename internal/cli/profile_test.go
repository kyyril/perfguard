package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pprof "github.com/google/pprof/profile"

	"github.com/kyyril/perfguard/internal/cli"
	"github.com/kyyril/perfguard/internal/model"
)

// writeFixtureProfile writes a minimal synthetic pprof profile to a .pb.gz file.
// The profile has one sample type and one sample attributed to funcName with the given value.
func writeFixtureProfile(t *testing.T, dir, name, funcName string, flatValue int64) string {
	t.Helper()
	fn := &pprof.Function{
		ID:       1,
		Name:     funcName,
		Filename: "bench_test.go",
	}
	loc := &pprof.Location{
		ID:   1,
		Line: []pprof.Line{{Function: fn}},
	}
	prof := &pprof.Profile{
		SampleType: []*pprof.ValueType{
			{Type: "samples", Unit: "count"},
		},
		PeriodType: &pprof.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Sample: []*pprof.Sample{
			{Location: []*pprof.Location{loc}, Value: []int64{flatValue}},
		},
		Function: []*pprof.Function{fn},
		Location: []*pprof.Location{loc},
	}

	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture profile %s: %v", path, err)
	}
	defer f.Close()
	// Profile.Write gzip-compresses internally; do not wrap in another gzip.Writer.
	if err := prof.Write(f); err != nil {
		t.Fatalf("write profile %s: %v", path, err)
	}
	return path
}

func TestProfileDiff_TextFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	basePath := writeFixtureProfile(t, dir, "base.pb.gz", "BenchmarkFoo", 100)
	headPath := writeFixtureProfile(t, dir, "head.pb.gz", "BenchmarkFoo", 200)

	root := cli.NewRootCommand()
	var stdout bytes.Buffer
	root.SetOut(&stdout)

	root.SetArgs([]string{
		"profile", "diff",
		basePath, headPath,
		"--ops-base", "100",
		"--ops-head", "100",
		"--format", "text",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("profile diff failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "BenchmarkFoo") {
		t.Errorf("expected output to contain 'BenchmarkFoo', got:\n%s", out)
	}
	// head flat = 200/100=2.0, base flat = 100/100=1.0, delta = +1.0
	if !strings.Contains(out, "+1.0000") {
		t.Errorf("expected flat delta +1.0000 in output, got:\n%s", out)
	}
}

func TestProfileDiff_JSONFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	basePath := writeFixtureProfile(t, dir, "base.pb.gz", "BenchmarkBar", 50)
	headPath := writeFixtureProfile(t, dir, "head.pb.gz", "BenchmarkBar", 150)

	root := cli.NewRootCommand()
	var stdout bytes.Buffer
	root.SetOut(&stdout)

	root.SetArgs([]string{
		"profile", "diff",
		basePath, headPath,
		"--ops-base", "50",
		"--ops-head", "50",
		"--format", "json",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("profile diff json failed: %v", err)
	}

	type output struct {
		SampleType string            `json:"sample_type"`
		Candidates []model.Candidate `json:"candidates"`
	}
	var got output
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput:\n%s", err, stdout.String())
	}
	if got.SampleType == "" {
		t.Errorf("expected non-empty sample_type in JSON output")
	}
	if len(got.Candidates) == 0 {
		t.Errorf("expected at least one candidate in JSON output")
	}
	// head flat = 150/50=3.0, base flat = 50/50=1.0, delta = +2.0
	found := false
	for _, c := range got.Candidates {
		if c.Function == "BenchmarkBar" {
			found = true
			// Allow small float tolerance
			if c.FlatDelta < 1.99 || c.FlatDelta > 2.01 {
				t.Errorf("expected FlatDelta ~2.0, got %f", c.FlatDelta)
			}
		}
	}
	if !found {
		t.Errorf("expected BenchmarkBar in candidates, got: %+v", got.Candidates)
	}
}

func TestProfileDiff_NoDifferences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	basePath := writeFixtureProfile(t, dir, "base.pb.gz", "BenchmarkStable", 100)
	headPath := writeFixtureProfile(t, dir, "head.pb.gz", "BenchmarkStable", 100)

	root := cli.NewRootCommand()
	var stdout bytes.Buffer
	root.SetOut(&stdout)

	root.SetArgs([]string{
		"profile", "diff",
		basePath, headPath,
		"--ops-base", "100",
		"--ops-head", "100",
		"--format", "text",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("profile diff failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "No significant differences") {
		t.Errorf("expected 'No significant differences' for identical profiles, got:\n%s", out)
	}
}

func TestProfileDiff_MissingArgs(t *testing.T) {
	t.Parallel()

	root := cli.NewRootCommand()
	root.SetArgs([]string{"profile", "diff", "only-one-arg.pb.gz"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when only one argument is provided")
	}
}

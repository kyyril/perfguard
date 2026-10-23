//go:build integration

package profile_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/profile"
	"github.com/kyyril/perfguard/internal/runner"
)

func TestIntegration_ValidateNPlusOneNormalizationAssumption(t *testing.T) {
	ctx := context.Background()
	fixtureDir, err := filepath.Abs("../../testdata/fixtures/benchmod")
	if err != nil {
		t.Fatalf("failed to resolve fixture directory: %v", err)
	}

	r := runner.New()
	outDir := t.TempDir()

	binaries, err := r.Build(ctx, runner.BuildSpec{
		WorkDir:  fixtureDir,
		Packages: []string{"./pkgwithtests"},
		OutDir:   outDir,
	})
	if err != nil {
		t.Fatalf("failed to build test binary: %v", err)
	}
	defer func() {
		for _, b := range binaries {
			_ = b.Remove()
		}
	}()

	var testBin runner.TestBinary
	for _, b := range binaries {
		if strings.Contains(b.Package, "pkgwithtests") {
			testBin = b
			break
		}
	}
	if testBin.Path == "" {
		t.Fatalf("pkgwithtests binary not found: %+v", binaries)
	}

	tests := []struct {
		n            int
		expectedIter string // "COUNTED:N+1"
	}{
		{n: 100, expectedIter: "COUNTED:101"},
		{n: 500, expectedIter: "COUNTED:501"},
	}

	for _, tc := range tests {
		t.Run(tc.expectedIter, func(t *testing.T) {
			raw, err := r.Execute(ctx, testBin, runner.ExecSpec{
				BenchRegex: "^BenchmarkCounted$",
				Benchtime:  profile.BuildBenchtime(int64(tc.n)),
				Timeout:    10 * time.Second,
			})
			if err != nil {
				t.Fatalf("execute failed: %v, stdout: %s, stderr: %s", err, raw.Stdout, raw.Stderr)
			}

			combined := string(raw.Stdout) + "\n" + string(raw.Stderr)
			if !strings.Contains(combined, tc.expectedIter) {
				t.Errorf("expected output to contain %q proving N+1 execution model; got:\n%s", tc.expectedIter, combined)
			}
		})
	}
}

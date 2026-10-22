//go:build integration

package profile_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pprof "github.com/google/pprof/profile"
	"github.com/kyyril/perfguard/internal/profile"
	"github.com/kyyril/perfguard/internal/runner"
)

type runnerProfileAdapter struct {
	r *runner.Runner
}

func (a *runnerProfileAdapter) ExecuteProfile(ctx context.Context, spec profile.ExecSpec) error {
	bin := runner.TestBinary{
		Path: spec.BinaryPath,
		Dir:  spec.WorkDir,
	}
	runnerSpec := runner.ExecSpec{
		BenchRegex:    spec.BenchRegex,
		Benchtime:     spec.Benchtime,
		Timeout:       spec.Timeout,
		ExtraEnv:      spec.ExtraEnv,
		CPUProfile:    spec.CPUProfile,
		MemProfile:    spec.MemProfile,
		MutexProfile:  spec.MutexProfile,
		MutexFraction: spec.MutexFraction,
		BlockProfile:  spec.BlockProfile,
		BlockRate:     spec.BlockRate,
	}

	_, err := a.r.Execute(ctx, bin, runnerSpec)
	return err
}

func TestIntegration_ProfileCollectionProducesValidProfiles(t *testing.T) {
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
		t.Fatalf("pkgwithtests binary not found in build output: %+v", binaries)
	}

	adapter := &runnerProfileAdapter{r: r}
	profileOutDir := t.TempDir()

	opts := profile.CollectOptions{
		BinaryPath: testBin.Path,
		WorkDir:    testBin.Dir,
		Benchmark:  "BenchmarkMixed",
		Iterations: 1000,
		Types:      []string{profile.TypeCPU, profile.TypeAllocs, profile.TypeMutex, profile.TypeBlock},
		Runs:       1,
		OutputDir:  profileOutDir,
		Prefix:     "head",
		Timeout:    30 * time.Second,
	}

	result, err := profile.Collect(ctx, adapter, opts)
	if err != nil {
		t.Fatalf("profile.Collect failed: %v", err)
	}

	if len(result.Files) != 4 {
		t.Fatalf("expected 4 profile files, got %d", len(result.Files))
	}

	// Verify each file is a valid .pb.gz readable by pprof
	for _, pf := range result.Files {
		t.Run(pf.Type, func(t *testing.T) {
			if !strings.HasSuffix(pf.Path, ".pb.gz") {
				t.Errorf("expected .pb.gz extension, got: %s", pf.Path)
			}

			f, err := os.Open(pf.Path)
			if err != nil {
				t.Fatalf("failed opening profile %s: %v", pf.Path, err)
			}
			defer f.Close()

			parsed, err := pprof.Parse(f)
			if err != nil {
				t.Fatalf("failed parsing %s profile with pprof.Parse: %v", pf.Type, err)
			}

			if parsed == nil {
				t.Fatalf("parsed %s profile is nil", pf.Type)
			}
			if len(parsed.SampleType) == 0 {
				t.Errorf("parsed %s profile has no sample types", pf.Type)
			}

			switch pf.Type {
			case profile.TypeAllocs:
				hasAllocSpace := false
				for _, st := range parsed.SampleType {
					if st.Type == "alloc_space" {
						hasAllocSpace = true
						break
					}
				}
				if !hasAllocSpace {
					t.Errorf("allocs profile missing alloc_space sample type: %+v", parsed.SampleType)
				}
			case profile.TypeCPU:
				hasSamples := false
				for _, st := range parsed.SampleType {
					if st.Type == "samples" || st.Type == "cpu" {
						hasSamples = true
						break
					}
				}
				if !hasSamples {
					t.Errorf("cpu profile missing expected sample type: %+v", parsed.SampleType)
				}
			}
		})
	}
}

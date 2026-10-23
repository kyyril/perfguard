package profile_test

import (
	"strings"
	"testing"

	pprof "github.com/google/pprof/profile"
	"github.com/kyyril/perfguard/internal/profile"
)

func TestCheckSampleCount(t *testing.T) {
	t.Parallel()

	prof := &pprof.Profile{
		Sample: []*pprof.Sample{
			{Value: []int64{100}},
			{Value: []int64{200}},
		},
	}

	// 1. Samples below minimum
	warn := profile.CheckSampleCount(prof, 200)
	if !strings.Contains(warn, "warning: profile too short (2 samples, minimum 200)") {
		t.Errorf("expected short profile warning, got: %q", warn)
	}

	// 2. Samples meeting minimum
	ok := profile.CheckSampleCount(prof, 2)
	if ok != "" {
		t.Errorf("expected empty warning when sample count meets minimum, got: %q", ok)
	}

	// 3. Nil profile
	nilWarn := profile.CheckSampleCount(nil, 100)
	if !strings.Contains(nilWarn, "warning: profile is nil") {
		t.Errorf("expected nil profile warning, got: %q", nilWarn)
	}

	// 4. Zero or negative minSamples
	noCheck := profile.CheckSampleCount(prof, 0)
	if noCheck != "" {
		t.Errorf("expected empty string when minSamples is 0, got: %q", noCheck)
	}
}

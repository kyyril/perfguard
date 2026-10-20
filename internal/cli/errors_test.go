package cli_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kyyril/perfguard/internal/cli"
	"github.com/kyyril/perfguard/internal/model"
)

func TestExitCode_Mapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error yields pass",
			err:      nil,
			expected: cli.ExitPass,
		},
		{
			name:     "sentinel ErrConfig yields 2",
			err:      cli.ErrConfig,
			expected: cli.ExitConfig,
		},
		{
			name:     "wrapped ErrConfig yields 2",
			err:      cli.WrapConfig(errors.New("invalid yaml")),
			expected: cli.ExitConfig,
		},
		{
			name:     "nested fmt.Errorf wrapping ErrConfig yields 2",
			err:      fmt.Errorf("reading file: %w", cli.ErrConfig),
			expected: cli.ExitConfig,
		},
		{
			name:     "sentinel ErrExecution yields 3",
			err:      cli.ErrExecution,
			expected: cli.ExitExecution,
		},
		{
			name:     "wrapped ErrExecution yields 3",
			err:      cli.WrapExecution(errors.New("benchmark panicked")),
			expected: cli.ExitExecution,
		},
		{
			name:     "nested fmt.Errorf wrapping ErrExecution yields 3",
			err:      fmt.Errorf("running binary: %w", cli.ErrExecution),
			expected: cli.ExitExecution,
		},
		{
			name:     "arbitrary error defaults to 3",
			err:      errors.New("unknown error"),
			expected: cli.ExitExecution,
		},
		{
			name: "verdict fail yields 1",
			err: &cli.VerdictError{
				Verdict: model.VerdictFail,
			},
			expected: cli.ExitFail,
		},
		{
			name: "verdict warn with fail-on fail yields 0",
			err: &cli.VerdictError{
				Verdict: model.VerdictWarn,
				FailOn:  "fail",
			},
			expected: cli.ExitPass,
		},
		{
			name: "verdict warn with fail-on warn yields 1",
			err: &cli.VerdictError{
				Verdict: model.VerdictWarn,
				FailOn:  "warn",
			},
			expected: cli.ExitFail,
		},
		{
			name: "verdict inconclusive without strict yields 0",
			err: &cli.VerdictError{
				Verdict: model.VerdictInconclusive,
				Strict:  false,
			},
			expected: cli.ExitPass,
		},
		{
			name: "verdict inconclusive with strict yields 4",
			err: &cli.VerdictError{
				Verdict: model.VerdictInconclusive,
				Strict:  true,
			},
			expected: cli.ExitInconclusive,
		},
		{
			name: "verdict pass yields 0",
			err: &cli.VerdictError{
				Verdict: model.VerdictPass,
			},
			expected: cli.ExitPass,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := cli.ExitCode(tc.err)
			if got != tc.expected {
				t.Errorf("ExitCode(%v) = %d; want %d", tc.err, got, tc.expected)
			}
		})
	}
}

func TestWrappedErrors_HelperMethods(t *testing.T) {
	t.Parallel()

	if err := cli.WrapConfig(nil); err != nil {
		t.Errorf("expected nil for WrapConfig(nil), got: %v", err)
	}
	if err := cli.WrapExecution(nil); err != nil {
		t.Errorf("expected nil for WrapExecution(nil), got: %v", err)
	}

	cfgErr := cli.WrapConfig(errors.New("bad field"))
	if !errors.Is(cfgErr, cli.ErrConfig) {
		t.Errorf("expected errors.Is(cfgErr, ErrConfig) to be true")
	}
	if cfgErr.Error() != "bad field" {
		t.Errorf("expected error string 'bad field', got: %s", cfgErr.Error())
	}

	execErr := cli.WrapExecution(errors.New("timeout reached"))
	if !errors.Is(execErr, cli.ErrExecution) {
		t.Errorf("expected errors.Is(execErr, ErrExecution) to be true")
	}
	if execErr.Error() != "timeout reached" {
		t.Errorf("expected error string 'timeout reached', got: %s", execErr.Error())
	}
}

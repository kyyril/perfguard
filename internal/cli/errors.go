package cli

import (
	"errors"
	"fmt"

	"github.com/kyyril/perfguard/internal/model"
)

// Exit codes per specification.
const (
	ExitPass         = 0
	ExitFail         = 1
	ExitConfig       = 2
	ExitExecution    = 3
	ExitInconclusive = 4
)

// Sentinel errors for exit code mapping.
var (
	ErrConfig    = errors.New("configuration error")
	ErrExecution = errors.New("execution error")
)

// ErrorCoder allows custom error types to provide specific exit codes.
type ErrorCoder interface {
	ExitCode() int
}

type wrappedError struct {
	sentinel error
	err      error
}

func (w *wrappedError) Error() string {
	if w.err == nil {
		return w.sentinel.Error()
	}
	return w.err.Error()
}

func (w *wrappedError) Unwrap() error {
	return w.err
}

func (w *wrappedError) Is(target error) bool {
	return target == w.sentinel
}

// WrapConfig wraps an error so it matches ErrConfig.
func WrapConfig(err error) error {
	if err == nil {
		return nil
	}
	return &wrappedError{sentinel: ErrConfig, err: err}
}

// WrapExecution wraps an error so it matches ErrExecution.
func WrapExecution(err error) error {
	if err == nil {
		return nil
	}
	return &wrappedError{sentinel: ErrExecution, err: err}
}

// VerdictError represents an exit condition derived from a comparison verdict.
type VerdictError struct {
	Verdict model.Verdict
	Strict  bool
	FailOn  string
}

func (e *VerdictError) Error() string {
	return fmt.Sprintf("verdict: %s", e.Verdict)
}

func (e *VerdictError) ExitCode() int {
	switch e.Verdict {
	case model.VerdictFail:
		return ExitFail
	case model.VerdictWarn:
		if e.FailOn == "warn" {
			return ExitFail
		}
		return ExitPass
	case model.VerdictInconclusive:
		if e.Strict {
			return ExitInconclusive
		}
		return ExitPass
	case model.VerdictPass:
		return ExitPass
	default:
		return ExitPass
	}
}

// ExitCode determines the process exit code for a given error.
func ExitCode(err error) int {
	if err == nil {
		return ExitPass
	}

	var coder ErrorCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}

	if errors.Is(err, ErrConfig) {
		return ExitConfig
	}
	if errors.Is(err, ErrExecution) {
		return ExitExecution
	}

	return ExitExecution
}

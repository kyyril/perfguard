package app

import (
	"context"

	"github.com/kyyril/perfguard/internal/gitx"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/runner"
)

// BenchRunner defines the interface for building and running benchmark binaries.
type BenchRunner interface {
	Build(ctx context.Context, spec runner.BuildSpec) ([]runner.TestBinary, error)
	Execute(ctx context.Context, bin runner.TestBinary, spec runner.ExecSpec) (runner.RawOutput, error)
	RunRounds(ctx context.Context, spec runner.RoundSpec) (runner.RoundResults, error)
}

// VCS defines the interface for repository queries.
type VCS interface {
	CurrentInfo(ctx context.Context) (model.GitInfo, error)
	Resolve(ctx context.Context, ref string) (gitx.Commit, error)
}

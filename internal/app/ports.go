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

// Worktree defines the interface for managing a temporary checkout worktree.
type Worktree interface {
	Location() string
	Remove(ctx context.Context) error
}

// VCS defines the interface for repository queries and worktree management.
type VCS interface {
	CurrentInfo(ctx context.Context) (model.GitInfo, error)
	Resolve(ctx context.Context, ref string) (gitx.Commit, error)
	CreateWorktree(ctx context.Context, commit gitx.Commit) (Worktree, error)
}

type gitAdapter struct {
	client *gitx.Client
}

func (g *gitAdapter) CurrentInfo(ctx context.Context) (model.GitInfo, error) {
	return g.client.CurrentInfo(ctx)
}

func (g *gitAdapter) Resolve(ctx context.Context, ref string) (gitx.Commit, error) {
	return g.client.Resolve(ctx, ref)
}

func (g *gitAdapter) CreateWorktree(ctx context.Context, commit gitx.Commit) (Worktree, error) {
	return g.client.AddWorktree(ctx, commit)
}

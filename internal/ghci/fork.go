package ghci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/report"
)

// EventInfo represents relevant GitHub event metadata parsed from GITHUB_EVENT_PATH.
type EventInfo struct {
	IsPR     bool
	IsFork   bool
	PRNumber int
	Owner    string
	Repo     string
	HeadSHA  string
	BaseRef  string
}

type rawEventPayload struct {
	Number      int `json:"number"`
	PullRequest *struct {
		Number int `json:"number"`
		Head   struct {
			SHA  string `json:"sha"`
			Repo *struct {
				Fork     bool   `json:"fork"`
				FullName string `json:"full_name"`
			} `json:"repo"`
		} `json:"head"`
		Base struct {
			Ref  string `json:"ref"`
			SHA  string `json:"sha"`
			Repo *struct {
				Fork     bool   `json:"fork"`
				FullName string `json:"full_name"`
				Owner    struct {
					Login string `json:"login"`
				} `json:"owner"`
				Name string `json:"name"`
			} `json:"repo"`
		} `json:"base"`
	} `json:"pull_request"`
	Repository *struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Fork     bool   `json:"fork"`
	} `json:"repository"`
}

// ParseEvent parses GitHub event JSON data.
func ParseEvent(data []byte) (*EventInfo, error) {
	if len(data) == 0 {
		return &EventInfo{}, nil
	}

	var raw rawEventPayload
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("ghci: parse event payload: %w", err)
	}

	info := &EventInfo{}

	if raw.PullRequest != nil {
		info.IsPR = true
		info.PRNumber = raw.PullRequest.Number
		if info.PRNumber == 0 {
			info.PRNumber = raw.Number
		}
		info.HeadSHA = raw.PullRequest.Head.SHA
		info.BaseRef = raw.PullRequest.Base.Ref

		// Determine if fork
		headRepo := raw.PullRequest.Head.Repo
		baseRepo := raw.PullRequest.Base.Repo

		if headRepo != nil && headRepo.Fork {
			info.IsFork = true
		} else if headRepo != nil && baseRepo != nil && headRepo.FullName != "" && baseRepo.FullName != "" {
			if !strings.EqualFold(headRepo.FullName, baseRepo.FullName) {
				info.IsFork = true
			}
		}

		if baseRepo != nil {
			info.Owner = baseRepo.Owner.Login
			info.Repo = baseRepo.Name
		}
	}

	// Fallback to repository field if owner/repo not set yet
	if raw.Repository != nil {
		if info.Owner == "" {
			info.Owner = raw.Repository.Owner.Login
		}
		if info.Repo == "" {
			info.Repo = raw.Repository.Name
		}
	}

	return info, nil
}

// ParseEventFile reads and parses the event JSON file at path.
// If path is empty, it uses the environment variable GITHUB_EVENT_PATH.
func ParseEventFile(path string) (*EventInfo, error) {
	if path == "" {
		path = os.Getenv("GITHUB_EVENT_PATH")
	}
	if path == "" {
		return &EventInfo{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ghci: read event file: %w", err)
	}

	return ParseEvent(data)
}

// PublishOptions controls publishing of reports to GitHub.
type PublishOptions struct {
	EventPath       string
	StepSummaryPath string
	Token           string
	Comment         bool
	StepSummary     bool
}

// PublishCheckResult coordinates publishing the check result to PR comments and step summary.
// If the pull request originates from a fork, comment publishing is skipped without error.
func PublishCheckResult(ctx context.Context, client Publisher, cmp model.Comparison, opts PublishOptions) error {
	info, err := ParseEventFile(opts.EventPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("ghci: read event: %w", err)
	}
	if info == nil {
		info = &EventInfo{}
	}

	// 1. Step summary
	if opts.StepSummary {
		summaryPath := opts.StepSummaryPath
		if summaryPath == "" {
			summaryPath = os.Getenv(EnvStepSummary)
		}
		if summaryPath != "" {
			if err := WriteStepSummaryFile(summaryPath, cmp); err != nil {
				return fmt.Errorf("ghci: write step summary: %w", err)
			}
		}
	}

	// 2. PR comment
	if opts.Comment {
		if info.IsFork {
			// AC-07: Fork PR skips comment write and job does not fail
			return nil
		}

		if !info.IsPR || info.PRNumber <= 0 || info.Owner == "" || info.Repo == "" {
			// Not running in a PR context, skip comment
			return nil
		}

		if client == nil {
			return errors.New("ghci: publisher client is required for comment")
		}

		var commentBody strings.Builder
		if err := report.RenderGitHub(&commentBody, cmp); err != nil {
			return fmt.Errorf("ghci: render comment: %w", err)
		}

		_, err := client.PublishComment(ctx, CommentParams{
			Owner:    info.Owner,
			Repo:     info.Repo,
			PRNumber: info.PRNumber,
			Body:     commentBody.String(),
		})
		if err != nil {
			return fmt.Errorf("ghci: publish pr comment: %w", err)
		}
	}

	return nil
}

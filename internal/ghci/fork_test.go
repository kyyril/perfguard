package ghci_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyyril/perfguard/internal/ghci"
)

type recordingPublisher struct {
	calls []ghci.CommentParams
}

func (r *recordingPublisher) PublishComment(_ context.Context, params ghci.CommentParams) (*ghci.CommentResult, error) {
	r.calls = append(r.calls, params)
	return &ghci.CommentResult{
		ID:     1,
		Action: "created",
		URL:    "https://github.com/test/repo/issues/1#issuecomment-1",
	}, nil
}

func TestParseEvent_ForkPR(t *testing.T) {
	t.Parallel()

	forkPayload := `{
		"pull_request": {
			"number": 42,
			"head": {
				"sha": "headsha123",
				"repo": {
					"fork": true,
					"full_name": "contributor/perfguard"
				}
			},
			"base": {
				"ref": "main",
				"sha": "basesha456",
				"repo": {
					"fork": false,
					"full_name": "kyyril/perfguard",
					"owner": {
						"login": "kyyril"
					},
					"name": "perfguard"
				}
			}
		}
	}`

	info, err := ghci.ParseEvent([]byte(forkPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !info.IsPR {
		t.Errorf("expected IsPR to be true")
	}
	if !info.IsFork {
		t.Errorf("expected IsFork to be true")
	}
	if info.PRNumber != 42 {
		t.Errorf("expected PRNumber 42, got %d", info.PRNumber)
	}
	if info.Owner != "kyyril" || info.Repo != "perfguard" {
		t.Errorf("expected owner kyyril and repo perfguard, got %s/%s", info.Owner, info.Repo)
	}
}

func TestParseEvent_FullNameMismatch(t *testing.T) {
	t.Parallel()

	payload := `{
		"pull_request": {
			"number": 10,
			"head": {
				"repo": {
					"fork": false,
					"full_name": "other-user/perfguard"
				}
			},
			"base": {
				"repo": {
					"fork": false,
					"full_name": "kyyril/perfguard",
					"owner": {"login": "kyyril"},
					"name": "perfguard"
				}
			}
		}
	}`

	info, err := ghci.ParseEvent([]byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !info.IsFork {
		t.Errorf("expected IsFork to be true when full_name differs")
	}
}

func TestParseEvent_InternalBranch(t *testing.T) {
	t.Parallel()

	payload := `{
		"pull_request": {
			"number": 15,
			"head": {
				"repo": {
					"fork": false,
					"full_name": "kyyril/perfguard"
				}
			},
			"base": {
				"repo": {
					"fork": false,
					"full_name": "kyyril/perfguard",
					"owner": {"login": "kyyril"},
					"name": "perfguard"
				}
			}
		}
	}`

	info, err := ghci.ParseEvent([]byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !info.IsPR {
		t.Errorf("expected IsPR to be true")
	}
	if info.IsFork {
		t.Errorf("expected IsFork to be false for internal branch")
	}
}

func TestPublishCheckResult_ForkPR_SkipsComment(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	eventFile := filepath.Join(tmpDir, "event.json")
	summaryFile := filepath.Join(tmpDir, "summary.md")

	forkPayload := `{
		"pull_request": {
			"number": 99,
			"head": {
				"repo": {
					"fork": true,
					"full_name": "external/perfguard"
				}
			},
			"base": {
				"repo": {
					"fork": false,
					"full_name": "kyyril/perfguard",
					"owner": {"login": "kyyril"},
					"name": "perfguard"
				}
			}
		}
	}`
	if err := os.WriteFile(eventFile, []byte(forkPayload), 0o600); err != nil {
		t.Fatalf("failed writing event file: %v", err)
	}

	rec := &recordingPublisher{}
	cmp := makeTestComparison()

	err := ghci.PublishCheckResult(context.Background(), rec, cmp, ghci.PublishOptions{
		EventPath:       eventFile,
		StepSummaryPath: summaryFile,
		Comment:         true,
		StepSummary:     true,
	})
	if err != nil {
		t.Fatalf("expected nil error on fork PR, got %v", err)
	}

	// AC-07: write API is NEVER called on fork PR
	if len(rec.calls) != 0 {
		t.Errorf("expected 0 publish comment calls for fork PR, got %d", len(rec.calls))
	}

	// AC-07: step summary is still filled
	content, err := os.ReadFile(summaryFile)
	if err != nil {
		t.Fatalf("failed reading step summary file: %v", err)
	}
	if !strings.Contains(string(content), "## PerfGuard: PASS") {
		t.Errorf("expected step summary to be written on fork PR")
	}
}

func TestPublishCheckResult_InternalPR_PublishesComment(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	eventFile := filepath.Join(tmpDir, "event.json")
	summaryFile := filepath.Join(tmpDir, "summary.md")

	internalPayload := `{
		"pull_request": {
			"number": 55,
			"head": {
				"repo": {
					"fork": false,
					"full_name": "kyyril/perfguard"
				}
			},
			"base": {
				"repo": {
					"fork": false,
					"full_name": "kyyril/perfguard",
					"owner": {"login": "kyyril"},
					"name": "perfguard"
				}
			}
		}
	}`
	if err := os.WriteFile(eventFile, []byte(internalPayload), 0o600); err != nil {
		t.Fatalf("failed writing event file: %v", err)
	}

	rec := &recordingPublisher{}
	cmp := makeTestComparison()

	err := ghci.PublishCheckResult(context.Background(), rec, cmp, ghci.PublishOptions{
		EventPath:       eventFile,
		StepSummaryPath: summaryFile,
		Comment:         true,
		StepSummary:     true,
	})
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Comment API was called once
	if len(rec.calls) != 1 {
		t.Fatalf("expected 1 publish comment call, got %d", len(rec.calls))
	}
	if rec.calls[0].Owner != "kyyril" || rec.calls[0].Repo != "perfguard" || rec.calls[0].PRNumber != 55 {
		t.Errorf("unexpected comment params: %+v", rec.calls[0])
	}

	// Step summary was also written
	content, err := os.ReadFile(summaryFile)
	if err != nil {
		t.Fatalf("failed reading step summary: %v", err)
	}
	if !strings.Contains(string(content), "## PerfGuard: PASS") {
		t.Errorf("expected step summary to be written")
	}
}

func TestParseEvent_EmptyAndErrors(t *testing.T) {
	t.Parallel()

	// Empty payload
	info, err := ghci.ParseEvent(nil)
	if err != nil {
		t.Fatalf("unexpected error on nil: %v", err)
	}
	if info.IsPR {
		t.Errorf("expected IsPR false on nil")
	}

	// Invalid json
	_, err = ghci.ParseEvent([]byte("{invalid"))
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}

	// Push event with only repository
	pushPayload := `{
		"repository": {
			"name": "perfguard",
			"owner": {"login": "kyyril"},
			"full_name": "kyyril/perfguard"
		}
	}`
	info, err = ghci.ParseEvent([]byte(pushPayload))
	if err != nil {
		t.Fatalf("unexpected error on push payload: %v", err)
	}
	if info.IsPR {
		t.Errorf("expected IsPR false on push event")
	}
	if info.Owner != "kyyril" || info.Repo != "perfguard" {
		t.Errorf("expected owner kyyril and repo perfguard, got %s/%s", info.Owner, info.Repo)
	}
}

func TestParseEventFile_PathHandling(t *testing.T) {
	// Empty path and unset env
	t.Setenv("GITHUB_EVENT_PATH", "")
	info, err := ghci.ParseEventFile("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsPR {
		t.Errorf("expected IsPR false")
	}

	// Non-existent path
	_, err = ghci.ParseEventFile("/path/does/not/exist/event.json")
	if err == nil {
		t.Fatal("expected error on missing file, got nil")
	}
}

func TestPublishCheckResult_NilClientError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	eventFile := filepath.Join(tmpDir, "event.json")

	payload := `{
		"pull_request": {
			"number": 1,
			"head": {"repo": {"full_name": "owner/repo"}},
			"base": {
				"repo": {
					"full_name": "owner/repo",
					"owner": {"login": "owner"},
					"name": "repo"
				}
			}
		}
	}`
	_ = os.WriteFile(eventFile, []byte(payload), 0o600)

	cmp := makeTestComparison()
	err := ghci.PublishCheckResult(context.Background(), nil, cmp, ghci.PublishOptions{
		EventPath: eventFile,
		Comment:   true,
	})
	if err == nil {
		t.Fatal("expected error when client is nil and Comment is true, got nil")
	}
}

func TestPublishCheckResult_NonPREvent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	eventFile := filepath.Join(tmpDir, "event.json")
	_ = os.WriteFile(eventFile, []byte(`{"ref": "refs/heads/main"}`), 0o600)

	rec := &recordingPublisher{}
	cmp := makeTestComparison()

	err := ghci.PublishCheckResult(context.Background(), rec, cmp, ghci.PublishOptions{
		EventPath: eventFile,
		Comment:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error on push event: %v", err)
	}
	if len(rec.calls) != 0 {
		t.Errorf("expected 0 comment calls on non-PR event, got %d", len(rec.calls))
	}
}

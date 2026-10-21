package ghci_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kyyril/perfguard/internal/ghci"
	"github.com/kyyril/perfguard/internal/report"
)

type mockComment struct {
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
}

type mockGitHubServer struct {
	mu          sync.Mutex
	nextID      int64
	comments    map[int64]*mockComment
	issueOrders map[int][]int64
}

func newMockGitHubServer() *mockGitHubServer {
	return &mockGitHubServer{
		nextID:      1,
		comments:    make(map[int64]*mockComment),
		issueOrders: make(map[int][]int64),
	}
}

func (m *mockGitHubServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Path: /repos/{owner}/{repo}/issues/{issue_number}/comments
	if strings.Contains(r.URL.Path, "/issues/") && strings.HasSuffix(r.URL.Path, "/comments") {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 6 && parts[0] == "repos" && parts[3] == "issues" && parts[5] == "comments" {
			prNum, _ := strconv.Atoi(parts[4])

			if r.Method == http.MethodGet {
				perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
				if perPage <= 0 {
					perPage = 30
				}
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				if page <= 0 {
					page = 1
				}

				allIDs := m.issueOrders[prNum]
				start := (page - 1) * perPage
				if start > len(allIDs) {
					start = len(allIDs)
				}
				end := start + perPage
				if end > len(allIDs) {
					end = len(allIDs)
				}

				var list []*mockComment
				for _, id := range allIDs[start:end] {
					list = append(list, m.comments[id])
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(list)
				return
			}

			if r.Method == http.MethodPost {
				var req struct {
					Body string `json:"body"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				id := m.nextID
				m.nextID++
				comment := &mockComment{
					ID:      id,
					Body:    req.Body,
					HTMLURL: fmt.Sprintf("https://github.com/test/repo/issues/%d#issuecomment-%d", prNum, id),
				}
				m.comments[id] = comment
				m.issueOrders[prNum] = append(m.issueOrders[prNum], id)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(comment)
				return
			}
		}
	}

	// Path: /repos/{owner}/{repo}/issues/comments/{comment_id}
	if strings.Contains(r.URL.Path, "/issues/comments/") {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) == 6 && parts[3] == "issues" && parts[4] == "comments" {
			cID, _ := strconv.ParseInt(parts[5], 10, 64)
			comment, ok := m.comments[cID]
			if !ok {
				http.NotFound(w, r)
				return
			}

			if r.Method == http.MethodPatch {
				var req struct {
					Body string `json:"body"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				comment.Body = req.Body
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(comment)
				return
			}
		}
	}

	http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
}

func TestPublishComment_PublishTwiceResultsInOneComment(t *testing.T) {
	t.Parallel()

	mock := newMockGitHubServer()
	ts := httptest.NewServer(mock)
	defer ts.Close()

	client := ghci.NewClient("fake-token",
		ghci.WithBaseURL(ts.URL),
		ghci.WithHTTPClient(ts.Client()),
	)

	ctx := context.Background()
	body1 := report.CommentMarker + "\n\n## PerfGuard: PASS\nFirst run"
	body2 := report.CommentMarker + "\n\n## PerfGuard: FAIL\nSecond run"

	// First publish: creates comment
	res1, err := client.PublishComment(ctx, ghci.CommentParams{
		Owner:    "test-owner",
		Repo:     "test-repo",
		PRNumber: 42,
		Body:     body1,
	})
	if err != nil {
		t.Fatalf("first publish failed: %v", err)
	}
	if res1.Action != "created" {
		t.Errorf("expected action 'created', got %q", res1.Action)
	}
	if res1.ID != 1 {
		t.Errorf("expected comment ID 1, got %d", res1.ID)
	}

	// Second publish: updates comment
	res2, err := client.PublishComment(ctx, ghci.CommentParams{
		Owner:    "test-owner",
		Repo:     "test-repo",
		PRNumber: 42,
		Body:     body2,
	})
	if err != nil {
		t.Fatalf("second publish failed: %v", err)
	}
	if res2.Action != "updated" {
		t.Errorf("expected action 'updated', got %q", res2.Action)
	}
	if res2.ID != res1.ID {
		t.Errorf("expected updated comment to keep same ID %d, got %d", res1.ID, res2.ID)
	}

	// Verify only one comment exists in the mock server
	mock.mu.Lock()
	defer mock.mu.Unlock()

	if len(mock.comments) != 1 {
		t.Fatalf("expected exactly 1 comment on server, got %d", len(mock.comments))
	}
	if mock.comments[res1.ID].Body != body2 {
		t.Errorf("expected comment body to be updated to %q, got %q", body2, mock.comments[res1.ID].Body)
	}
}

func TestPublishComment_Validation(t *testing.T) {
	t.Parallel()

	client := ghci.NewClient("token")
	ctx := context.Background()

	cases := []struct {
		name    string
		params  ghci.CommentParams
		wantErr string
	}{
		{
			name:    "missing owner",
			params:  ghci.CommentParams{Repo: "repo", PRNumber: 1, Body: "body"},
			wantErr: "owner is required",
		},
		{
			name:    "missing repo",
			params:  ghci.CommentParams{Owner: "owner", PRNumber: 1, Body: "body"},
			wantErr: "repo is required",
		},
		{
			name:    "zero pr number",
			params:  ghci.CommentParams{Owner: "owner", Repo: "repo", PRNumber: 0, Body: "body"},
			wantErr: "pr number must be positive",
		},
		{
			name:    "negative pr number",
			params:  ghci.CommentParams{Owner: "owner", Repo: "repo", PRNumber: -5, Body: "body"},
			wantErr: "pr number must be positive",
		},
		{
			name:    "empty body",
			params:  ghci.CommentParams{Owner: "owner", Repo: "repo", PRNumber: 1, Body: ""},
			wantErr: "body is required",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := client.PublishComment(ctx, tc.params)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error %q to contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestPublishComment_ExistingOtherComments(t *testing.T) {
	t.Parallel()

	mock := newMockGitHubServer()
	// Pre-populate with a user comment that doesn't have the marker
	mock.comments[99] = &mockComment{
		ID:      99,
		Body:    "Looks good to me!",
		HTMLURL: "https://github.com/test/repo/issues/10#issuecomment-99",
	}
	mock.issueOrders[10] = []int64{99}
	mock.nextID = 100

	ts := httptest.NewServer(mock)
	defer ts.Close()

	client := ghci.NewClient("fake-token",
		ghci.WithBaseURL(ts.URL),
		ghci.WithHTTPClient(ts.Client()),
	)

	body := report.CommentMarker + "\n\n## PerfGuard: PASS"
	res, err := client.PublishComment(context.Background(), ghci.CommentParams{
		Owner:    "test-owner",
		Repo:     "test-repo",
		PRNumber: 10,
		Body:     body,
	})
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if res.Action != "created" {
		t.Errorf("expected action 'created', got %q", res.Action)
	}
	if res.ID == 99 {
		t.Errorf("did not expect to overwrite user comment 99")
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(mock.comments))
	}
	if mock.comments[99].Body != "Looks good to me!" {
		t.Errorf("user comment was modified")
	}
}

func TestPublishComment_ServerError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := ghci.NewClient("token",
		ghci.WithBaseURL(ts.URL),
		ghci.WithHTTPClient(ts.Client()),
	)

	_, err := client.PublishComment(context.Background(), ghci.CommentParams{
		Owner:    "owner",
		Repo:     "repo",
		PRNumber: 1,
		Body:     "body",
	})
	if err == nil {
		t.Fatal("expected server error, got nil")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected error to mention status 500, got %v", err)
	}
}

func TestPublishComment_Pagination(t *testing.T) {
	t.Parallel()

	mock := newMockGitHubServer()
	// Pre-populate with 105 comments: first 100 on page 1 without marker,
	// comment #103 on page 2 contains marker
	for i := 1; i <= 105; i++ {
		id := int64(i)
		body := fmt.Sprintf("Comment number %d", i)
		if i == 103 {
			body = report.CommentMarker + "\n\nOld PerfGuard Report"
		}
		mock.comments[id] = &mockComment{
			ID:      id,
			Body:    body,
			HTMLURL: fmt.Sprintf("https://github.com/test/repo/issues/7#issuecomment-%d", id),
		}
		mock.issueOrders[7] = append(mock.issueOrders[7], id)
	}
	mock.nextID = 106

	ts := httptest.NewServer(mock)
	defer ts.Close()

	client := ghci.NewClient("fake-token",
		ghci.WithBaseURL(ts.URL),
		ghci.WithHTTPClient(ts.Client()),
	)

	newBody := report.CommentMarker + "\n\nUpdated PerfGuard Report on Page 2"
	res, err := client.PublishComment(context.Background(), ghci.CommentParams{
		Owner:    "test-owner",
		Repo:     "test-repo",
		PRNumber: 7,
		Body:     newBody,
	})
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if res.Action != "updated" {
		t.Errorf("expected action 'updated', got %q", res.Action)
	}
	if res.ID != 103 {
		t.Errorf("expected updated comment ID 103, got %d", res.ID)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.comments[103].Body != newBody {
		t.Errorf("expected comment body to be updated, got %s", mock.comments[103].Body)
	}
}

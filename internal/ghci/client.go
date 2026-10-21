package ghci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kyyril/perfguard/internal/report"
)

// Publisher publishes PR comments to GitHub.
type Publisher interface {
	PublishComment(ctx context.Context, params CommentParams) (*CommentResult, error)
}

// CommentParams contains parameters for publishing a PR comment.
type CommentParams struct {
	Owner    string
	Repo     string
	PRNumber int
	Body     string
}

// CommentResult describes the outcome of publishing a comment.
type CommentResult struct {
	ID     int64
	Action string // "created" or "updated"
	URL    string
}

// Option configures Client.
type Option func(*Client)

// WithBaseURL overrides the GitHub API base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// Client interacts with the GitHub REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

var _ Publisher = (*Client)(nil)

// NewClient returns a new Client.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		httpClient: http.DefaultClient,
		baseURL:    "https://api.github.com",
		token:      token,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type githubComment struct {
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
}

// PublishComment creates or updates a PR comment containing report.CommentMarker.
func (c *Client) PublishComment(ctx context.Context, params CommentParams) (*CommentResult, error) {
	if params.Owner == "" {
		return nil, errors.New("ghci: owner is required")
	}
	if params.Repo == "" {
		return nil, errors.New("ghci: repo is required")
	}
	if params.PRNumber <= 0 {
		return nil, errors.New("ghci: pr number must be positive")
	}
	if params.Body == "" {
		return nil, errors.New("ghci: body is required")
	}

	// 1. Search existing comments for marker
	existingID, err := c.findExistingComment(ctx, params.Owner, params.Repo, params.PRNumber)
	if err != nil {
		return nil, fmt.Errorf("ghci: list comments: %w", err)
	}

	if existingID > 0 {
		// Update existing comment
		res, err := c.updateComment(ctx, params.Owner, params.Repo, existingID, params.Body)
		if err != nil {
			return nil, fmt.Errorf("ghci: update comment: %w", err)
		}
		return res, nil
	}

	// Create new comment
	res, err := c.createComment(ctx, params.Owner, params.Repo, params.PRNumber, params.Body)
	if err != nil {
		return nil, fmt.Errorf("ghci: create comment: %w", err)
	}
	return res, nil
}

func (c *Client) findExistingComment(ctx context.Context, owner, repo string, prNumber int) (int64, error) {
	page := 1
	for {
		url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments?per_page=100&page=%d",
			c.baseURL, owner, repo, prNumber, page)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return 0, err
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return 0, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			_ = resp.Body.Close()
			return 0, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
		}

		var comments []githubComment
		decErr := json.NewDecoder(resp.Body).Decode(&comments)
		_ = resp.Body.Close()
		if decErr != nil {
			return 0, fmt.Errorf("decode comments: %w", decErr)
		}

		for _, cm := range comments {
			if strings.Contains(cm.Body, report.CommentMarker) {
				return cm.ID, nil
			}
		}

		if len(comments) < 100 {
			break
		}
		page++
	}
	return 0, nil
}

func (c *Client) createComment(ctx context.Context, owner, repo string, prNumber int, body string) (*CommentResult, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.baseURL, owner, repo, prNumber)
	reqBody, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBytes))
	}

	var created githubComment
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &CommentResult{
		ID:     created.ID,
		Action: "created",
		URL:    created.HTMLURL,
	}, nil
}

func (c *Client) updateComment(ctx context.Context, owner, repo string, commentID int64, body string) (*CommentResult, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/comments/%d", c.baseURL, owner, repo, commentID)
	reqBody, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBytes))
	}

	var updated githubComment
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &CommentResult{
		ID:     updated.ID,
		Action: "updated",
		URL:    updated.HTMLURL,
	}, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

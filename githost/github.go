// Package githost asks the git host for repository metadata before cloning:
// existence, canonical name, default branch and size.
package githost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

var ErrNotFound = errors.New("repository not found")

type RepoInfo struct {
	Owner         string
	Name          string
	DefaultBranch string
	SizeKB        int64
	Private       bool
	Fork          bool
}

type Github struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func NewGithub(token string) *Github {
	return &Github{
		BaseURL: "https://api.github.com",
		Token:   token,
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *Github) Info(ctx context.Context, owner, name string) (*RepoInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s", g.BaseURL, owner, name), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "clocme")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusUnavailableForLegalReasons:
		return nil, ErrNotFound
	default:
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("github api: status %d: %s", resp.StatusCode, b)
	}

	var body struct {
		Name          string `json:"name"`
		DefaultBranch string `json:"default_branch"`
		Size          int64  `json:"size"` // KB
		Private       bool   `json:"private"`
		Fork          bool   `json:"fork"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("github api: decode: %w", err)
	}
	if body.Private {
		return nil, ErrNotFound
	}

	return &RepoInfo{
		Owner:         body.Owner.Login,
		Name:          body.Name,
		DefaultBranch: body.DefaultBranch,
		SizeKB:        body.Size,
		Private:       body.Private,
		Fork:          body.Fork,
	}, nil
}

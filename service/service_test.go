package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/holacloud/store"

	"github.com/holacloud/clocme/githost"
	"github.com/holacloud/clocme/model"
	"github.com/holacloud/clocme/telegram"
)

type fakeHost struct{ size int64 }

func (f *fakeHost) Info(ctx context.Context, owner, name string) (*githost.RepoInfo, error) {
	if name == "missing" {
		return nil, githost.ErrNotFound
	}
	return &githost.RepoInfo{Owner: owner, Name: name, DefaultBranch: "main", SizeKB: f.size}, nil
}

type fakeAnalyzer struct {
	mu       sync.Mutex
	head     string
	analyzed int
	prevLen  []int
}

func (f *fakeAnalyzer) Sync(ctx context.Context, repoID, cloneURL, branch string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.head, nil
}

func (f *fakeAnalyzer) Analyze(ctx context.Context, repoID, head string, prev []model.Point, progress func(done, total int)) ([]model.Point, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.analyzed++
	f.prevLen = append(f.prevLen, len(prev))
	progress(1, 1)
	return append(prev, model.Point{Commit: head, Time: int64(len(prev)), Languages: map[string]model.Stats{"Go": {Code: 10, Files: 1}}}), nil
}

func newTestService(a Analyzer, h Host) *Service {
	return New(store.NewStoreMemory[model.Repo](), store.NewStoreMemory[model.Series](), a, h, telegram.New(telegram.Config{}), Options{
		RefreshInterval: time.Hour,
		MaxRepoSizeKB:   1000,
	})
}

func waitStatus(t *testing.T, s *Service, id, status string) *model.Repo {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := s.Get(context.Background(), id)
		if r != nil && r.Status == status {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	r, _ := s.Get(context.Background(), id)
	t.Fatalf("timeout waiting for %s, repo=%+v", status, r)
	return nil
}

func TestRequestAnalyzeAndRefresh(t *testing.T) {
	a := &fakeAnalyzer{head: "c1"}
	s := newTestService(a, &fakeHost{size: 10})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	r, err := s.Request(ctx, "github.com", "Foo", "Bar")
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != model.StatusPending {
		t.Fatalf("status %s", r.Status)
	}

	r = waitStatus(t, s, "github.com~foo~bar", model.StatusReady)
	if r.HeadCommit != "c1" || r.Commits != 1 || r.Latest["Go"].Code != 10 {
		t.Fatalf("unexpected repo %+v", r)
	}

	// Fresh: requesting again must not re-analyze.
	s.Request(ctx, "github.com", "foo", "bar")
	time.Sleep(50 * time.Millisecond)
	if a.analyzed != 1 {
		t.Fatalf("analyzed %d times", a.analyzed)
	}

	// Stale + new head: re-analyzed incrementally.
	a.mu.Lock()
	a.head = "c2"
	a.mu.Unlock()
	s.update(ctx, "github.com~foo~bar", func(r *model.Repo) error { r.CheckedAt = time.Now().Add(-2 * time.Hour); return nil })
	s.Request(ctx, "github.com", "foo", "bar")

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r, _ = s.Get(ctx, "github.com~foo~bar")
		if r.HeadCommit == "c2" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r.HeadCommit != "c2" || r.Commits != 2 {
		t.Fatalf("not refreshed: %+v", r)
	}
	if a.prevLen[1] != 1 {
		t.Fatalf("second analysis should receive previous points, got %v", a.prevLen)
	}
}

func TestTooBigAndNotFound(t *testing.T) {
	s := newTestService(&fakeAnalyzer{head: "x"}, &fakeHost{size: 5000})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	s.Request(ctx, "github.com", "big", "repo")
	waitStatus(t, s, "github.com~big~repo", model.StatusTooBig)

	s.Request(ctx, "github.com", "some", "missing")
	waitStatus(t, s, "github.com~some~missing", model.StatusNotFound)
}

func TestRequestValidation(t *testing.T) {
	s := newTestService(&fakeAnalyzer{}, &fakeHost{})
	for _, c := range [][3]string{
		{"gitlab.com", "a", "b"},
		{"github.com", "a/b", "c"},
		{"github.com", "a", ".."},
		{"github.com", "a", "b.git"},
		{"github.com", "-a", "b"},
	} {
		if _, err := s.Request(context.Background(), c[0], c[1], c[2]); err == nil {
			t.Errorf("%v should fail", c)
		}
	}
}

// Package check asks github.com (the website, not the API) whether the owner
// of a referenced repository still exists. No token is needed and API rate
// limits do not apply.
package check

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Status of an owner or repository.
type Status string

const (
	Exists  Status = "exists"
	Missing Status = "missing"
	Moved   Status = "moved"
	Unknown Status = "unknown"
)

// Result describes one owner/repo lookup.
type Result struct {
	Owner   Status `json:"owner"`
	Repo    Status `json:"repo"`
	MovedTo string `json:"moved_to,omitempty"`
	Err     string `json:"error,omitempty"`
}

// Checker performs cached, concurrency-limited lookups.
type Checker struct {
	Base   string // default https://github.com
	Client *http.Client
	sem    chan struct{}
	mu     sync.Mutex
	owners map[string]*entry
	repos  map[string]*entry
}

type entry struct {
	once sync.Once
	st   Status
	loc  string
	err  error
}

// New returns a Checker with the given concurrency.
func New(concurrency int) *Checker {
	if concurrency < 1 {
		concurrency = 4
	}
	return &Checker{
		Base: "https://github.com",
		Client: &http.Client{
			Timeout:       20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		sem:    make(chan struct{}, concurrency),
		owners: map[string]*entry{},
		repos:  map[string]*entry{},
	}
}

func (c *Checker) get(ctx context.Context, m map[string]*entry, key, path string) (Status, string, error) {
	c.mu.Lock()
	e, ok := m[key]
	if !ok {
		e = &entry{}
		m[key] = e
	}
	c.mu.Unlock()
	e.once.Do(func() { e.st, e.loc, e.err = c.probe(ctx, path) })
	return e.st, e.loc, e.err
}

func (c *Checker) probe(ctx context.Context, path string) (Status, string, error) {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Unknown, "", ctx.Err()
			case <-time.After(time.Duration(attempt*attempt) * 2 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.Base+path, nil)
		if err != nil {
			return Unknown, "", err
		}
		req.Header.Set("User-Agent", "repojack (+https://github.com/ucsurf/repojack)")
		resp, err := c.Client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_ = resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusOK:
			return Exists, "", nil
		case resp.StatusCode == http.StatusNotFound:
			return Missing, "", nil
		case resp.StatusCode >= 300 && resp.StatusCode < 400:
			loc := strings.TrimPrefix(resp.Header.Get("Location"), c.Base+"/")
			return Moved, strings.TrimPrefix(loc, "https://github.com/"), nil
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		default:
			return Unknown, "", fmt.Errorf("HTTP %d", resp.StatusCode)
		}
	}
	return Unknown, "", lastErr
}

// Lookup checks owner and, when the owner is missing, whether the repository
// still resolves through a rename redirect.
func (c *Checker) Lookup(ctx context.Context, owner, repo string) Result {
	ok := strings.ToLower(owner)
	st, _, err := c.get(ctx, c.owners, ok, "/"+owner)
	r := Result{Owner: st, Repo: Unknown}
	if err != nil {
		r.Err = err.Error()
		return r
	}
	if st == Exists || st == Moved {
		r.Repo = Exists // not probed: only owner takeover matters here
		return r
	}
	rk := ok + "/" + strings.ToLower(repo)
	rs, loc, err := c.get(ctx, c.repos, rk, "/"+owner+"/"+repo)
	r.Repo, r.MovedTo = rs, loc
	if err != nil {
		r.Err = err.Error()
	}
	return r
}

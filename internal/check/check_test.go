package check

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func server(hits *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		switch r.URL.Path {
		case "/alive", "/alive/x":
			w.WriteHeader(200)
		case "/gone/renamed":
			w.Header().Set("Location", "https://github.com/newname/renamed")
			w.WriteHeader(301)
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestLookup(t *testing.T) {
	var hits int32
	s := server(&hits)
	defer s.Close()
	c := New(2)
	c.Base = s.URL
	ctx := context.Background()
	if r := c.Lookup(ctx, "alive", "x"); r.Owner != Exists {
		t.Fatalf("alive: %+v", r)
	}
	if r := c.Lookup(ctx, "gone", "renamed"); r.Owner != Missing || r.Repo != Moved || r.MovedTo != "newname/renamed" {
		t.Fatalf("renamed: %+v", r)
	}
	if r := c.Lookup(ctx, "gone", "deleted"); r.Owner != Missing || r.Repo != Missing {
		t.Fatalf("deleted: %+v", r)
	}
	before := atomic.LoadInt32(&hits)
	c.Lookup(ctx, "ALIVE", "y")
	if atomic.LoadInt32(&hits) != before {
		t.Fatal("owner lookups must be cached case-insensitively")
	}
}

package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func refs(t *testing.T, name string) []Ref {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "proj", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return File(p, b)
}

func names(rs []Ref) map[string]bool {
	m := map[string]bool{}
	for _, r := range rs {
		m[r.Owner+"/"+r.Repo] = true
	}
	return m
}

func TestGoMod(t *testing.T) {
	m := names(refs(t, "go.mod"))
	for _, want := range []string{"spf13/cobra", "ghost-owner-rj404/oldlib", "renamed-owner-rj404/cobra"} {
		if !m[want] {
			t.Errorf("missing %s in %v", want, m)
		}
	}
}

func TestWorkflow(t *testing.T) {
	rs := refs(t, ".github/workflows/ci.yml")
	m := names(rs)
	if len(rs) != 2 || !m["actions/checkout"] || !m["ghost-owner-rj404/setup-thing"] {
		t.Fatalf("got %v", rs)
	}
}

func TestNPM(t *testing.T) {
	m := names(refs(t, "package.json"))
	if len(m) != 2 || !m["ghost-owner-rj404/thing"] || !m["expressjs/express"] {
		t.Fatalf("got %v", m)
	}
}

func TestPipAndSubmodule(t *testing.T) {
	if m := names(refs(t, "requirements.txt")); !m["ghost-owner-rj404/pylib"] || len(m) != 1 {
		t.Fatalf("pip: %v", m)
	}
	if m := names(refs(t, ".gitmodules")); !m["ghost-owner-rj404/sub"] {
		t.Fatalf("submodule: %v", m)
	}
}

func TestCargoAndKinds(t *testing.T) {
	rs := File("Cargo.toml", []byte("[dependencies]\nfoo = { git = \"https://github.com/a-b/foo\" }\n# bar = { git = \"https://github.com/c/d\" }\n"))
	if len(rs) != 1 || rs[0].Owner != "a-b" || rs[0].Line != 2 {
		t.Fatalf("got %v", rs)
	}
	if Kind("README.md") != "" || Kind("x/.github/workflows/a.yaml") != "action" {
		t.Fatal("kind")
	}
	if rs := File("go.mod", []byte("module github.com/me/self\n")); len(rs) != 0 {
		t.Fatal("module line must be skipped")
	}
}

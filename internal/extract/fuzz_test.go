package extract

import (
	"reflect"
	"strings"
	"testing"
)

var fuzzPaths = []string{"go.mod", ".github/workflows/ci.yml", "package.json", "requirements.txt", "Cargo.toml", ".gitmodules"}

func FuzzFile(f *testing.F) {
	f.Add(uint8(0), "require github.com/foo/bar v1.0.0\n")
	f.Add(uint8(1), "steps:\n  - uses: foo/bar/sub@v1\n")
	f.Add(uint8(2), `{"dependencies":{"a":"foo/bar#v1","b":"github:x/y.git","c":"https://github.com/o/r.git","d":"npm:z"}}`)
	f.Add(uint8(3), "git+https://github.com/foo/bar.git@main#egg=bar\n")
	f.Add(uint8(4), `x = { git = "https://github.com/foo/bar" }`)
	f.Add(uint8(5), "[submodule \"x\"]\n\turl = git@github.com:foo/bar.git\n")
	f.Fuzz(func(t *testing.T, k uint8, data string) {
		p := fuzzPaths[int(k)%len(fuzzPaths)]
		a := File(p, []byte(data))
		if b := File(p, []byte(data)); !reflect.DeepEqual(a, b) {
			t.Fatalf("non-deterministic output for %s", p)
		}
		for _, r := range a {
			if !ownerValid.MatchString(r.Owner) || r.Repo == "" || strings.ContainsAny(r.Repo, "/ ") || strings.HasSuffix(r.Repo, ".git") || r.Line < 0 {
				t.Fatalf("bad ref %+v", r)
			}
		}
	})
}

func TestFileEdgeCases(t *testing.T) {
	cases := []struct {
		path, data string
		want       int
	}{
		{"go.mod", "module github.com/me/x\n// github.com/a/b\nrequire github.com/a/b v1\n", 1},
		{"package.json", "not json", 0},
		{"package.json", `{"dependencies":{"a":1,"b":"file:../x","c":"workspace:*"}}`, 0},
		{"requirements.txt", "git+https://github.com/-bad/x\ngit+https://github.com/ok/.git\n", 0},
		{".github/workflows/x.yml", "- uses: ./local\n- uses: docker://alpine\n", 0},
		{"README.md", "github.com/a/b", 0},
	}
	for _, c := range cases {
		if got := File(c.path, []byte(c.data)); len(got) != c.want {
			t.Errorf("%s %q: got %d refs %+v, want %d", c.path, c.data, len(got), got, c.want)
		}
	}
}

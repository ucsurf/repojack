// Command repojack finds dependencies whose GitHub owner no longer exists,
// so anyone could register the name and take over the repository.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"

	"github.com/ucsurf/repojack/internal/check"
	"github.com/ucsurf/repojack/internal/extract"
)

var version = ""

func buildVersion() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

// Finding is a reference whose owner can be registered by anyone.
type Finding struct {
	extract.Ref
	Reason  string `json:"reason"`
	MovedTo string `json:"moved_to,omitempty"`
}

var skipDirs = map[string]bool{".git": true, "node_modules": true, "vendor": true, "target": true, ".venv": true, "venv": true, "dist": true, "testdata": true}

func collect(paths []string) ([]extract.Ref, error) {
	var refs []extract.Ref
	for _, root := range paths {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if p != root && skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if extract.Kind(p) == "" {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			refs = append(refs, extract.File(p, data)...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return refs, nil
}

func scan(ctx context.Context, c *check.Checker, refs []extract.Ref, ignore map[string]bool) ([]Finding, []string) {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		findings []Finding
		errs     []string
	)
	for _, r := range refs {
		if ignore[strings.ToLower(r.Owner)] || ignore[strings.ToLower(r.Owner+"/"+r.Repo)] {
			continue
		}
		wg.Add(1)
		go func(r extract.Ref) {
			defer wg.Done()
			res := c.Lookup(ctx, r.Owner, r.Repo)
			mu.Lock()
			defer mu.Unlock()
			if res.Err != "" && res.Owner == check.Unknown {
				errs = append(errs, fmt.Sprintf("%s/%s: %s", r.Owner, r.Repo, res.Err))
				return
			}
			if res.Owner != check.Missing {
				return
			}
			f := Finding{Ref: r}
			if res.Repo == check.Moved {
				f.Reason = "owner renamed or deleted; repository now redirects, the old name can be registered by anyone"
				f.MovedTo = res.MovedTo
			} else {
				f.Reason = "owner does not exist; the name can be registered by anyone"
			}
			findings = append(findings, f)
		}(r)
	}
	wg.Wait()
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
	sort.Strings(errs)
	return findings, uniq(errs)
}

func uniq(s []string) []string {
	var out []string
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func writeText(w io.Writer, refs int, fs []Finding) {
	for _, f := range fs {
		fmt.Fprintf(w, "%s:%d  %s/%s  [%s]\n  %s\n", f.File, f.Line, f.Owner, f.Repo, f.Kind, f.Reason)
		if f.MovedTo != "" {
			fmt.Fprintf(w, "  fix: point to github.com/%s\n", f.MovedTo)
		}
		fmt.Fprintf(w, "  > %s\n\n", f.Text)
	}
	fmt.Fprintf(w, "%d GitHub references checked, %d hijackable\n", refs, len(fs))
}

func run(args []string, stdout, stderr io.Writer, base string) int {
	fl := flag.NewFlagSet("repojack", flag.ContinueOnError)
	fl.SetOutput(stderr)
	format := fl.String("format", "text", "output format: text, json or sarif")
	conc := fl.Int("concurrency", 4, "parallel requests to github.com")
	ign := fl.String("ignore", "", "comma-separated owners or owner/repo to skip")
	list := fl.Bool("list", false, "only list the GitHub references found, no network")
	showVer := fl.Bool("version", false, "print version")
	fl.Usage = func() {
		fmt.Fprintf(stderr, "usage: repojack [flags] [path ...]\n\nChecks go.mod, GitHub Actions, package.json, requirements*.txt, pyproject.toml,\nPipfile, Cargo.toml and .gitmodules for GitHub owners that no longer exist.\nNo token needed. Exit code 1 when something is hijackable.\n\n")
		fl.PrintDefaults()
	}
	if err := fl.Parse(args); err != nil {
		return 2
	}
	if *showVer {
		fmt.Fprintln(stdout, "repojack", buildVersion())
		return 0
	}
	paths := fl.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	refs, err := collect(paths)
	if err != nil {
		fmt.Fprintln(stderr, "repojack:", err)
		return 2
	}
	if *list {
		for _, r := range refs {
			fmt.Fprintf(stdout, "%s:%d\t%s\t%s/%s\n", r.File, r.Line, r.Kind, r.Owner, r.Repo)
		}
		return 0
	}
	ignore := map[string]bool{}
	for _, s := range strings.Split(*ign, ",") {
		if s = strings.TrimSpace(s); s != "" {
			ignore[strings.ToLower(s)] = true
		}
	}
	c := check.New(*conc)
	if base != "" {
		c.Base = base
	}
	findings, errs := scan(context.Background(), c, refs, ignore)
	for _, e := range errs {
		fmt.Fprintln(stderr, "repojack: warning:", e)
	}
	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if findings == nil {
			findings = []Finding{}
		}
		_ = enc.Encode(findings)
	case "sarif":
		_ = writeSARIF(stdout, findings)
	case "text":
		writeText(stdout, len(refs), findings)
	default:
		fmt.Fprintln(stderr, "repojack: unknown format", *format)
		return 2
	}
	if len(findings) > 0 {
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, "")) }

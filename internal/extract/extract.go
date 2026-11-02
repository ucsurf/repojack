// Package extract finds references to GitHub repositories in dependency
// manifests, workflows and submodule files.
package extract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Ref is one reference to a GitHub repository.
type Ref struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Kind  string `json:"kind"` // go, action, npm, pip, cargo, submodule
	Text  string `json:"text"`
}

var (
	ghURL      = regexp.MustCompile(`(?i)github\.com[/:]([A-Za-z0-9](?:[A-Za-z0-9-]{0,38}))/([A-Za-z0-9._-]+)`)
	goMod      = regexp.MustCompile(`(?:^|\s)github\.com/([A-Za-z0-9-]+)/([A-Za-z0-9._-]+)`)
	usesRe     = regexp.MustCompile(`^\s*-?\s*uses:\s*["']?([A-Za-z0-9-]+)/([A-Za-z0-9._-]+)[^@\s"']*@`)
	npmShort   = regexp.MustCompile(`^(?:github:)?([A-Za-z0-9-]+)/([A-Za-z0-9._-]+?)(?:\.git)?(?:#.*)?$`)
	ownerValid = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9]|-[A-Za-z0-9]){0,38}$`)
)

// Kind returns the manifest kind for a file name, or "" if unsupported.
func Kind(path string) string {
	p := filepath.ToSlash(path)
	base := filepath.Base(p)
	switch {
	case base == "go.mod":
		return "go"
	case (strings.Contains(p, ".github/workflows/") && (strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml"))) || base == "action.yml" || base == "action.yaml":
		return "action"
	case base == "package.json":
		return "npm"
	case strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt"), base == "pyproject.toml", base == "Pipfile":
		return "pip"
	case base == "Cargo.toml":
		return "cargo"
	case base == ".gitmodules":
		return "submodule"
	}
	return ""
}

// File extracts references from one file's content.
func File(path string, data []byte) []Ref {
	kind := Kind(path)
	switch kind {
	case "":
		return nil
	case "npm":
		return npm(path, data)
	}
	var out []Ref
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
			continue
		}
		var ms [][]string
		switch kind {
		case "go":
			if strings.HasPrefix(t, "module ") {
				continue
			}
			ms = goMod.FindAllStringSubmatch(line, -1)
		case "action":
			if m := usesRe.FindStringSubmatch(line); m != nil {
				ms = [][]string{m}
			}
		default:
			ms = ghURL.FindAllStringSubmatch(line, -1)
		}
		for _, m := range ms {
			if r, ok := mk(path, n, m[1], m[2], kind, t); ok {
				out = append(out, r)
			}
		}
	}
	return out
}

func mk(path string, line int, owner, repo, kind, text string) (Ref, bool) {
	repo = strings.TrimSuffix(repo, ".git")
	if !ownerValid.MatchString(owner) || repo == "" || repo == "." || repo == ".." {
		return Ref{}, false
	}
	return Ref{File: filepath.ToSlash(path), Line: line, Owner: owner, Repo: repo, Kind: kind, Text: text}, true
}

func npm(path string, data []byte) []Ref {
	var pkg map[string]json.RawMessage
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	var out []Ref
	for _, sec := range []string{"dependencies", "devDependencies", "optionalDependencies", "peerDependencies", "overrides", "resolutions"} {
		var deps map[string]any
		if json.Unmarshal(pkg[sec], &deps) != nil {
			continue
		}
		names := make([]string, 0, len(deps))
		for name := range deps {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			v := deps[name]
			spec, ok := v.(string)
			if !ok {
				continue
			}
			var owner, repo string
			if m := ghURL.FindStringSubmatch(spec); m != nil {
				owner, repo = m[1], m[2]
			} else if m := npmShort.FindStringSubmatch(spec); m != nil && !strings.HasPrefix(spec, "npm:") && !strings.HasPrefix(spec, "file:") && !strings.HasPrefix(spec, "link:") && !strings.HasPrefix(spec, "workspace:") {
				owner, repo = m[1], m[2]
			} else {
				continue
			}
			if r, ok := mk(path, lineOf(data, spec), owner, repo, "npm", name+": "+spec); ok {
				out = append(out, r)
			}
		}
	}
	return out
}

func lineOf(data []byte, s string) int {
	i := bytes.Index(data, []byte(`"`+s+`"`))
	if i < 0 {
		return 0
	}
	return bytes.Count(data[:i], []byte("\n")) + 1
}

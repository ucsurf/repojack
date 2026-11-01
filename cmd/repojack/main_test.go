package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fake() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "rj404") {
			if r.URL.Path == "/renamed-owner-rj404/cobra" {
				w.Header().Set("Location", "https://github.com/spf13/cobra")
				w.WriteHeader(301)
				return
			}
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(200)
	}))
}

func TestRunText(t *testing.T) {
	s := fake()
	defer s.Close()
	var out, errb bytes.Buffer
	code := run([]string{"../../testdata/proj"}, &out, &errb, s.URL)
	if code != 1 {
		t.Fatalf("code %d: %s %s", code, out.String(), errb.String())
	}
	o := out.String()
	for _, want := range []string{"10 GitHub references checked, 6 hijackable", "fix: point to github.com/spf13/cobra", "ghost-owner-rj404/pylib"} {
		if !strings.Contains(o, want) {
			t.Errorf("missing %q in\n%s", want, o)
		}
	}
}

func TestRunJSONIgnoreAndSARIF(t *testing.T) {
	s := fake()
	defer s.Close()
	var out bytes.Buffer
	code := run([]string{"-format", "json", "-ignore", "ghost-owner-rj404,renamed-owner-rj404/cobra", "../../testdata/proj"}, &out, &bytes.Buffer{}, s.URL)
	var fs []Finding
	if err := json.Unmarshal(out.Bytes(), &fs); err != nil || code != 0 || len(fs) != 0 {
		t.Fatalf("code %d, %v, %s", code, err, out.String())
	}
	out.Reset()
	run([]string{"-format", "sarif", "../../testdata/proj"}, &out, &bytes.Buffer{}, s.URL)
	if !strings.Contains(out.String(), `"ruleId": "RJ001"`) {
		t.Fatal("sarif")
	}
}

func TestSkipsTestdataAndVersion(t *testing.T) {
	var out bytes.Buffer
	if run([]string{"-list", "../.."}, &out, &bytes.Buffer{}, "") != 0 || strings.Contains(out.String(), "rj404") {
		t.Fatalf("testdata should be skipped:\n%s", out.String())
	}
	out.Reset()
	run([]string{"-version"}, &out, &bytes.Buffer{}, "")
	if !strings.HasPrefix(out.String(), "repojack ") {
		t.Fatal(out.String())
	}
}

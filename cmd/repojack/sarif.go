package main

import (
	"encoding/json"
	"fmt"
	"io"
)

func writeSARIF(w io.Writer, fs []Finding) error {
	results := []map[string]any{}
	for _, f := range fs {
		line := f.Line
		if line < 1 {
			line = 1
		}
		results = append(results, map[string]any{
			"ruleId":  "RJ001",
			"level":   "error",
			"message": map[string]any{"text": fmt.Sprintf("github.com/%s/%s: %s", f.Owner, f.Repo, f.Reason)},
			"locations": []any{map[string]any{"physicalLocation": map[string]any{
				"artifactLocation": map[string]any{"uri": f.File},
				"region":           map[string]any{"startLine": line},
			}}},
		})
	}
	doc := map[string]any{
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"version": "2.1.0",
		"runs": []any{map[string]any{
			"tool": map[string]any{"driver": map[string]any{
				"name": "repojack", "version": buildVersion(), "informationUri": "https://github.com/ucsurf/repojack",
				"rules": []any{map[string]any{
					"id": "RJ001", "name": "HijackableGitHubOwner",
					"shortDescription": map[string]any{"text": "Dependency on a GitHub owner that no longer exists"},
					"help":             map[string]any{"text": "Anyone can register the missing owner name and serve code at this path. Point the dependency at the new location or a fork you control."},
					"properties":       map[string]any{"security-severity": "8.1", "tags": []string{"security", "supply-chain"}},
				}},
			}},
			"results": results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

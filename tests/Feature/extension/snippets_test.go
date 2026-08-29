package extension_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSnippetsCoverAWholeViewAndEveryKyseWritingFlow(t *testing.T) {
	type snippet struct {
		Prefix string          `json:"prefix"`
		Body   json.RawMessage `json:"body"`
	}
	var snippets map[string]snippet
	readJSON(t, "snippets/kyse.json", &snippets)

	want := map[string][]string{
		"view":     {"//go:build kyse", "package ", "@extends(", "@section(", "@endsection"},
		"layout":   {"//go:build kyse", "package ", "@yield("},
		"if":       {"@if(", "@else", "@endif"},
		"elseif":   {"@elseif("},
		"foreach":  {"@foreach(", "@endforeach"},
		"forelse":  {"@forelse(", "@empty", "@endforelse"},
		"for":      {"@for(", "@endfor"},
		"while":    {"@while(", "@endwhile"},
		"go":       {"@go", "@endgo"},
		"extends":  {"@extends("},
		"section":  {"@section(", "@endsection"},
		"yield":    {"@yield("},
		"include":  {"@include("},
		"csrf":     {"@csrf"},
		"echo":     {"{{ ", " }}"},
		"raw":      {"{!! ", " !!}"},
		"continue": {"@continue"},
		"break":    {"@break"},
	}
	byPrefix := make(map[string]string, len(snippets))
	for name, candidate := range snippets {
		body := snippetBody(t, name, candidate.Body)
		if _, exists := byPrefix[candidate.Prefix]; exists {
			t.Fatalf("duplicate snippet prefix %q", candidate.Prefix)
		}
		byPrefix[candidate.Prefix] = body
	}
	for prefix, fragments := range want {
		body, ok := byPrefix[prefix]
		if !ok {
			t.Errorf("missing %q snippet", prefix)
			continue
		}
		for _, fragment := range fragments {
			if !strings.Contains(body, fragment) {
				t.Errorf("%q snippet does not contain %q", prefix, fragment)
			}
		}
	}
}

func snippetBody(t *testing.T, name string, raw json.RawMessage) string {
	t.Helper()
	var lines []string
	if err := json.Unmarshal(raw, &lines); err == nil {
		return strings.Join(lines, "\n")
	}
	var line string
	if err := json.Unmarshal(raw, &line); err != nil {
		t.Fatalf("decode %q snippet body: %v", name, err)
	}
	return line
}

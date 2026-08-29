package extension_test

import (
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type grammarInclude struct {
	Include string `json:"include"`
}

func TestTheGrammarRecognizesExactlyTheKyseLanguage(t *testing.T) {
	type pattern struct {
		Match string `json:"match"`
		Name  string `json:"name"`
	}
	var grammar struct {
		Name       string `json:"name"`
		ScopeName  string `json:"scopeName"`
		Repository map[string]struct {
			Patterns []pattern `json:"patterns"`
		} `json:"repository"`
	}
	readJSON(t, "syntaxes/kyse.tmLanguage.json", &grammar)

	if grammar.Name != "Kyse" || grammar.ScopeName != "source.kyse" {
		t.Fatalf("grammar identity = %q %q", grammar.Name, grammar.ScopeName)
	}
	want := []string{
		"break", "continue", "csrf", "else", "elseif", "empty", "endfor",
		"endforeach", "endforelse", "endgo", "endif", "endsection", "endwhile",
		"extends", "for", "foreach", "forelse", "go", "if", "include",
		"section", "while", "yield",
	}
	literal := regexp.MustCompile(`^@([a-z]+)\\b$`)
	var got []string
	for _, candidate := range grammar.Repository["known-directives"].Patterns {
		if candidate.Name != "keyword.control.kyse" {
			t.Fatalf("directive %q has scope %q", candidate.Match, candidate.Name)
		}
		parts := literal.FindStringSubmatch(candidate.Match)
		if len(parts) != 2 {
			t.Fatalf("directive pattern %q is not one closed literal", candidate.Match)
		}
		got = append(got, parts[1])
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("grammar directives = %v, want %v", got, want)
	}

	raw, err := os.ReadFile(rootPath(t, "syntaxes/kyse.tmLanguage.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, scope := range []string{
		"comment.block.kyse",
		"meta.interpolation.raw.kyse",
		"meta.interpolation.escaped.kyse",
		"meta.embedded.block.go",
		"meta.embedded.html.kyse",
	} {
		if !strings.Contains(text, scope) {
			t.Errorf("grammar has no %s scope", scope)
		}
	}
	if !strings.Contains(text, `source.go`) || !strings.Contains(text, `text.html.basic`) {
		t.Error("grammar does not embed both Go and HTML")
	}
}

func TestTheGrammarKeepsSingleAndBlockGoImportsOutOfHTML(t *testing.T) {
	type pattern struct {
		Begin    string           `json:"begin"`
		End      string           `json:"end"`
		Name     string           `json:"name"`
		Patterns []grammarInclude `json:"patterns"`
	}
	var grammar struct {
		Patterns   []grammarInclude `json:"patterns"`
		Repository map[string]struct {
			Patterns []pattern `json:"patterns"`
		} `json:"repository"`
	}
	readJSON(t, "syntaxes/kyse.tmLanguage.json", &grammar)

	commentPosition, directivePosition := -1, -1
	for position, candidate := range grammar.Patterns {
		switch candidate.Include {
		case "#comments":
			commentPosition = position
		case "#known-directives":
			directivePosition = position
		}
	}
	if commentPosition < 0 || directivePosition < 0 || commentPosition > directivePosition {
		t.Fatal("Kyse comments must be recognized before directives")
	}

	imports := make(map[string]pattern)
	positions := make(map[string]int)
	for position, candidate := range grammar.Repository["header"].Patterns {
		if candidate.Name == "meta.import.single.go" || candidate.Name == "meta.import.block.go" {
			imports[candidate.Name] = candidate
			positions[candidate.Name] = position
		}
	}
	single, ok := imports["meta.import.single.go"]
	if !ok {
		t.Fatal("single Go imports have no grammar scope")
	}
	for _, line := range []string{
		`import "github.com/arandu-io/hesape/view"`,
		`import components "github.com/arandu-io/kyse/components"`,
	} {
		if !regexp.MustCompile(single.Begin).MatchString(line) {
			t.Errorf("single import pattern does not recognize %q", line)
		}
	}
	if single.End != "$" || !includes(single.Patterns, "source.go") {
		t.Fatal("single imports are not embedded as Go through the end of the line")
	}

	block, ok := imports["meta.import.block.go"]
	if !ok {
		t.Fatal("Go import blocks have no grammar scope")
	}
	if !regexp.MustCompile(block.Begin).MatchString("import (") || !regexp.MustCompile(block.End).MatchString(")") {
		t.Fatal("import block boundaries are not recognized")
	}
	if !includes(block.Patterns, "source.go") {
		t.Fatal("import block contents are not embedded as Go")
	}
	if positions["meta.import.block.go"] > positions["meta.import.single.go"] {
		t.Fatal("the single-line import rule hides the import-block rule")
	}
}

func includes(patterns []grammarInclude, want string) bool {
	for _, pattern := range patterns {
		if pattern.Include == want {
			return true
		}
	}
	return false
}

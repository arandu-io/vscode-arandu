package extension_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	tagInjectionPath  = "syntaxes/kyse.tag.injection.tmLanguage.json"
	tagInjectionScope = "kyse.tag.injection"
)

// tagToken is one run of a line with the same Kyse scopes. An empty scope list
// is text the HTML grammar owns.
type tagToken struct {
	Text   string
	Scopes string
}

func TestTheGrammarInjectsKyseIntoEveryHTMLTag(t *testing.T) {
	var manifest struct {
		Files       []string `json:"files"`
		Contributes struct {
			Grammars []struct {
				Language  string   `json:"language"`
				ScopeName string   `json:"scopeName"`
				Path      string   `json:"path"`
				InjectTo  []string `json:"injectTo"`
			} `json:"grammars"`
		} `json:"contributes"`
	}
	readJSON(t, "package.json", &manifest)

	found := false
	for _, grammar := range manifest.Contributes.Grammars {
		if grammar.ScopeName != tagInjectionScope {
			continue
		}
		found = true
		// An injection grammar owns no language: it is pulled into the Kyse one
		// and runs inside whatever scope its selector names.
		if grammar.Language != "" || grammar.Path != tagInjectionPath || len(grammar.InjectTo) != 1 || grammar.InjectTo[0] != "source.kyse" {
			t.Fatalf("tag injection declaration = %#v", grammar)
		}
	}
	if !found {
		t.Fatalf("package.json declares no %s grammar", tagInjectionScope)
	}
	if !contains(manifest.Files, tagInjectionPath) {
		t.Fatalf("published files do not contain %s", tagInjectionPath)
	}

	var injection tmGrammar
	readJSON(t, tagInjectionPath, &injection)
	if injection.ScopeName != tagInjectionScope {
		t.Fatalf("injection scope = %q, want %q", injection.ScopeName, tagInjectionScope)
	}
	// L: puts the injection ahead of the HTML grammar when both match at the
	// same column, which is the whole point: at that column HTML would read a
	// directive as an attribute name. meta.tag is the prefix of every tag scope
	// the HTML grammar has, attribute values included, so a {{ }} inside a
	// quoted value is reached too. A comment and an interpolation already
	// belong to Kyse, and the injection must not reopen them.
	if want := "L:meta.tag - (comment, meta.interpolation)"; injection.InjectionSelector != want {
		t.Fatalf("injection selector = %q, want %q", injection.InjectionSelector, want)
	}
}

// TestTheGrammarTokenizesKyseInsideOpeningTags pins what a reader sees in the
// opening tags kyse/components writes. The expectations were recorded from the
// editor's own TextMate engine with its HTML grammar, with the scopes HTML owns
// removed; tokenizeTag reproduces the Kyse half of that engine, so a rule that
// stops matching -- or starts matching what HTML owns -- changes the tokens.
func TestTheGrammarTokenizesKyseInsideOpeningTags(t *testing.T) {
	const (
		directive   = "meta.directive.kyse"
		keyword     = directive + " keyword.control.kyse"
		argsBegin   = directive + " punctuation.section.arguments.begin.kyse"
		argsEnd     = directive + " punctuation.section.arguments.end.kyse"
		argument    = directive + " meta.embedded.expression.go"
		escaped     = "meta.interpolation.escaped.kyse"
		escapedOpen = escaped + " punctuation.section.interpolation.escaped.begin.kyse"
		escapedEnd  = escaped + " punctuation.section.interpolation.escaped.end.kyse"
		raw         = "meta.interpolation.raw.kyse"
		rawOpen     = raw + " punctuation.section.interpolation.raw.begin.kyse"
		rawEnd      = raw + " punctuation.section.interpolation.raw.end.kyse"
		member      = " variable.other.member.go"
		goString    = " string.quoted.double.go"
	)
	cases := []struct {
		name string
		tag  []string
		want []tagToken
	}{
		{
			// kyse/components/dialog.kyse.go: a directive per line, the shape
			// every component writes its parts in.
			name: "multi-line tag with directives",
			tag: []string{
				`			<h2`,
				`				data-part="title"`,
				`				@if(.PartClass("title") != "")`,
				`					class="{{ .PartClass("title") }}"`,
				`				@endif`,
				`				id="{{ .ID }}-title"`,
				`				@attributes(.PartAttrs("title"))`,
				`			>`,
			},
			want: []tagToken{
				{"\t\t\t<h2", ""},
				{"\t\t\t\tdata-part=\"title\"", ""},
				{"@if", keyword},
				{"(", argsBegin},
				{".PartClass", argument + member},
				{"(", argument},
				{`"title"`, argument + goString},
				{") != ", argument},
				{`""`, argument + goString},
				{")", argsEnd},
				{"\t\t\t\t\tclass=\"", ""},
				{"{{", escapedOpen},
				{".PartClass", escaped + member},
				{"(", escaped},
				{`"title"`, escaped + goString},
				{") ", escaped},
				{"}}", escapedEnd},
				{`"`, ""},
				{"@endif", keyword},
				{"\t\t\t\tid=\"", ""},
				{"{{", escapedOpen},
				{".ID", escaped + member},
				{"}}", escapedEnd},
				{"-title\"", ""},
				{"@attributes", keyword},
				{"(", argsBegin},
				{".PartAttrs", argument + member},
				{"(", argument},
				{`"title"`, argument + goString},
				{")", argument},
				{")", argsEnd},
				{"\t\t\t>", ""},
			},
		},
		{
			// kyse/components/separator.kyse.go: @else between attributes, and a
			// quoted Go string inside a quoted attribute value.
			name: "else branch and nested quotes",
			tag: []string{
				`<hr`,
				`	@if(.Decorative)`,
				`		role="none" aria-hidden="true"`,
				`	@else`,
				`		role="separator" aria-orientation="{{ .Direction() }}"`,
				`	@endif`,
				`	@if(.Direction() == "vertical")`,
				`		class="{{ .RootClass("bg-border w-px shrink-0 self-stretch border-0") }}"`,
				`	@endif`,
				`	@attributes(.RootAttrs())`,
				`>`,
			},
			want: []tagToken{
				{"<hr", ""},
				{"@if", keyword},
				{"(", argsBegin},
				{".Decorative", argument + member},
				{")", argsEnd},
				{"\t\trole=\"none\" aria-hidden=\"true\"", ""},
				{"@else", keyword},
				{"\t\trole=\"separator\" aria-orientation=\"", ""},
				{"{{", escapedOpen},
				{".Direction", escaped + member},
				{"() ", escaped},
				{"}}", escapedEnd},
				{`"`, ""},
				{"@endif", keyword},
				{"@if", keyword},
				{"(", argsBegin},
				{".Direction", argument + member},
				{"() == ", argument},
				{`"vertical"`, argument + goString},
				{")", argsEnd},
				{"\t\tclass=\"", ""},
				{"{{", escapedOpen},
				{".RootClass", escaped + member},
				{"(", escaped},
				{`"bg-border w-px shrink-0 self-stretch border-0"`, escaped + goString},
				{") ", escaped},
				{"}}", escapedEnd},
				{`"`, ""},
				{"@endif", keyword},
				{"@attributes", keyword},
				{"(", argsBegin},
				{".RootAttrs", argument + member},
				{"()", argument},
				{")", argsEnd},
				{">", ""},
			},
		},
		{
			// kyse/components/dialog.kyse.go: a whole opening tag on one line.
			name: "single-line tag",
			tag: []string{
				`			<form method="{{ .FormMethod() }}" action="{{ .Action }}">`,
			},
			want: []tagToken{
				{"\t\t\t<form method=\"", ""},
				{"{{", escapedOpen},
				{".FormMethod", escaped + member},
				{"() ", escaped},
				{"}}", escapedEnd},
				{"\" action=\"", ""},
				{"{{", escapedOpen},
				{".Action", escaped + member},
				{"}}", escapedEnd},
				{"\">", ""},
			},
		},
		{
			// A comment, the raw form, a directive Kyse refuses, and an @ that is
			// not at the start of its line -- which Kyse leaves to the markup.
			name: "comment, raw interpolation, unknown directive",
			tag: []string{
				`<span`,
				`	{{-- the icon is drawn by the server --}}`,
				`	data-icon="{!! icons.Star(icons.Props{}) !!}"`,
				`	@click="open = true"`,
				`	data-contact="team@arandu.io"`,
				`>`,
			},
			want: []tagToken{
				{"<span", ""},
				{"{{-- the icon is drawn by the server --}}", "comment.block.kyse"},
				{"\tdata-icon=\"", ""},
				{"{!!", rawOpen},
				{"icons", raw + " variable.other.go"},
				{".Star", raw + member},
				{"(", raw},
				{"icons", raw + " variable.other.go"},
				{".Props", raw + member},
				{"{}) ", raw},
				{"!!}", rawEnd},
				{`"`, ""},
				{"@click", "invalid.illegal.unknown-directive.kyse"},
				{`="open = true"`, ""},
				{"\tdata-contact=\"team@arandu.io\"", ""},
				{">", ""},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenizeTag(t, tc.tag)
			if len(got) != len(tc.want) {
				t.Fatalf("tokens = %d, want %d\n%s", len(got), len(tc.want), formatTagTokens(got))
			}
			for index := range got {
				if got[index] != tc.want[index] {
					t.Fatalf("token %d = %q %q, want %q %q\n%s", index, got[index].Text, got[index].Scopes, tc.want[index].Text, tc.want[index].Scopes, formatTagTokens(got))
				}
			}
		})
	}
}

// tmRule is the part of a TextMate rule the Kyse grammars use.
type tmRule struct {
	Include       string            `json:"include"`
	Name          string            `json:"name"`
	Match         string            `json:"match"`
	Begin         string            `json:"begin"`
	End           string            `json:"end"`
	Captures      map[string]tmRule `json:"captures"`
	BeginCaptures map[string]tmRule `json:"beginCaptures"`
	EndCaptures   map[string]tmRule `json:"endCaptures"`
	Patterns      []tmRule          `json:"patterns"`
}

type tmGrammar struct {
	ScopeName         string            `json:"scopeName"`
	InjectionSelector string            `json:"injectionSelector"`
	Patterns          []tmRule          `json:"patterns"`
	Repository        map[string]tmRule `json:"repository"`
}

// boundRule is a rule with the grammar its own #includes resolve against.
type boundRule struct {
	grammar *tmGrammar
	rule    tmRule
}

type tmFrame struct {
	rule     boundRule
	patterns []boundRule
	scopes   []string
}

type tagTokenizer struct {
	t        *testing.T
	grammars map[string]*tmGrammar
	compiled map[string]*regexp.Regexp
}

// tokenizeTag runs the tag injection over the lines of an opening tag, the way
// the editor's TextMate engine runs it once HTML has opened the tag: at every
// column the leftmost match wins, an open rule's end beats its patterns on a
// tie, and a capture with patterns is tokenized again inside its own range.
// What no Kyse rule matches is left to HTML and comes back with no scope.
//
// The regular expressions run on Go's engine, so every rule a tag reaches has
// to stay inside the syntax both engines read alike -- no lookaround and no
// backreference. A rule that leaves it fails to compile here.
func tokenizeTag(t *testing.T, lines []string) []tagToken {
	t.Helper()
	main := new(tmGrammar)
	readJSON(t, "syntaxes/kyse.tmLanguage.json", main)
	injection := new(tmGrammar)
	readJSON(t, tagInjectionPath, injection)
	tokenizer := &tagTokenizer{
		t:        t,
		grammars: map[string]*tmGrammar{main.ScopeName: main, injection.ScopeName: injection},
		compiled: make(map[string]*regexp.Regexp),
	}
	root := tokenizer.resolve(injection, injection.Patterns)

	var tokens []tagToken
	var stack []tmFrame
	for _, line := range lines {
		scopes := make([][]string, len(line))
		tokenizer.scan(line, 0, len(line), &stack, root, nil, scopes)
		for start := 0; start < len(line); {
			end := start + 1
			for end < len(line) && strings.Join(scopes[end], " ") == strings.Join(scopes[start], " ") {
				end++
			}
			if text := line[start:end]; strings.TrimSpace(text) != "" {
				tokens = append(tokens, tagToken{Text: text, Scopes: strings.Join(scopes[start], " ")})
			}
			start = end
		}
	}
	if len(stack) != 0 {
		t.Fatalf("tag left %d Kyse rules open", len(stack))
	}
	return tokens
}

// resolve flattens includes into the rules that can match. A rule with neither
// match nor begin only groups others, and its name is never applied.
func (k *tagTokenizer) resolve(owner *tmGrammar, rules []tmRule) []boundRule {
	var out []boundRule
	for _, rule := range rules {
		if rule.Include == "" {
			if rule.Match == "" && rule.Begin == "" {
				out = append(out, k.resolve(owner, rule.Patterns)...)
				continue
			}
			out = append(out, boundRule{grammar: owner, rule: rule})
			continue
		}
		scope, name, _ := strings.Cut(rule.Include, "#")
		target := owner
		if scope != "" {
			target = k.grammars[scope]
			if target == nil {
				k.t.Fatalf("include %q names a grammar outside Kyse", rule.Include)
			}
		}
		if name == "" {
			out = append(out, k.resolve(target, target.Patterns)...)
			continue
		}
		included, ok := target.Repository[name]
		if !ok {
			k.t.Fatalf("include %q names no repository rule", rule.Include)
		}
		out = append(out, k.resolve(target, []tmRule{included})...)
	}
	return out
}

// scan paints scopes over line[from:to].
func (k *tagTokenizer) scan(line string, from, to int, stack *[]tmFrame, root []boundRule, base []string, scopes [][]string) {
	for position := from; position < to; {
		patterns, current, end := root, base, ""
		if top := len(*stack) - 1; top >= 0 {
			frame := (*stack)[top]
			patterns, current, end = frame.patterns, frame.scopes, frame.rule.rule.End
		}

		var best []int
		chosen := -1
		if end != "" {
			best = k.search(end, line[:to], position)
		}
		for index, candidate := range patterns {
			pattern := candidate.rule.Match
			if pattern == "" {
				pattern = candidate.rule.Begin
			}
			if found := k.search(pattern, line[:to], position); found != nil && (best == nil || found[0] < best[0]) {
				best, chosen = found, index
			}
		}
		if best == nil {
			paint(scopes, position, to, current)
			return
		}
		paint(scopes, position, best[0], current)

		switch {
		case chosen < 0:
			frame := (*stack)[len(*stack)-1]
			paint(scopes, best[0], best[1], current)
			k.captures(line, best, frame.rule, frame.rule.rule.EndCaptures, current, scopes)
			*stack = (*stack)[:len(*stack)-1]
		case patterns[chosen].rule.Match != "":
			rule := patterns[chosen]
			matched := withScope(current, rule.rule.Name)
			paint(scopes, best[0], best[1], matched)
			k.captures(line, best, rule, rule.rule.Captures, matched, scopes)
		default:
			rule := patterns[chosen]
			opened := withScope(current, rule.rule.Name)
			paint(scopes, best[0], best[1], opened)
			k.captures(line, best, rule, rule.rule.BeginCaptures, opened, scopes)
			*stack = append(*stack, tmFrame{rule: rule, patterns: k.resolve(rule.grammar, rule.rule.Patterns), scopes: opened})
		}
		if best[1] == position && chosen >= 0 {
			k.t.Fatalf("rule matched nothing at column %d of %q", position, line)
		}
		position = best[1]
	}
}

// captures paints each numbered group, inside the groups that enclose it.
func (k *tagTokenizer) captures(line string, match []int, owner boundRule, captures map[string]tmRule, base []string, scopes [][]string) {
	for group := 0; 2*group+1 < len(match); group++ {
		capture, ok := captures[fmt.Sprint(group)]
		start, end := match[2*group], match[2*group+1]
		if !ok || start < 0 {
			continue
		}
		enclosing := base
		for outer := 0; outer < group; outer++ {
			outerCapture, named := captures[fmt.Sprint(outer)]
			if named && outerCapture.Name != "" && match[2*outer] <= start && end <= match[2*outer+1] && match[2*outer] >= 0 {
				enclosing = withScope(enclosing, outerCapture.Name)
			}
		}
		painted := withScope(enclosing, capture.Name)
		paint(scopes, start, end, painted)
		if len(capture.Patterns) != 0 {
			var local []tmFrame
			k.scan(line, start, end, &local, k.resolve(owner.grammar, capture.Patterns), painted, scopes)
		}
	}
}

// search finds the leftmost match of pattern at or after position, reading the
// line the way the editor does: ^ holds only at the start of the line, and \b
// sees the character before position.
func (k *tagTokenizer) search(pattern, line string, position int) []int {
	key := pattern
	if position > 0 {
		key = "\x00" + pattern
	}
	re, ok := k.compiled[key]
	if !ok {
		source := "(" + pattern + ")"
		if position > 0 {
			source = `^[\s\S][\s\S]*?(` + pattern + ")"
		}
		var err error
		re, err = regexp.Compile(source)
		if err != nil {
			k.t.Fatalf("grammar pattern %q is outside the syntax both engines share: %v", pattern, err)
		}
		k.compiled[key] = re
	}
	offset := 0
	if position > 0 {
		_, size := utf8.DecodeLastRuneInString(line[:position])
		offset = position - size
	}
	found := re.FindStringSubmatchIndex(line[offset:])
	if found == nil {
		return nil
	}
	match := found[2:]
	for index := range match {
		if match[index] >= 0 {
			match[index] += offset
		}
	}
	return match
}

func paint(scopes [][]string, from, to int, with []string) {
	for index := from; index < to; index++ {
		scopes[index] = with
	}
}

func withScope(scopes []string, name string) []string {
	if name == "" {
		return scopes
	}
	return append(append([]string(nil), scopes...), name)
}

func formatTagTokens(tokens []tagToken) string {
	var out strings.Builder
	for _, token := range tokens {
		fmt.Fprintf(&out, "{%q, %q},\n", token.Text, token.Scopes)
	}
	return out.String()
}

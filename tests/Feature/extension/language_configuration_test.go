package extension_test

import (
	"regexp"
	"testing"
)

func TestTheEditorPairsCommentsAndIndentsEveryKyseBlock(t *testing.T) {
	var configuration struct {
		Comments struct {
			Block []string `json:"blockComment"`
		} `json:"comments"`
		Brackets [][]string `json:"brackets"`
		Indent   struct {
			Increase string `json:"increaseIndentPattern"`
			Decrease string `json:"decreaseIndentPattern"`
		} `json:"indentationRules"`
	}
	readJSON(t, "language-configuration.json", &configuration)

	if !equalPair(configuration.Comments.Block, "{{--", "--}}") {
		t.Fatalf("block comment = %v", configuration.Comments.Block)
	}
	for _, pair := range [][2]string{{"{{", "}}"}, {"{!!", "!!}"}} {
		if !hasPair(configuration.Brackets, pair[0], pair[1]) {
			t.Errorf("brackets do not pair %q with %q", pair[0], pair[1])
		}
	}
	increase := regexp.MustCompile(configuration.Indent.Increase)
	for _, line := range []string{
		"@section('content')", "@if(.Ready)", "@foreach(.Items as item)",
		"@forelse(.Items as item)", "@for(i := 0; i < 3; i++)", "@while(.Ready)",
		"@go", "@else", "@elseif(.Other)", "@empty",
	} {
		if !increase.MatchString(line) {
			t.Errorf("%q does not indent its following line", line)
		}
	}
	decrease := regexp.MustCompile(configuration.Indent.Decrease)
	for _, line := range []string{
		"@endsection", "@endif", "@endforeach", "@endforelse", "@endfor",
		"@endwhile", "@endgo", "@else", "@elseif(.Other)", "@empty",
	} {
		if !decrease.MatchString(line) {
			t.Errorf("%q does not return to its block indentation", line)
		}
	}
}

func hasPair(pairs [][]string, open, close string) bool {
	for _, pair := range pairs {
		if equalPair(pair, open, close) {
			return true
		}
	}
	return false
}

func equalPair(pair []string, open, close string) bool {
	return len(pair) == 2 && pair[0] == open && pair[1] == close
}

package template

import (
	"strings"
	"testing"

	"github.com/flowcmd/cli/internal/types"
)

func TestExtractRefs(t *testing.T) {
	refs, err := ExtractRefs(`echo {{ steps.foo.output }} {{ steps[1].exitcode }} {{ steps.bar.error != '' }}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}
	if refs[0].Name != "foo" || refs[0].Prop != "output" || refs[0].ByIndex {
		t.Errorf("bad ref 0: %+v", refs[0])
	}
	if !refs[1].ByIndex || refs[1].Index != 1 || refs[1].Prop != "exitcode" {
		t.Errorf("bad ref 1: %+v", refs[1])
	}
	if refs[2].Name != "bar" || refs[2].Prop != "error" {
		t.Errorf("bad ref 2: %+v", refs[2])
	}
}

func TestExtractRefs_InvalidRef(t *testing.T) {
	_, err := ExtractRefs(`{{ steps.BADNAME.output }}`)
	if err == nil {
		t.Fatal("expected error for invalid step name")
	}
}

func TestExtractRefs_Empty(t *testing.T) {
	refs, err := ExtractRefs("")
	if err != nil || refs != nil {
		t.Fatalf("expected nil,nil got %v,%v", refs, err)
	}
}

func makeResults() (map[string]*types.StepResult, []string) {
	res := map[string]*types.StepResult{
		"foo": {Name: "foo", Output: "  hello  \n", Error: "", ExitCode: 0},
		"bar": {Name: "bar", Output: "", Error: "oops", ExitCode: 2},
	}
	return res, []string{"foo", "bar"}
}

func TestRender_NamedAndIndexed(t *testing.T) {
	res, order := makeResults()
	out, err := Render(`name={{ steps.foo.output }} idx={{ steps[0].output }} err={{ steps.bar.error }} code={{ steps.bar.exitcode }}`, res, order)
	if err != nil {
		t.Fatal(err)
	}
	want := "name=hello idx=hello err=oops code=2"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
}

func TestRender_OutputTrimmed(t *testing.T) {
	res, order := makeResults()
	out, _ := Render(`[{{ steps.foo.output }}]`, res, order)
	if out != "[hello]" {
		t.Errorf("output should be trimmed; got %q", out)
	}
}

func TestRender_Comparisons(t *testing.T) {
	res, order := makeResults()
	cases := map[string]string{
		`{{ steps.foo.output != '' }}`:  "true",
		`{{ steps.bar.output != '' }}`:  "false",
		`{{ steps.bar.exitcode == 2 }}`: "true",
		`{{ steps.bar.exitcode == 0 }}`: "false",
	}
	for in, want := range cases {
		got, err := Render(in, res, order)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestRender_UnknownStep(t *testing.T) {
	res, order := makeResults()
	_, err := Render(`{{ steps.missing.output }}`, res, order)
	if err == nil || !strings.Contains(err.Error(), "has not executed") {
		t.Errorf("expected has-not-executed error, got %v", err)
	}
}

func TestRender_IndexOutOfRange(t *testing.T) {
	res, order := makeResults()
	_, err := Render(`{{ steps[9].output }}`, res, order)
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected out-of-range error, got %v", err)
	}
}

func TestExtractExpressions(t *testing.T) {
	got, err := ExtractExpressions(`echo {{ steps.a.output }} and {{ steps.b.output != '' }}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"steps.a.output", "steps.b.output != ''"}
	if len(got) != len(want) {
		t.Fatalf("expected %d exprs, got %d (%v)", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("expr %d: got %q want %q", i, got[i], w)
		}
	}
	if out, err := ExtractExpressions(""); err != nil || out != nil {
		t.Errorf("empty input should yield nil/nil, got %v/%v", out, err)
	}
}

func TestRender_UnsupportedExpression(t *testing.T) {
	res, order := makeResults()
	_, err := Render(`{{ a b c d }}`, res, order)
	if err == nil {
		t.Fatal("expected unsupported-expression error")
	}
}

func TestRender_ComparisonLeftSideError(t *testing.T) {
	res, order := makeResults()
	_, err := Render(`{{ steps[9].output == 'x' }}`, res, order)
	if err == nil {
		t.Fatal("expected resolveToken error on lhs")
	}
}

func TestRender_LiteralOnly(t *testing.T) {
	res, order := makeResults()
	out, err := Render(`{{ literal }}`, res, order)
	if err != nil {
		t.Fatal(err)
	}
	if out != "literal" {
		t.Errorf("got %q want %q", out, "literal")
	}
}

func TestEvalBool(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"false": false,
		"0":     false,
		"true":  true,
		"hello": true,
		"  ":    false,
	}
	for in, want := range cases {
		if got := EvalBool(in); got != want {
			t.Errorf("EvalBool(%q)=%v want %v", in, got, want)
		}
	}
}

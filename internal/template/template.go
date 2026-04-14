package template

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/flowcmd/cli/internal/types"
)

// tmplRe matches {{ ... }} with lazy inner content.
var tmplRe = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)

// refRe matches either steps.NAME.PROP or steps[N].PROP where PROP ∈ {output,error,exitcode}.
var refRe = regexp.MustCompile(`^steps(?:\.([a-z][a-z0-9_-]*)|\[(\d+)\])\.(output|error|exitcode)$`)

type Ref struct {
	ByIndex bool
	Index   int
	Name    string
	Prop    string // output | error | exitcode
	Raw     string // full token (e.g. steps.foo.output)
	ExprRaw string // full {{ ... }} inner expression this ref appears in
}

// ExtractRefs scans text for {{ }} blocks and returns parsed refs.
// Supports comparison expressions like `{{ steps.foo.output != ” }}` by extracting
// any step references found inside the expression.
func ExtractRefs(text string) ([]Ref, error) {
	if text == "" {
		return nil, nil
	}
	var refs []Ref
	for _, m := range tmplRe.FindAllStringSubmatch(text, -1) {
		expr := strings.TrimSpace(m[1])
		for _, tok := range tokenize(expr) {
			if !strings.HasPrefix(tok, "steps") {
				continue
			}
			r, err := parseRef(tok)
			if err != nil {
				return nil, err
			}
			r.ExprRaw = expr
			refs = append(refs, r)
		}
	}
	return refs, nil
}

// ExtractExpressions returns each {{ ... }} inner expression (trimmed) in order.
func ExtractExpressions(text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}
	var out []string
	for _, m := range tmplRe.FindAllStringSubmatch(text, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out, nil
}

// tokenize splits an expression into whitespace-separated tokens, stripping quotes.
func tokenize(expr string) []string {
	fields := strings.Fields(expr)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, strings.Trim(f, "'\""))
	}
	return out
}

func parseRef(tok string) (Ref, error) {
	m := refRe.FindStringSubmatch(tok)
	if m == nil {
		return Ref{}, fmt.Errorf("invalid template reference: %q", tok)
	}
	r := Ref{Prop: m[3], Raw: tok}
	if m[1] != "" {
		r.Name = m[1]
	} else {
		n, _ := strconv.Atoi(m[2])
		r.ByIndex = true
		r.Index = n
	}
	return r, nil
}

// Render resolves {{ }} expressions using completed step results.
// results is keyed by step name; order gives index access.
func Render(text string, results map[string]*types.StepResult, order []string) (string, error) {
	var outErr error
	rendered := tmplRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := tmplRe.FindStringSubmatch(match)[1]
		expr := strings.TrimSpace(inner)
		value, err := evalExpr(expr, results, order)
		if err != nil {
			outErr = err
			return match
		}
		return value
	})
	if outErr != nil {
		return "", outErr
	}
	return rendered, nil
}

// evalExpr evaluates either a bare reference or a simple `<ref> <op> <literal>` comparison.
// Supported ops: == !=. Returns "" or "true"/"false" for comparisons.
func evalExpr(expr string, results map[string]*types.StepResult, order []string) (string, error) {
	tokens := tokenize(expr)
	// simple comparison: ref op value
	if len(tokens) == 3 && (tokens[1] == "==" || tokens[1] == "!=") {
		left, err := resolveToken(tokens[0], results, order)
		if err != nil {
			return "", err
		}
		right := tokens[2]
		eq := left == right
		if tokens[1] == "!=" {
			eq = !eq
		}
		if eq {
			return "true", nil
		}
		return "false", nil
	}
	if len(tokens) == 1 {
		return resolveToken(tokens[0], results, order)
	}
	return "", fmt.Errorf("unsupported template expression: %q", expr)
}

func resolveToken(tok string, results map[string]*types.StepResult, order []string) (string, error) {
	if !strings.HasPrefix(tok, "steps") {
		// literal
		return tok, nil
	}
	r, err := parseRef(tok)
	if err != nil {
		return "", err
	}
	var res *types.StepResult
	if r.ByIndex {
		if r.Index < 0 || r.Index >= len(order) {
			return "", fmt.Errorf("template: steps[%d] out of range", r.Index)
		}
		res = results[order[r.Index]]
	} else {
		res = results[r.Name]
	}
	if res == nil {
		return "", fmt.Errorf("template: step %q has not executed", r.Raw)
	}
	switch r.Prop {
	case "output":
		return strings.TrimSpace(res.Output), nil
	case "error":
		return strings.TrimSpace(res.Error), nil
	case "exitcode":
		return strconv.Itoa(res.ExitCode), nil
	}
	return "", fmt.Errorf("template: unknown property %q", r.Prop)
}

// EvalBool evaluates a `when` expression; empty string/"false"/"0" = false, else true.
func EvalBool(rendered string) bool {
	s := strings.TrimSpace(rendered)
	if s == "" || s == "false" || s == "0" {
		return false
	}
	return true
}

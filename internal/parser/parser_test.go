package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flowcmd/cli/internal/types"
)

func writeTmp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "wf.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParse_Valid(t *testing.T) {
	path := writeTmp(t, `
name: Demo
description: Demo flow
steps:
  - name: one
    run: echo one
  - name: two
    run: echo two
    parallel: true
  - name: three
    run: echo "{{ steps.one.output }}"
    when: "{{ steps.one.output != '' }}"
    retry:
      attempts: 2
      delay: 1s
`)
	wf, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if wf.Name != "Demo" || len(wf.Steps) != 3 {
		t.Fatalf("unexpected wf: %+v", wf)
	}
	if !wf.Steps[1].Parallel {
		t.Error("step two should be parallel")
	}
	if wf.Steps[2].Retry == nil || wf.Steps[2].Retry.Attempts != 2 {
		t.Error("retry not parsed")
	}
}

func TestValidate_Errors(t *testing.T) {
	cases := []struct {
		name string
		wf   types.Workflow
		want string
	}{
		{"no name", types.Workflow{Steps: []types.Step{{Name: "a", Run: "x"}}}, "name"},
		{"no steps", types.Workflow{Name: "x"}, "at least one step"},
		{"step missing name", types.Workflow{Name: "x", Steps: []types.Step{{Run: "y"}}}, "'name' is required"},
		{"bad name chars", types.Workflow{Name: "x", Steps: []types.Step{{Name: "Bad_Name", Run: "y"}}}, "must match"},
		{"name too long", types.Workflow{Name: "x", Steps: []types.Step{{Name: strings.Repeat("a", 65), Run: "y"}}}, "64"},
		{"duplicate name", types.Workflow{Name: "x", Steps: []types.Step{{Name: "a", Run: "y"}, {Name: "a", Run: "z"}}}, "duplicate"},
		{"no run", types.Workflow{Name: "x", Steps: []types.Step{{Name: "a"}}}, "'run' is required"},
		{"retry attempts < 1", types.Workflow{Name: "x", Steps: []types.Step{{Name: "a", Run: "y", Retry: &types.Retry{Attempts: 0}}}}, "attempts"},
		{"forward named ref", types.Workflow{Name: "x", Steps: []types.Step{{Name: "a", Run: "echo {{ steps.b.output }}"}, {Name: "b", Run: "y"}}}, "does not precede"},
		{"self ref", types.Workflow{Name: "x", Steps: []types.Step{{Name: "a", Run: "echo {{ steps.a.output }}"}}}, "does not precede"},
		{"unknown named ref", types.Workflow{Name: "x", Steps: []types.Step{{Name: "a", Run: "echo {{ steps.nope.output }}"}}}, "unknown"},
		{"index out of range", types.Workflow{Name: "x", Steps: []types.Step{{Name: "a", Run: "echo {{ steps[5].output }}"}}}, "does not precede"},
		{"bad template", types.Workflow{Name: "x", Steps: []types.Step{{Name: "a", Run: "echo {{ bogus }}"}}}, "invalid template reference"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(&tc.wf)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q should contain %q", err, tc.want)
			}
		})
	}
}

func TestParse_FileNotFound(t *testing.T) {
	if _, err := Parse("/no/such/path.yml"); err == nil {
		t.Fatal("expected read error")
	}
}

func TestParse_BadYAML(t *testing.T) {
	path := writeTmp(t, "name: : :\nbroken")
	if _, err := Parse(path); err == nil {
		t.Fatal("expected yaml error")
	}
}

func TestParse_ValidationErrorPropagates(t *testing.T) {
	// YAML parses fine but workflow has no steps — Validate fails.
	path := writeTmp(t, "name: x\n")
	if _, err := Parse(path); err == nil {
		t.Fatal("expected validation error from Parse")
	}
}

func TestValidate_BadWhenTemplate(t *testing.T) {
	wf := types.Workflow{
		Name: "x",
		Steps: []types.Step{
			{Name: "a", Run: "echo a", When: "{{ steps.BAD.output }}"},
		},
	}
	if err := Validate(&wf); err == nil {
		t.Fatal("expected when-template error")
	}
}

func TestValidate_ValidTemplateRefs(t *testing.T) {
	wf := types.Workflow{
		Name: "x",
		Steps: []types.Step{
			{Name: "a", Run: "echo a"},
			{Name: "b", Run: "echo {{ steps.a.output }} {{ steps[0].exitcode }}"},
			{Name: "c", Run: "echo c", When: "{{ steps.a.output != '' }}"},
		},
	}
	if err := Validate(&wf); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

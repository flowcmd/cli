package parser

import (
	"fmt"
	"os"
	"regexp"

	"github.com/flowcmd/cli/internal/template"
	"github.com/flowcmd/cli/internal/types"
	"gopkg.in/yaml.v3"
)

var stepNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func Parse(path string) (*types.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var wf types.Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if err := Validate(&wf); err != nil {
		return nil, err
	}
	return &wf, nil
}

func Validate(wf *types.Workflow) error {
	if wf.Name == "" {
		return fmt.Errorf("workflow: 'name' is required")
	}
	if len(wf.Steps) == 0 {
		return fmt.Errorf("workflow: at least one step is required")
	}

	// First pass: shape checks + collect all step names for forward-ref detection.
	names := make(map[string]int, len(wf.Steps))
	for i, s := range wf.Steps {
		if s.Name == "" {
			return fmt.Errorf("step %d: 'name' is required", i)
		}
		if len(s.Name) > 64 {
			return fmt.Errorf("step %q: name exceeds 64 characters", s.Name)
		}
		if !stepNameRe.MatchString(s.Name) {
			return fmt.Errorf("step %q: name must match ^[a-z][a-z0-9_-]*$", s.Name)
		}
		if _, dup := names[s.Name]; dup {
			return fmt.Errorf("step %q: duplicate name", s.Name)
		}
		names[s.Name] = i
		if s.Run == "" {
			return fmt.Errorf("step %q: 'run' is required", s.Name)
		}
		if s.Retry != nil && s.Retry.Attempts < 1 {
			return fmt.Errorf("step %q: retry.attempts must be >= 1", s.Name)
		}
	}

	// Second pass: template validation with full name table so forward-refs produce
	// a precedence error rather than an "unknown" error.
	for i, s := range wf.Steps {
		if err := validateTemplate(s.Run, names, i, s.Name, ""); err != nil {
			return err
		}
		if err := validateTemplate(s.When, names, i, s.Name, "when: "); err != nil {
			return err
		}
	}
	return nil
}

func validateTemplate(text string, names map[string]int, curIdx int, curName, label string) error {
	exprs, err := template.ExtractExpressions(text)
	if err != nil {
		return fmt.Errorf("step %q %s: %w", curName, label, err)
	}
	refs, err := template.ExtractRefs(text)
	if err != nil {
		return fmt.Errorf("step %q %s: %w", curName, label, err)
	}
	// If an expression contains no step ref at all, it's a meaningless template.
	for _, ex := range exprs {
		if !containsRef(ex, refs) {
			return fmt.Errorf("step %q %s: invalid template reference: {{ %s }}", curName, label, ex)
		}
	}
	for _, r := range refs {
		if err := checkRef(r, names, curIdx, curName); err != nil {
			return err
		}
	}
	return nil
}

func containsRef(expr string, refs []template.Ref) bool {
	for _, r := range refs {
		if r.ExprRaw == expr {
			return true
		}
	}
	return false
}

func checkRef(r template.Ref, names map[string]int, curIdx int, curName string) error {
	if r.ByIndex {
		if r.Index < 0 || r.Index >= curIdx {
			return fmt.Errorf("step %q: template references steps[%d] which does not precede this step", curName, r.Index)
		}
		return nil
	}
	idx, ok := names[r.Name]
	if !ok {
		return fmt.Errorf("step %q: template references unknown step %q", curName, r.Name)
	}
	if idx >= curIdx {
		return fmt.Errorf("step %q: template references step %q which does not precede this step", curName, r.Name)
	}
	return nil
}

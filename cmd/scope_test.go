package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// helpers below; tests for resolveWorkflow appear after.

// chdirAndFakeHome puts the test into an isolated cwd and overrides HOME so
// global-scope operations land inside the temp dir as well.
func chdirAndFakeHome(t *testing.T) (cwd, home string) {
	t.Helper()
	base := t.TempDir()
	cwd = filepath.Join(base, "cwd")
	home = filepath.Join(base, "home")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	return cwd, home
}

func TestResolveWorkflow_PathPassthrough(t *testing.T) {
	cases := []string{"./x.yml", "dir/x", "x.yml", "x.yaml", "/abs/path/x.yml"}
	for _, in := range cases {
		out, err := resolveWorkflow(in, io.Discard)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if out != in {
			t.Errorf("%s: got %q want passthrough", in, out)
		}
	}
}

func TestResolveWorkflow_LocalOnly(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	dir := filepath.Join(cwd, scopeDirName)
	_ = os.MkdirAll(dir, 0o755)
	writeFile(t, filepath.Join(dir, "hello.yml"), sampleYAML)

	var buf bytes.Buffer
	got, err := resolveWorkflow("hello", &buf)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(localDir(), "hello.yml")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no warning, got %q", buf.String())
	}
}

func TestResolveWorkflow_GlobalOnly(t *testing.T) {
	_, home := chdirAndFakeHome(t)
	dir := filepath.Join(home, scopeDirName)
	_ = os.MkdirAll(dir, 0o755)
	writeFile(t, filepath.Join(dir, "hello.yml"), sampleYAML)

	var buf bytes.Buffer
	got, err := resolveWorkflow("hello", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "hello.yml") {
		t.Errorf("got %q want global path", got)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no warning, got %q", buf.String())
	}
}

func TestResolveWorkflow_BothScopesPrefersLocalAndWarns(t *testing.T) {
	cwd, home := chdirAndFakeHome(t)
	ldir := filepath.Join(cwd, scopeDirName)
	gdir := filepath.Join(home, scopeDirName)
	_ = os.MkdirAll(ldir, 0o755)
	_ = os.MkdirAll(gdir, 0o755)
	writeFile(t, filepath.Join(ldir, "hello.yml"), sampleYAML)
	writeFile(t, filepath.Join(gdir, "hello.yml"), sampleYAML)

	var buf bytes.Buffer
	got, err := resolveWorkflow("hello", &buf)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(localDir(), "hello.yml")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if !strings.Contains(buf.String(), "both") || !strings.Contains(buf.String(), "using local") {
		t.Errorf("warning missing: %q", buf.String())
	}
}

func TestResolveWorkflow_NotFound(t *testing.T) {
	chdirAndFakeHome(t)
	_, err := resolveWorkflow("missing", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestCollectWorkflowNames(t *testing.T) {
	cwd, home := chdirAndFakeHome(t)
	ldir := filepath.Join(cwd, scopeDirName)
	gdir := filepath.Join(home, scopeDirName)
	_ = os.MkdirAll(ldir, 0o755)
	_ = os.MkdirAll(gdir, 0o755)
	writeFile(t, filepath.Join(ldir, "local-only.yml"), sampleYAML)
	writeFile(t, filepath.Join(ldir, "shared.yml"), sampleYAML)
	writeFile(t, filepath.Join(ldir, "notes.txt"), "skip me") // non-yaml, must be ignored
	writeFile(t, filepath.Join(gdir, "global-only.yaml"), sampleYAML)
	writeFile(t, filepath.Join(gdir, "shared.yml"), sampleYAML) // duplicate stem

	names := collectWorkflowNames()
	want := []string{"global-only", "local-only", "shared"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("names[%d]=%q want %q (full: %v)", i, names[i], n, names)
		}
	}
}

func TestCollectWorkflowNames_EmptyScopes(t *testing.T) {
	chdirAndFakeHome(t)
	if got := collectWorkflowNames(); len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestCompleteWorkflowNames_SuggestsStems(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	dir := filepath.Join(cwd, scopeDirName)
	_ = os.MkdirAll(dir, 0o755)
	writeFile(t, filepath.Join(dir, "hello.yml"), sampleYAML)
	writeFile(t, filepath.Join(dir, "deploy.yml"), sampleYAML)

	got, directive := completeWorkflowNames(nil, nil, "")
	if len(got) != 2 {
		t.Fatalf("expected 2 names, got %v", got)
	}
	if directive&cobra.ShellCompDirectiveNoFileComp == 0 {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
}

func TestCompleteWorkflowNames_SecondArgNoSuggestions(t *testing.T) {
	// Commands take a single arg; once one is provided, completion should
	// return nothing rather than repeating the list.
	got, _ := completeWorkflowNames(nil, []string{"already"}, "")
	if got != nil {
		t.Errorf("expected nil after first arg filled, got %v", got)
	}
}

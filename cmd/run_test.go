package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flowcmd/cli/internal/engine"
	"github.com/flowcmd/cli/internal/parser"
	"github.com/flowcmd/cli/internal/types"
)

func resetRunFlags(t *testing.T) {
	t.Helper()
	runVerbose, runDryRun, runNoTUI = false, false, false
	t.Cleanup(func() { runVerbose, runDryRun, runNoTUI = false, false, false })
}

func writeWorkflow(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wf.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const dryRunWorkflow = `name: Dry
description: A dry-run workflow.
steps:
  - name: a
    run: echo a
  - name: b
    run: |
      echo b
      echo more
    parallel: true
  - name: c
    run: echo c
    parallel: true
`

const passingWorkflow = `name: Pass
steps:
  - name: a
    run: echo hello
  - name: b
    run: echo world
    when: "{{ steps.a.output != '' }}"
`

const failingWorkflow = `name: Fail
steps:
  - name: oops
    run: bash -c "echo to-stderr 1>&2; exit 7"
`

const skipWorkflow = `name: Skip
steps:
  - name: a
    run: printf ''
  - name: b
    run: echo unreachable
    when: "{{ steps.a.output != '' }}"
`

func TestRun_DryRunPrintsPlan(t *testing.T) {
	resetRunFlags(t)
	runDryRun = true
	wf := writeWorkflow(t, dryRunWorkflow)
	out := captureStdout(t, func() {
		if err := runCmd.RunE(runCmd, []string{wf}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Workflow: Dry") {
		t.Errorf("missing title: %q", out)
	}
	if !strings.Contains(out, "‖ b") {
		t.Errorf("parallel marker missing: %q", out)
	}
	if !strings.Contains(out, "1.   a") {
		t.Errorf("sequential row missing: %q", out)
	}
}

func TestRun_NoTUIPlainSuccess(t *testing.T) {
	resetRunFlags(t)
	runNoTUI = true
	wf := writeWorkflow(t, passingWorkflow)
	out := captureStdout(t, func() {
		if err := runCmd.RunE(runCmd, []string{wf}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "✓ a") || !strings.Contains(out, "✓ b") {
		t.Errorf("missing completion lines: %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("missing last-line preview: %q", out)
	}
}

func TestRun_NoTUIVerboseEmitsFullOutput(t *testing.T) {
	resetRunFlags(t)
	runNoTUI = true
	runVerbose = true
	wf := writeWorkflow(t, dryRunWorkflow)
	out := captureStdout(t, func() {
		_ = runCmd.RunE(runCmd, []string{wf})
	})
	// Verbose path indents output with 4 spaces.
	if !strings.Contains(out, "    more") {
		t.Errorf("verbose indented output missing: %q", out)
	}
}

func TestRun_NoTUIFailurePropagates(t *testing.T) {
	resetRunFlags(t)
	runNoTUI = true
	wf := writeWorkflow(t, failingWorkflow)
	out := captureStdout(t, func() {
		err := runCmd.RunE(runCmd, []string{wf})
		if err == nil {
			t.Fatal("expected failure")
		}
		if !strings.Contains(err.Error(), "workflow failed") {
			t.Errorf("expected wrapped error, got %v", err)
		}
	})
	if !strings.Contains(out, "✗ oops") {
		t.Errorf("missing failure marker: %q", out)
	}
	if !strings.Contains(out, "to-stderr") {
		t.Errorf("missing stderr in output: %q", out)
	}
}

func TestRun_NoTUISkipsWhenFalse(t *testing.T) {
	resetRunFlags(t)
	runNoTUI = true
	wf := writeWorkflow(t, skipWorkflow)
	out := captureStdout(t, func() {
		_ = runCmd.RunE(runCmd, []string{wf})
	})
	if !strings.Contains(out, "○ b (skipped)") {
		t.Errorf("missing skipped marker: %q", out)
	}
}

func TestRun_ResolvesNameFromLocalScope(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	resetRunFlags(t)
	runNoTUI = true

	dir := filepath.Join(cwd, scopeDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "myflow.yml"), passingWorkflow)

	out := captureStdout(t, func() {
		if err := runCmd.RunE(runCmd, []string{"myflow"}); err != nil {
			t.Fatalf("run by name: %v", err)
		}
	})
	if !strings.Contains(out, "✓ a") || !strings.Contains(out, "✓ b") {
		t.Errorf("expected completion lines, got %q", out)
	}
}

func TestRun_ParseError(t *testing.T) {
	resetRunFlags(t)
	err := runCmd.RunE(runCmd, []string{"/no/such/file.yml"})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

// printEvent retry branch: a running event with Attempts>1 must NOT print the spinner line.
func TestPrintEvent_RetryAttemptSilent(t *testing.T) {
	out := captureStdout(t, func() {
		printEvent(engine.Event{
			Step: "x",
			Result: &types.StepResult{
				Name:     "x",
				State:    types.StateRunning,
				Attempts: 2,
			},
		}, false)
	})
	if out != "" {
		t.Errorf("retry running event should be silent, got %q", out)
	}
}

// printEvent: completed step with empty output should not print the ╰─ prefix.
func TestPrintEvent_CompletedNoOutput(t *testing.T) {
	out := captureStdout(t, func() {
		printEvent(engine.Event{
			Step: "x",
			Result: &types.StepResult{
				Name:    "x",
				State:   types.StateCompleted,
				Output:  "",
				Started: time.Now().Add(-time.Millisecond),
			},
		}, false)
	})
	if !strings.Contains(out, "✓ x") {
		t.Errorf("missing completion line: %q", out)
	}
	if strings.Contains(out, "╰─") {
		t.Errorf("should not print preview line for empty output: %q", out)
	}
}

// printPlan: workflow without description omits the description line.
func TestPrintPlan_NoDescription(t *testing.T) {
	out := captureStdout(t, func() {
		printPlan(&types.Workflow{
			Name:  "X",
			Steps: []types.Step{{Name: "only", Run: "echo only"}},
		})
	})
	if !strings.Contains(out, "Workflow: X") {
		t.Errorf("title missing: %q", out)
	}
	// Should be exactly one indented line per step + header — no description line
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (header + 1 step), got %d: %q", len(lines), out)
	}
}

// runTUI: drive bubbletea with piped stdin/stdout so it does not require a TTY.
func TestRun_TUIEndToEnd(t *testing.T) {
	resetRunFlags(t)
	wfPath := writeWorkflow(t, passingWorkflow)
	wf, err := parser.Parse(wfPath)
	if err != nil {
		t.Fatal(err)
	}
	pr, pw := io.Pipe()
	defer pw.Close()
	var stdout bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	eng := engine.New(wf)

	done := make(chan error, 1)
	go func() {
		done <- runTUI(ctx, wf, eng, tea.WithInput(pr), tea.WithOutput(&stdout))
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runTUI: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("runTUI did not return within 10s")
	}
	if !strings.Contains(stdout.String(), "Pass") {
		t.Errorf("TUI output missing workflow name: %q", stdout.String())
	}
}

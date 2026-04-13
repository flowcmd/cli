package engine

import (
	"context"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flowcmd/cli/internal/types"
)

func drain(e *Engine) []*types.StepResult {
	var out []*types.StepResult
	for ev := range e.Events {
		out = append(out, ev.Result)
	}
	return out
}

func runAndDrain(t *testing.T, wf *types.Workflow) (*Engine, []*types.StepResult, error) {
	t.Helper()
	e := New(wf)
	evs := make(chan []*types.StepResult, 1)
	go func() { evs <- drain(e) }()
	err := e.Run(context.Background())
	return e, <-evs, err
}

func TestEngine_Sequential(t *testing.T) {
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "a", Run: "echo hello"},
			{Name: "b", Run: "echo {{ steps.a.output }}"},
		},
	}
	_, events, err := runAndDrain(t, wf)
	if err != nil {
		t.Fatal(err)
	}
	// Expect running+completed per step
	if len(events) < 4 {
		t.Fatalf("expected >=4 events, got %d", len(events))
	}
	final := map[string]*types.StepResult{}
	for _, e := range events {
		final[e.Name] = e
	}
	if final["b"].Output != "hello\n" {
		t.Errorf("templated output wrong: %q", final["b"].Output)
	}
}

func TestEngine_ParallelExecutesConcurrently(t *testing.T) {
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "a", Run: "sleep 0.2", Parallel: true},
			{Name: "b", Run: "sleep 0.2", Parallel: true},
			{Name: "c", Run: "sleep 0.2", Parallel: true},
		},
	}
	start := time.Now()
	_, _, err := runAndDrain(t, wf)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Errorf("parallel steps should run concurrently; took %v", elapsed)
	}
}

func TestEngine_WhenSkip(t *testing.T) {
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "a", Run: "echo ''"},
			{Name: "b", Run: "echo should-not-run", When: "{{ steps.a.output != '' }}"},
		},
	}
	e, _, err := runAndDrain(t, wf)
	if err != nil {
		t.Fatal(err)
	}
	res := e.Results()
	if res["b"].State != types.StateSkipped {
		t.Errorf("step b should be skipped, got %s", res["b"].State)
	}
}

func TestEngine_RetrySucceedsSecondAttempt(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "flag")
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{
				Name: "flaky",
				Run:  "if [ -f " + tmp + " ]; then echo ok; else touch " + tmp + "; exit 1; fi",
				Retry: &types.Retry{Attempts: 3, Delay: 10 * time.Millisecond},
			},
		},
	}
	e, _, err := runAndDrain(t, wf)
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	r := e.Results()["flaky"]
	if r.State != types.StateCompleted {
		t.Errorf("state=%s", r.State)
	}
	if r.Attempts != 2 {
		t.Errorf("attempts=%d want 2", r.Attempts)
	}
}

func TestEngine_RetryExhaustion(t *testing.T) {
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "nope", Run: "exit 7", Retry: &types.Retry{Attempts: 2, Delay: 1 * time.Millisecond}},
		},
	}
	e, _, err := runAndDrain(t, wf)
	if err == nil {
		t.Fatal("expected err")
	}
	r := e.Results()["nope"]
	if r.State != types.StateFailed || r.ExitCode != 7 {
		t.Errorf("wrong final result: %+v", r)
	}
}

func TestEngine_FailFastSequential(t *testing.T) {
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "a", Run: "exit 1"},
			{Name: "b", Run: "echo should-not-run"},
		},
	}
	e, _, err := runAndDrain(t, wf)
	if err == nil {
		t.Fatal("expected err")
	}
	if _, ran := e.Results()["b"]; ran {
		t.Errorf("step b should not have run")
	}
}

func TestEngine_ParallelCancellationOnFailure(t *testing.T) {
	// One fast-failing step, one long sleeper: sleeper should be cancelled.
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "fast-fail", Run: "exit 1", Parallel: true},
			{Name: "slow", Run: "sleep 5", Parallel: true},
		},
	}
	start := time.Now()
	_, _, err := runAndDrain(t, wf)
	if err == nil {
		t.Fatal("expected err")
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("slow step should have been cancelled; elapsed=%v", time.Since(start))
	}
}

func TestEngine_TemplateOutputTrimmed(t *testing.T) {
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "a", Run: "printf 'hello\\n'"},
			{Name: "b", Run: "printf '[%s]' \"{{ steps.a.output }}\""},
		},
	}
	e, _, err := runAndDrain(t, wf)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Results()["b"].Output; got != "[hello]" {
		t.Errorf("output=%q want [hello]", got)
	}
}

func TestEngine_RetryDelayCancelled(t *testing.T) {
	// Step always fails; long retry delay; cancel ctx mid-delay → ctx.Err returned.
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "fail", Run: "exit 1", Retry: &types.Retry{Attempts: 5, Delay: 2 * time.Second}},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := New(wf)
	go func() {
		for range e.Events {
		}
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := e.Run(ctx)
	if err == nil {
		t.Fatal("expected ctx error")
	}
	if time.Since(start) > 1500*time.Millisecond {
		t.Errorf("ctx cancel during delay didn't shortcut: %v", time.Since(start))
	}
}

func TestEngine_TemplateRenderErrorInWhen(t *testing.T) {
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "a", Run: "echo a"},
			{Name: "b", Run: "echo b", When: "{{ steps[9].output }}"},
		},
	}
	e := New(wf)
	go func() {
		for range e.Events {
		}
	}()
	if err := e.Run(context.Background()); err == nil {
		t.Fatal("expected render error")
	}
}

func TestEngine_TemplateRenderErrorInRun(t *testing.T) {
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "a", Run: "echo {{ steps[9].output }}"},
		},
	}
	e := New(wf)
	go func() {
		for range e.Events {
		}
	}()
	if err := e.Run(context.Background()); err == nil {
		t.Fatal("expected run-template error")
	}
}

func TestEngine_ExecShellNonExitError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, err := execShell(ctx, "echo never")
	if err == nil {
		t.Error("expected error when ctx is already cancelled")
	}
}

func TestEngine_GroupSteps(t *testing.T) {
	steps := []types.Step{
		{Name: "a"},
		{Name: "b", Parallel: true},
		{Name: "c", Parallel: true},
		{Name: "d"},
	}
	groups := groupSteps(steps)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if len(groups[1]) != 2 {
		t.Errorf("middle group should have 2 parallel steps, got %d", len(groups[1]))
	}
}

func TestEngine_ContextCancellation(t *testing.T) {
	wf := &types.Workflow{
		Name: "t",
		Steps: []types.Step{
			{Name: "a", Run: "sleep 5"},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := New(wf)
	var drained atomic.Bool
	go func() {
		for range e.Events {
		}
		drained.Store(true)
	}()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := e.Run(ctx)
	if err == nil {
		t.Fatal("expected err on cancel")
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("cancel should have stopped sleep; elapsed=%v", time.Since(start))
	}
	// give drain goroutine a moment
	time.Sleep(50 * time.Millisecond)
	if !drained.Load() {
		t.Error("events channel should be closed")
	}
	_ = strings.Contains
}

package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/flowcmd/cli/internal/template"
	"github.com/flowcmd/cli/internal/types"
)

type Event struct {
	Step   string
	Result *types.StepResult
}

type Engine struct {
	Workflow *types.Workflow
	Events   chan Event

	mu      sync.Mutex
	results map[string]*types.StepResult
	order   []string
}

func New(wf *types.Workflow) *Engine {
	order := make([]string, len(wf.Steps))
	for i, s := range wf.Steps {
		order[i] = s.Name
	}
	return &Engine{
		Workflow: wf,
		Events:   make(chan Event, len(wf.Steps)*4),
		results:  make(map[string]*types.StepResult, len(wf.Steps)),
		order:    order,
	}
}

func (e *Engine) Results() map[string]*types.StepResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]*types.StepResult, len(e.results))
	for k, v := range e.results {
		out[k] = v
	}
	return out
}

func (e *Engine) setResult(r *types.StepResult) {
	snapshot := *r
	e.mu.Lock()
	e.results[r.Name] = &snapshot
	e.mu.Unlock()
	e.Events <- Event{Step: r.Name, Result: &snapshot}
}

func (e *Engine) Run(ctx context.Context) error {
	defer close(e.Events)

	groups := groupSteps(e.Workflow.Steps)
	for _, g := range groups {
		if err := e.runGroup(ctx, g); err != nil {
			return err
		}
	}
	return nil
}

func groupSteps(steps []types.Step) [][]types.Step {
	var groups [][]types.Step
	var cur []types.Step
	for _, s := range steps {
		if s.Parallel {
			cur = append(cur, s)
			continue
		}
		if len(cur) > 0 {
			groups = append(groups, cur)
			cur = nil
		}
		groups = append(groups, []types.Step{s})
	}
	if len(cur) > 0 {
		groups = append(groups, cur)
	}
	return groups
}

func (e *Engine) runGroup(ctx context.Context, steps []types.Step) error {
	if len(steps) == 1 && !steps[0].Parallel {
		return e.runStep(ctx, steps[0])
	}
	groupCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, s := range steps {
		wg.Add(1)
		go func(s types.Step) {
			defer wg.Done()
			if err := e.runStep(groupCtx, s); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				cancel()
			}
		}(s)
	}
	wg.Wait()
	return firstErr
}

func (e *Engine) runStep(ctx context.Context, s types.Step) error {
	res := &types.StepResult{Name: s.Name, State: types.StateRunning, Started: time.Now()}
	e.setResult(res)

	snapshot := e.Results()

	if s.When != "" {
		rendered, err := template.Render(s.When, snapshot, e.order)
		if err != nil {
			return e.fail(res, err, -1)
		}
		if !template.EvalBool(rendered) {
			res.State = types.StateSkipped
			res.Finished = time.Now()
			e.setResult(res)
			return nil
		}
	}

	cmdText, err := template.Render(s.Run, snapshot, e.order)
	if err != nil {
		return e.fail(res, err, -1)
	}

	attempts := 1
	var delay time.Duration
	if s.Retry != nil {
		attempts = s.Retry.Attempts
		delay = s.Retry.Delay
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		res.Attempts = attempt
		stdout, stderr, code, runErr := execShell(ctx, cmdText)
		res.Output = stdout
		res.Error = stderr
		res.ExitCode = code
		if runErr == nil && code == 0 {
			res.State = types.StateCompleted
			res.Finished = time.Now()
			e.setResult(res)
			return nil
		}
		lastErr = runErr
		if attempt < attempts {
			e.setResult(res)
			if delay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
			}
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("step %q exited with code %d", s.Name, res.ExitCode)
	}
	return e.fail(res, lastErr, res.ExitCode)
}

func (e *Engine) fail(res *types.StepResult, err error, code int) error {
	res.State = types.StateFailed
	if res.Error == "" && err != nil {
		res.Error = err.Error()
	}
	if code >= 0 {
		res.ExitCode = code
	}
	res.Finished = time.Now()
	e.setResult(res)
	return err
}

func execShell(ctx context.Context, cmd string) (stdout, stderr string, code int, err error) {
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	var so, se bytes.Buffer
	c.Stdout = &so
	c.Stderr = &se
	configureCmd(c)
	runErr := c.Run()
	stdout = so.String()
	stderr = se.String()
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			return stdout, stderr, ee.ExitCode(), nil
		}
		return stdout, stderr, -1, runErr
	}
	return stdout, stderr, 0, nil
}

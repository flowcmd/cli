package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flowcmd/cli/internal/engine"
	"github.com/flowcmd/cli/internal/types"
)

func testWorkflow() *types.Workflow {
	return &types.Workflow{
		Name:        "Demo",
		Description: "demo desc",
		Steps: []types.Step{
			{Name: "one", Description: "first"},
			{Name: "two", Description: "second", Parallel: true},
		},
	}
}

func TestView_InitialPending(t *testing.T) {
	wf := testWorkflow()
	events := make(chan engine.Event)
	m := NewModel(wf, events, false)
	v := m.View()
	if !strings.Contains(v, "Demo") {
		t.Error("title not rendered")
	}
	if !strings.Contains(v, "demo desc") {
		t.Error("description not rendered")
	}
	if !strings.Contains(v, "one") || !strings.Contains(v, "two") {
		t.Error("step names not rendered")
	}
	if !strings.Contains(v, "‖") {
		t.Error("parallel marker not rendered")
	}
	close(events)
}

func TestUpdate_EventTransitions(t *testing.T) {
	wf := testWorkflow()
	events := make(chan engine.Event, 4)
	m := NewModel(wf, events, false)

	running := &types.StepResult{Name: "one", State: types.StateRunning, Started: time.Now()}
	updated, _ := m.Update(EventMsg{Step: "one", Result: running})
	m = updated.(Model)
	if m.results["one"].State != types.StateRunning {
		t.Errorf("state not updated")
	}

	completed := &types.StepResult{Name: "one", State: types.StateCompleted, Output: "hello\nworld\n", Started: running.Started, Finished: time.Now()}
	updated, _ = m.Update(EventMsg{Step: "one", Result: completed})
	m = updated.(Model)

	v := m.View()
	if !strings.Contains(v, "✓") {
		t.Error("completed icon not rendered")
	}
	if !strings.Contains(v, "world") {
		t.Errorf("last line of output not shown: %s", v)
	}

	failed := &types.StepResult{Name: "two", State: types.StateFailed, Error: "boom", ExitCode: 1, Started: time.Now()}
	updated, _ = m.Update(EventMsg{Step: "two", Result: failed})
	m = updated.(Model)
	if !strings.Contains(m.View(), "✗") {
		t.Error("failed icon not rendered")
	}
	if !strings.Contains(m.View(), "boom") {
		t.Error("stderr not shown on fail")
	}

	updated, cmd := m.Update(DoneMsg{})
	m = updated.(Model)
	if !m.done {
		t.Error("done not set")
	}
	if cmd == nil {
		t.Error("expected quit cmd")
	}
}

func TestUpdate_WindowSize(t *testing.T) {
	m := NewModel(testWorkflow(), make(chan engine.Event), false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = updated.(Model)
	if m.width != 200 {
		t.Errorf("width not updated: %d", m.width)
	}
}

func TestUpdate_CtrlCQuits(t *testing.T) {
	m := NewModel(testWorkflow(), make(chan engine.Event), false)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("ctrl-c should emit quit cmd")
	}
}

func TestInitReturnsCmd(t *testing.T) {
	m := NewModel(testWorkflow(), make(chan engine.Event, 1), false)
	if cmd := m.Init(); cmd == nil {
		t.Error("Init should return a tea.Cmd")
	}
}

func TestWaitForEvent_ChannelClosed(t *testing.T) {
	ch := make(chan engine.Event)
	close(ch)
	msg := waitForEvent(ch)()
	if _, ok := msg.(DoneMsg); !ok {
		t.Errorf("expected DoneMsg on closed channel, got %T", msg)
	}
}

func TestWaitForEvent_DeliversEvent(t *testing.T) {
	ch := make(chan engine.Event, 1)
	ch <- engine.Event{Step: "x", Result: &types.StepResult{Name: "x", State: types.StateRunning}}
	msg := waitForEvent(ch)()
	ev, ok := msg.(EventMsg)
	if !ok || ev.Step != "x" {
		t.Errorf("expected EventMsg{Step:x}, got %T %v", msg, msg)
	}
}

func TestUpdate_SkippedAndVerboseOutput(t *testing.T) {
	wf := testWorkflow()
	m := NewModel(wf, make(chan engine.Event, 1), true)

	skipped := &types.StepResult{Name: "one", State: types.StateSkipped, Started: time.Now(), Finished: time.Now()}
	updated, _ := m.Update(EventMsg{Step: "one", Result: skipped})
	m = updated.(Model)
	v := m.View()
	if !strings.Contains(v, "○") {
		t.Errorf("skipped icon missing: %q", v)
	}

	verbose := &types.StepResult{Name: "two", State: types.StateCompleted, Output: "line1\nline2\n", Started: time.Now()}
	updated, _ = m.Update(EventMsg{Step: "two", Result: verbose})
	m = updated.(Model)
	if !strings.Contains(m.View(), "line1") || !strings.Contains(m.View(), "line2") {
		t.Errorf("verbose mode should show all output lines: %q", m.View())
	}
}

func TestUpdate_NonCtrlCKeyIgnored(t *testing.T) {
	m := NewModel(testWorkflow(), make(chan engine.Event), false)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("non-ctrl-c key should be a no-op, got cmd %v", cmd)
	}
}

func TestUpdate_SpinnerTick(t *testing.T) {
	m := NewModel(testWorkflow(), make(chan engine.Event), false)
	updated, cmd := m.Update(m.spinner.Tick())
	_ = updated.(Model)
	if cmd == nil {
		t.Error("spinner tick should produce a follow-up cmd")
	}
}

func TestUpdate_UnknownMsgIsNoop(t *testing.T) {
	m := NewModel(testWorkflow(), make(chan engine.Event), false)
	_, cmd := m.Update("some-other-message")
	if cmd != nil {
		t.Errorf("unknown msg should produce no cmd, got %v", cmd)
	}
}

func TestStateDisplay_AllStates(t *testing.T) {
	m := NewModel(testWorkflow(), make(chan engine.Event), false)
	cases := map[types.StepState]string{
		types.StateRunning:   "",
		types.StateCompleted: "✓",
		types.StateFailed:    "✗",
		types.StateSkipped:   "○",
		types.StatePending:   "○",
	}
	for st, wantIcon := range cases {
		icon, _ := m.stateDisplay(st)
		if wantIcon != "" && icon != wantIcon {
			t.Errorf("state %v: icon=%q want %q", st, icon, wantIcon)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("no trunc: %q", got)
	}
	if got := truncate("helloworld", 6); got != "hello…" {
		t.Errorf("trunc: %q", got)
	}
}

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flowcmd/cli/internal/engine"
	"github.com/flowcmd/cli/internal/textutil"
	"github.com/flowcmd/cli/internal/types"
)

type EventMsg engine.Event
type DoneMsg struct{}

const defaultWidth = 100

type Model struct {
	workflow   *types.Workflow
	stepByName map[string]types.Step
	results    map[string]*types.StepResult
	verbose    bool
	spinner    spinner.Model
	events     <-chan engine.Event
	width      int
	done       bool
}

func NewModel(wf *types.Workflow, events <-chan engine.Event, verbose bool) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	byName := make(map[string]types.Step, len(wf.Steps))
	results := make(map[string]*types.StepResult, len(wf.Steps))
	for _, s := range wf.Steps {
		byName[s.Name] = s
		results[s.Name] = &types.StepResult{Name: s.Name, State: types.StatePending}
	}
	return Model{
		workflow:   wf,
		stepByName: byName,
		results:    results,
		verbose:    verbose,
		spinner:    sp,
		events:     events,
		width:      defaultWidth,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForEvent(m.events))
}

func waitForEvent(ch <-chan engine.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return DoneMsg{}
		}
		return EventMsg(ev)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case EventMsg:
		m.results[msg.Step] = msg.Result
		return m, waitForEvent(m.events)
	case DoneMsg:
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.workflow.Name))
	b.WriteByte('\n')
	if m.workflow.Description != "" {
		b.WriteString(descStyle.Render(m.workflow.Description))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	for _, s := range m.workflow.Steps {
		b.WriteString(m.renderStep(s, m.results[s.Name]))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) renderStep(s types.Step, r *types.StepResult) string {
	icon, style := m.stateDisplay(r.State)

	prefix := " "
	if s.Parallel {
		prefix = "‖"
	}
	line := fmt.Sprintf("%s %s %-20s", prefix, icon, s.Name)
	if s.Description != "" {
		line += "  " + s.Description
	}
	if !r.Started.IsZero() {
		line += "  " + timeStyle.Render(r.Duration().Round(time.Millisecond).String())
	}

	styled := style.Render(line)
	switch {
	case r.State == types.StateCompleted && r.Output != "":
		if m.verbose {
			return styled + "\n" + outStyle.Render(textutil.Indent(r.Output, "     "))
		}
		if last := textutil.LastLine(r.Output); last != "" {
			return styled + "\n" + outStyle.Render("   ╰─ "+truncate(last, m.width-6))
		}
	case r.State == types.StateFailed && r.Error != "":
		return styled + "\n" + failStyle.Render(textutil.Indent(r.Error, "     "))
	}
	return styled
}

func (m Model) stateDisplay(st types.StepState) (string, lipgloss.Style) {
	switch st {
	case types.StateRunning:
		return m.spinner.View(), runningStyle
	case types.StateCompleted:
		return "✓", doneStyle
	case types.StateFailed:
		return "✗", failStyle
	case types.StateSkipped:
		return "○", skipStyle
	}
	return "○", pendingStyle
}

func truncate(s string, n int) string {
	if n <= 1 || lipgloss.Width(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flowcmd/cli/internal/engine"
	"github.com/flowcmd/cli/internal/parser"
	"github.com/flowcmd/cli/internal/textutil"
	"github.com/flowcmd/cli/internal/tui"
	"github.com/flowcmd/cli/internal/types"
	"github.com/spf13/cobra"
)

var (
	runVerbose bool
	runDryRun  bool
	runNoTUI   bool
)

var runCmd = &cobra.Command{
	Use:   "run <name-or-path>",
	Short: "Execute a workflow by name (looks up .flowcmd/) or by file path",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := resolveWorkflow(args[0], os.Stderr)
		if err != nil {
			return err
		}
		wf, err := parser.Parse(path)
		if err != nil {
			return err
		}

		if runDryRun {
			printPlan(wf)
			return nil
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		eng := engine.New(wf)
		if runNoTUI {
			err = runPlain(ctx, eng)
		} else {
			err = runTUI(ctx, wf, eng)
		}
		if err != nil {
			return fmt.Errorf("workflow failed: %w", err)
		}
		return nil
	},
}

func runPlain(ctx context.Context, eng *engine.Engine) error {
	done := make(chan struct{})
	go func() {
		for ev := range eng.Events {
			printEvent(ev, runVerbose)
		}
		close(done)
	}()
	err := eng.Run(ctx)
	<-done
	return err
}

func runTUI(ctx context.Context, wf *types.Workflow, eng *engine.Engine, opts ...tea.ProgramOption) error {
	errCh := make(chan error, 1)
	go func() { errCh <- eng.Run(ctx) }()

	p := tea.NewProgram(tui.NewModel(wf, eng.Events, runVerbose), opts...)
	if _, err := p.Run(); err != nil {
		return err
	}
	return <-errCh
}

func printPlan(wf *types.Workflow) {
	fmt.Printf("Workflow: %s\n", wf.Name)
	if wf.Description != "" {
		fmt.Printf("  %s\n", wf.Description)
	}
	for i, s := range wf.Steps {
		marker := " "
		if s.Parallel {
			marker = "‖"
		}
		first, _, _ := strings.Cut(s.Run, "\n")
		fmt.Printf("  %2d. %s %s — %s\n", i+1, marker, s.Name, first)
	}
}

func printEvent(ev engine.Event, verbose bool) {
	r := ev.Result
	switch r.State {
	case types.StateRunning:
		if r.Attempts <= 1 {
			fmt.Printf("● %s\n", r.Name)
		}
	case types.StateCompleted:
		fmt.Printf("✓ %s (%s)\n", r.Name, r.Duration().Round(1e6))
		if verbose && r.Output != "" {
			fmt.Println(textutil.Indent(r.Output, "    "))
		} else if last := textutil.LastLine(r.Output); last != "" {
			fmt.Printf("  ╰─ %s\n", last)
		}
	case types.StateFailed:
		fmt.Printf("✗ %s (exit %d)\n", r.Name, r.ExitCode)
		if r.Error != "" {
			fmt.Println(textutil.Indent(r.Error, "    "))
		}
	case types.StateSkipped:
		fmt.Printf("○ %s (skipped)\n", r.Name)
	}
}

func init() {
	runCmd.Flags().BoolVarP(&runVerbose, "verbose", "v", false, "Show full step output inline")
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Validate and show execution plan without executing")
	runCmd.Flags().BoolVar(&runNoTUI, "no-tui", false, "Plain text output (for CI/piping)")
	rootCmd.AddCommand(runCmd)
}

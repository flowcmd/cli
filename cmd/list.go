package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flowcmd/cli/internal/parser"
	"github.com/spf13/cobra"
)

var (
	listGlobal bool
	listLocal  bool
)

type flowEntry struct {
	name  string // filename stem (e.g. "hello")
	title string // workflow.Name from YAML
	steps int
	err   error // set if parse failed
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List workflows in local and/or global scope",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		showLocal := !listGlobal || listLocal
		showGlobal := !listLocal || listGlobal

		globalPath, err := globalDir()
		if err != nil {
			return err
		}

		anyPrinted := false

		if showGlobal {
			entries := scanFlows(globalPath)
			if len(entries) > 0 {
				if anyPrinted {
					fmt.Println()
				}
				fmt.Println("global")
				printEntries(entries)
				anyPrinted = true
			}
		}

		if showLocal {
			entries := scanFlows(localDir())
			if len(entries) > 0 {
				if anyPrinted {
					fmt.Println()
				}
				fmt.Println("local")
				printEntries(entries)
				anyPrinted = true
			}
		}

		if !anyPrinted {
			fmt.Println("no workflows found")
		}
		return nil
	},
}

func scanFlows(dir string) []flowEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return []flowEntry{{name: dir, err: err}}
	}
	var out []flowEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yml" && ext != ".yaml" {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), ext)
		path := filepath.Join(dir, e.Name())
		wf, err := parser.Parse(path)
		if err != nil {
			out = append(out, flowEntry{name: stem, err: err})
			continue
		}
		out = append(out, flowEntry{name: stem, title: wf.Name, steps: len(wf.Steps)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func printEntries(entries []flowEntry) {
	for _, e := range entries {
		if e.err != nil {
			fmt.Printf("- %s — (invalid: %v)\n", e.name, e.err)
			continue
		}
		fmt.Printf("- %s — %s [%d steps]\n", e.name, e.title, e.steps)
	}
}

func init() {
	listCmd.Flags().BoolVarP(&listGlobal, "global", "g", false, "Show only global scope (~/.flowcmd)")
	listCmd.Flags().BoolVarP(&listLocal, "local", "l", false, "Show only local scope (./.flowcmd)")
	rootCmd.AddCommand(listCmd)
}

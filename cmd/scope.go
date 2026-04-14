package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const scopeDirName = ".flowcmd"

func localDir() string { return scopeDirName }

func globalDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, scopeDirName), nil
}

func scopeDir(global bool) (string, error) {
	if global {
		return globalDir()
	}
	return localDir(), nil
}

func isYAMLExt(ext string) bool { return ext == ".yml" || ext == ".yaml" }

func ensureYAMLExt(name string) string {
	if isYAMLExt(filepath.Ext(name)) {
		return name
	}
	return name + ".yml"
}

// validateWorkflowName rejects path separators and dotfile prefixes.
func validateWorkflowName(name string) error {
	if name == "" {
		return errors.New("workflow name is empty")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("workflow name %q must not contain path separators", name)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("workflow name %q must not start with '.'", name)
	}
	return nil
}

// looksLikePath returns true when arg should be treated as a literal file path
// rather than a short workflow name.
func looksLikePath(arg string) bool {
	if strings.ContainsAny(arg, `/\`) {
		return true
	}
	return isYAMLExt(filepath.Ext(arg))
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// completeWorkflowNames is a cobra ValidArgsFunction that completes the first
// positional argument of commands that take a single workflow name. Runs on
// every TAB press in a completion-enabled shell; must be fast and silent.
func completeWorkflowNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return collectWorkflowNames(), cobra.ShellCompDirectiveNoFileComp
}

// collectWorkflowNames returns the sorted, de-duplicated union of workflow
// names (filename stems) across the local and global scopes. Used for shell
// tab-completion of `run`, `remove`, and `validate` arguments.
//
// Never returns an error: completion must degrade silently. Missing scope
// directories and unreadable files are skipped.
func collectWorkflowNames() []string {
	seen := make(map[string]struct{})
	addFrom := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !isYAMLExt(filepath.Ext(e.Name())) {
				continue
			}
			stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			seen[stem] = struct{}{}
		}
	}
	addFrom(localDir())
	if gdir, err := globalDir(); err == nil {
		addFrom(gdir)
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// resolveWorkflow turns a `flowcmd run` argument into a concrete file path.
// Path-like args pass through unchanged. Bare names resolve to ./.flowcmd/<name>.yml
// first, then ~/.flowcmd/<name>.yml. If both scopes have the name, the local
// copy wins and a warning is written to stderr.
func resolveWorkflow(arg string, stderr io.Writer) (string, error) {
	if looksLikePath(arg) {
		return arg, nil
	}
	name := arg + ".yml"
	local := filepath.Join(localDir(), name)
	gdir, err := globalDir()
	if err != nil {
		return "", err
	}
	globalPath := filepath.Join(gdir, name)

	if fileExists(local) {
		if fileExists(globalPath) {
			fmt.Fprintf(stderr, "warning: %q exists in both local and global scope; using local (%s)\n", arg, local)
		}
		return local, nil
	}
	if fileExists(globalPath) {
		return globalPath, nil
	}
	return "", fmt.Errorf("workflow %q not found in local (%s) or global (%s) scope", arg, localDir(), gdir)
}

// errAlreadyExists is returned by writeIfAbsent when the target path exists
// and force is false. Callers can check with errors.Is and choose whether to
// surface it as an error or a friendly message.
var errAlreadyExists = errors.New("already exists")

// writeIfAbsent writes data to path, creating parent dirs as needed.
// Returns errAlreadyExists (wrapped) if the path is present and force is false.
func writeIfAbsent(path string, data []byte, force bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if _, err := os.Stat(path); err == nil {
		if !force {
			return fmt.Errorf("%s: %w", path, errAlreadyExists)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

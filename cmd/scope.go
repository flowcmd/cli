package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

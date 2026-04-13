package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flowcmd/cli/internal/parser"
	"github.com/spf13/cobra"
)

const sourceMaxBytes = 1 << 20 // 1 MiB

var httpClient = &http.Client{Timeout: 30 * time.Second}

var (
	addGlobal bool
	addForce  bool
	addAs     string
)

var addCmd = &cobra.Command{
	Use:   "add <path-or-url>",
	Short: "Add a workflow from a local file or URL into local/global scope",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, baseName, err := fetchSource(args[0])
		if err != nil {
			return err
		}
		name := baseName
		if addAs != "" {
			name = addAs
		}
		name = ensureYAMLExt(name)
		if err := validateWorkflowName(name); err != nil {
			return err
		}

		dir, err := scopeDir(addGlobal)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		dest := filepath.Join(dir, name)

		if _, err := os.Stat(dest); err == nil && !addForce {
			return fmt.Errorf("%s already exists (use --force to overwrite)", dest)
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		// Stage to a temp file and validate before atomic rename, so a broken
		// download never replaces a working flow on disk.
		tmp, err := os.CreateTemp(dir, ".add-*.yml")
		if err != nil {
			return err
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath)
		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			return err
		}
		tmp.Close()
		if _, err := parser.Parse(tmpPath); err != nil {
			return fmt.Errorf("workflow is invalid: %w", err)
		}
		if err := os.Rename(tmpPath, dest); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		fmt.Printf("✓ Added %s\n", dest)
		return nil
	},
}

// fetchSource returns the bytes and a suggested base filename for either a URL
// or a local path. Both paths cap reads at sourceMaxBytes.
func fetchSource(src string) ([]byte, string, error) {
	if isURL(src) {
		return fetchURL(src)
	}
	f, err := os.Open(src)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", src, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, sourceMaxBytes))
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", src, err)
	}
	return data, filepath.Base(src), nil
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func fetchURL(raw string) ([]byte, string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, "", fmt.Errorf("parse url: %w", err)
	}
	resp, err := httpClient.Get(raw)
	if err != nil {
		return nil, "", fmt.Errorf("fetch %s: %w", raw, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("fetch %s: HTTP %d", raw, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, sourceMaxBytes))
	if err != nil {
		return nil, "", fmt.Errorf("read response: %w", err)
	}
	base := filepath.Base(u.Path)
	if base == "" || base == "/" || base == "." {
		base = "workflow.yml"
	}
	return data, base, nil
}

func init() {
	addCmd.Flags().BoolVarP(&addGlobal, "global", "g", false, "Add to global scope (~/.flowcmd)")
	addCmd.Flags().BoolVarP(&addForce, "force", "f", false, "Overwrite if already exists")
	addCmd.Flags().StringVar(&addAs, "as", "", "Save under a different filename")
	rootCmd.AddCommand(addCmd)
}

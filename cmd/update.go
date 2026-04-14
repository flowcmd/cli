package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	releasesAPI      = "https://api.github.com/repos/flowcmd/cli/releases/latest"
	installShellURL  = "https://raw.githubusercontent.com/flowcmd/cli/main/install.sh"
	installPSShURL   = "https://raw.githubusercontent.com/flowcmd/cli/main/install.ps1"
	updateHTTPPrefix = "update: "
)

var (
	updateCheck   bool
	updateVersion string

	// Overridable by tests.
	releasesAPIURL = releasesAPI
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Upgrade flowcmd to the latest release (or a specific version)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if updateCheck {
			return runUpdateCheck(cmd.OutOrStdout(), cmd.ErrOrStderr())
		}
		return runUpdate(cmd.OutOrStdout(), updateVersion)
	},
}

// runUpdateCheck fetches the latest release and compares against the running
// binary's version. It never returns an error for transient API problems;
// those print a warning to stderr and exit 0 so scripts calling this are not
// broken by upstream outages.
func runUpdateCheck(stdout, stderr io.Writer) error {
	current := currentVersion()
	if current == "dev" {
		fmt.Fprintln(stdout, "installed via 'go install'; update with:")
		fmt.Fprintln(stdout, "  go install github.com/flowcmd/cli@latest")
		return nil
	}

	latest, err := fetchLatestTag(releasesAPIURL)
	if err != nil {
		fmt.Fprintf(stderr, "warning: %sfailed to check for updates: %v\n", updateHTTPPrefix, err)
		return nil
	}

	cmp, err := compareSemver(current, latest)
	if err != nil {
		fmt.Fprintf(stderr, "warning: %s%v\n", updateHTTPPrefix, err)
		return nil
	}
	if cmp >= 0 {
		fmt.Fprintf(stdout, "up to date (%s)\n", current)
		return nil
	}
	fmt.Fprintf(stdout, "update available: %s → %s — run 'flowcmd update' to upgrade\n", current, latest)
	return nil
}

// runUpdate shells out to the OS-appropriate install script. On Windows the
// PowerShell script is used; on Linux/macOS the POSIX sh script is used.
func runUpdate(stdout io.Writer, pinnedVersion string) error {
	if currentVersion() == "dev" {
		return fmt.Errorf("this binary was built via 'go install'; update with: go install github.com/flowcmd/cli@latest")
	}

	script, err := chooseInstallScript()
	if err != nil {
		return err
	}

	env := os.Environ()
	if pinnedVersion != "" {
		env = append(env, "FLOWCMD_VERSION="+pinnedVersion)
	}

	cmd := exec.Command(script.program, script.args...) // #nosec G204 -- args are constant strings from our code
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

type installScript struct {
	program string
	args    []string
}

func chooseInstallScript() (installScript, error) {
	switch runtime.GOOS {
	case "windows":
		// Prefer pwsh when present, fall back to powershell.exe.
		prog := "powershell"
		if _, err := exec.LookPath("pwsh"); err == nil {
			prog = "pwsh"
		}
		return installScript{
			program: prog,
			args: []string{
				"-NoProfile",
				"-Command",
				fmt.Sprintf("irm %s | iex", installPSShURL),
			},
		}, nil
	case "linux", "darwin", "freebsd", "openbsd", "netbsd":
		return installScript{
			program: "sh",
			args: []string{
				"-c",
				fmt.Sprintf("curl -fsSL %s | sh", installShellURL),
			},
		}, nil
	default:
		return installScript{}, fmt.Errorf("unsupported OS for automatic update: %s", runtime.GOOS)
	}
}

// currentVersion extracts the semver portion of rootCmd.Version, which is
// formatted by SetVersionInfo as "<version> (commit <sha>, built <date>)".
// Returns "dev" when the binary was not stamped (go install path).
func currentVersion() string {
	v := rootCmd.Version
	if v == "" {
		return "dev"
	}
	// Take the first whitespace-separated token.
	if i := strings.IndexByte(v, ' '); i >= 0 {
		v = v[:i]
	}
	return strings.TrimPrefix(v, "v")
}

// fetchLatestTag returns the latest release tag (without the leading v).
func fetchLatestTag(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.TagName == "" {
		return "", fmt.Errorf("release response missing tag_name")
	}
	return strings.TrimPrefix(body.TagName, "v"), nil
}

// compareSemver returns -1 if a < b, 0 if equal, 1 if a > b. Pre-release
// suffixes are stripped (so "1.0.0-rc1" compares equal to "1.0.0"); this is
// acceptable for our single-release-channel use.
func compareSemver(a, b string) (int, error) {
	pa, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	pb, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1, nil
		}
		if pa[i] > pb[i] {
			return 1, nil
		}
	}
	return 0, nil
}

func parseSemver(v string) ([3]int, error) {
	var out [3]int
	v = strings.TrimPrefix(v, "v")
	// Strip pre-release / build metadata.
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return out, fmt.Errorf("invalid semver: %q", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("invalid semver part %q in %q", p, v)
		}
		out[i] = n
	}
	return out, nil
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Check for updates without installing")
	updateCmd.Flags().StringVar(&updateVersion, "version", "", "Install a specific version (e.g. v0.1.0)")
	rootCmd.AddCommand(updateCmd)
}

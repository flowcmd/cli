package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetListFlags(t *testing.T) {
	t.Helper()
	listGlobal, listLocal = false, false
	t.Cleanup(func() { listGlobal, listLocal = false, false })
}

func TestList_EmptyScopes(t *testing.T) {
	chdirAndFakeHome(t)
	resetListFlags(t)

	out := captureStdout(t, func() {
		if err := listCmd.RunE(listCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "no workflows found") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestList_ShowsBothScopes(t *testing.T) {
	cwd, home := chdirAndFakeHome(t)
	resetListFlags(t)

	_ = os.MkdirAll(filepath.Join(home, scopeDirName), 0o755)
	_ = os.MkdirAll(filepath.Join(cwd, scopeDirName), 0o755)
	writeFile(t, filepath.Join(home, scopeDirName, "gflow.yml"), sampleYAML)
	writeFile(t, filepath.Join(cwd, scopeDirName, "lflow.yml"), sampleYAML)

	out := captureStdout(t, func() {
		if err := listCmd.RunE(listCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "global") || !strings.Contains(out, "gflow") {
		t.Errorf("global section missing: %q", out)
	}
	if !strings.Contains(out, "local") || !strings.Contains(out, "lflow") {
		t.Errorf("local section missing: %q", out)
	}
	if idx := strings.Index(out, "global"); idx > strings.Index(out, "local") {
		t.Error("global should be printed before local")
	}
}

func TestList_GlobalOnlyFlag(t *testing.T) {
	cwd, home := chdirAndFakeHome(t)
	resetListFlags(t)
	listGlobal = true

	_ = os.MkdirAll(filepath.Join(home, scopeDirName), 0o755)
	_ = os.MkdirAll(filepath.Join(cwd, scopeDirName), 0o755)
	writeFile(t, filepath.Join(home, scopeDirName, "gflow.yml"), sampleYAML)
	writeFile(t, filepath.Join(cwd, scopeDirName, "lflow.yml"), sampleYAML)

	out := captureStdout(t, func() {
		if err := listCmd.RunE(listCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "gflow") || strings.Contains(out, "lflow") {
		t.Errorf("expected only global entries, got: %q", out)
	}
}

func TestList_LocalOnlyFlag(t *testing.T) {
	cwd, home := chdirAndFakeHome(t)
	resetListFlags(t)
	listLocal = true

	_ = os.MkdirAll(filepath.Join(home, scopeDirName), 0o755)
	_ = os.MkdirAll(filepath.Join(cwd, scopeDirName), 0o755)
	writeFile(t, filepath.Join(home, scopeDirName, "gflow.yml"), sampleYAML)
	writeFile(t, filepath.Join(cwd, scopeDirName, "lflow.yml"), sampleYAML)

	out := captureStdout(t, func() {
		if err := listCmd.RunE(listCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "lflow") || strings.Contains(out, "gflow") {
		t.Errorf("expected only local entries, got: %q", out)
	}
}

func TestList_InvalidFlowIsShown(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	resetListFlags(t)

	dir := filepath.Join(cwd, scopeDirName)
	_ = os.MkdirAll(dir, 0o755)
	writeFile(t, filepath.Join(dir, "bad.yml"), "name: x\nsteps: []\n")

	out := captureStdout(t, func() {
		if err := listCmd.RunE(listCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "invalid") {
		t.Errorf("expected invalid marker, got: %q", out)
	}
}

func TestScanFlows_UnreadableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}
	dir := t.TempDir()
	sub := filepath.Join(dir, "noaccess")
	if err := os.Mkdir(sub, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o700) })

	entries := scanFlows(sub)
	if len(entries) != 1 || entries[0].err == nil {
		t.Fatalf("expected single error entry, got %+v", entries)
	}
}

func TestScanFlows_SkipsDirsAndNonYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "notes.txt"), "hi")
	writeFile(t, filepath.Join(dir, "real.yml"), sampleYAML)

	got := scanFlows(dir)
	if len(got) != 1 || got[0].name != "real" {
		t.Fatalf("expected only real.yml, got %+v", got)
	}
}

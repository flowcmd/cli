package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetRemoveFlags(t *testing.T) {
	t.Helper()
	removeGlobal = false
	t.Cleanup(func() { removeGlobal = false })
}

func TestRemove_LocalSuccess(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	resetRemoveFlags(t)

	dir := filepath.Join(cwd, scopeDirName)
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "victim.yml")
	writeFile(t, path, sampleYAML)

	if err := removeCmd.RunE(removeCmd, []string{"victim"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be gone")
	}
}

func TestRemove_AcceptsNameWithExtension(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	resetRemoveFlags(t)

	dir := filepath.Join(cwd, scopeDirName)
	_ = os.MkdirAll(dir, 0o755)
	writeFile(t, filepath.Join(dir, "v.yml"), sampleYAML)

	if err := removeCmd.RunE(removeCmd, []string{"v.yml"}); err != nil {
		t.Fatal(err)
	}
}

func TestRemove_Global(t *testing.T) {
	_, home := chdirAndFakeHome(t)
	resetRemoveFlags(t)
	removeGlobal = true

	dir := filepath.Join(home, scopeDirName)
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "g.yml")
	writeFile(t, path, sampleYAML)

	if err := removeCmd.RunE(removeCmd, []string{"g"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("global file should be removed")
	}
}

func TestRemove_MissingReturnsError(t *testing.T) {
	chdirAndFakeHome(t)
	resetRemoveFlags(t)
	err := removeCmd.RunE(removeCmd, []string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRemove_OtherErrorPath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permissions")
	}
	cwd, _ := chdirAndFakeHome(t)
	resetRemoveFlags(t)

	dir := filepath.Join(cwd, scopeDirName)
	_ = os.MkdirAll(dir, 0o755)
	writeFile(t, filepath.Join(dir, "x.yml"), sampleYAML)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if err := removeCmd.RunE(removeCmd, []string{"x"}); err == nil {
		t.Fatal("expected remove failure")
	}
}

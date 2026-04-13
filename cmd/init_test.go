package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flowcmd/cli/internal/parser"
)

func runInit(t *testing.T, force bool) {
	t.Helper()
	initForce = force
	t.Cleanup(func() { initForce = false })
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("init: %v", err)
	}
}

func TestInit_FreshCreatesSample(t *testing.T) {
	dir, _ := chdirAndFakeHome(t)
	runInit(t, false)

	path := filepath.Join(dir, scopeDirName, sampleName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("sample not created: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("sample file is empty")
	}
	if _, err := parser.Parse(path); err != nil {
		t.Fatalf("embedded sample fails validation: %v", err)
	}
}

func TestInit_ExistingIsNoOp(t *testing.T) {
	chdirAndFakeHome(t)
	runInit(t, false)

	path := filepath.Join(scopeDirName, sampleName)
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	runInit(t, false)

	data, _ := os.ReadFile(path)
	if string(data) != "tampered" {
		t.Errorf("init should not overwrite without --force; got %q", string(data))
	}
}

func TestInit_ForceOverwrites(t *testing.T) {
	chdirAndFakeHome(t)
	runInit(t, false)

	path := filepath.Join(scopeDirName, sampleName)
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	runInit(t, true)

	data, _ := os.ReadFile(path)
	if string(data) == "tampered" {
		t.Error("--force should overwrite existing file")
	}
}

func TestInit_MkdirFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permissions")
	}
	cwd, _ := chdirAndFakeHome(t)
	if err := os.Chmod(cwd, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cwd, 0o755) })

	if err := initCmd.RunE(initCmd, nil); err == nil {
		t.Fatal("expected mkdir failure")
	}
}

func TestInit_CollisionPrintsHint(t *testing.T) {
	chdirAndFakeHome(t)
	runInit(t, false)

	out := captureStdout(t, func() {
		if err := initCmd.RunE(initCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "already exists") {
		t.Errorf("expected hint message, got %q", out)
	}
}

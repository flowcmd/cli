package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetAddFlags(t *testing.T) {
	t.Helper()
	addGlobal, addForce, addAs = false, false, ""
	t.Cleanup(func() { addGlobal, addForce, addAs = false, false, "" })
}

func TestAdd_LocalFromFile(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	resetAddFlags(t)

	src := filepath.Join(cwd, "mine.yml")
	writeFile(t, src, sampleYAML)

	if err := addCmd.RunE(addCmd, []string{src}); err != nil {
		t.Fatalf("add: %v", err)
	}
	dest := filepath.Join(cwd, scopeDirName, "mine.yml")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected %s: %v", dest, err)
	}
}

func TestAdd_GlobalFromFile(t *testing.T) {
	cwd, home := chdirAndFakeHome(t)
	resetAddFlags(t)
	addGlobal = true

	src := filepath.Join(cwd, "g.yml")
	writeFile(t, src, sampleYAML)

	if err := addCmd.RunE(addCmd, []string{src}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, scopeDirName, "g.yml")); err != nil {
		t.Fatalf("expected global file: %v", err)
	}
}

func TestAdd_AsRenames(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	resetAddFlags(t)
	addAs = "renamed"

	src := filepath.Join(cwd, "mine.yml")
	writeFile(t, src, sampleYAML)

	if err := addCmd.RunE(addCmd, []string{src}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cwd, scopeDirName, "renamed.yml")); err != nil {
		t.Fatalf("rename failed: %v", err)
	}
}

func TestAdd_CollisionRequiresForce(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	resetAddFlags(t)

	src := filepath.Join(cwd, "x.yml")
	writeFile(t, src, sampleYAML)

	if err := addCmd.RunE(addCmd, []string{src}); err != nil {
		t.Fatal(err)
	}
	err := addCmd.RunE(addCmd, []string{src})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected collision error, got %v", err)
	}

	addForce = true
	if err := addCmd.RunE(addCmd, []string{src}); err != nil {
		t.Fatalf("force should succeed: %v", err)
	}
}

func TestAdd_RejectsInvalidWorkflow(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	resetAddFlags(t)

	src := filepath.Join(cwd, "bad.yml")
	writeFile(t, src, invalidYAML)

	err := addCmd.RunE(addCmd, []string{src})
	if err == nil {
		t.Fatal("expected validation error")
	}
	// Confirm nothing was written.
	if _, err := os.Stat(filepath.Join(cwd, scopeDirName, "bad.yml")); !os.IsNotExist(err) {
		t.Error("invalid workflow should not be persisted")
	}
}

func TestAdd_FromURL(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	resetAddFlags(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleYAML))
	}))
	defer srv.Close()

	url := srv.URL + "/webflow.yml"
	if err := addCmd.RunE(addCmd, []string{url}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cwd, scopeDirName, "webflow.yml")); err != nil {
		t.Fatalf("expected file from url: %v", err)
	}
}

func TestAdd_RejectsPathTraversalInAs(t *testing.T) {
	cwd, _ := chdirAndFakeHome(t)
	resetAddFlags(t)
	addAs = "../escape"

	src := filepath.Join(cwd, "mine.yml")
	writeFile(t, src, sampleYAML)

	err := addCmd.RunE(addCmd, []string{src})
	if err == nil || !strings.Contains(err.Error(), "path separators") {
		t.Fatalf("expected path separator rejection, got %v", err)
	}
}

func TestFetchSource_LocalMissing(t *testing.T) {
	_, _, err := fetchSource(filepath.Join(t.TempDir(), "no-such-file.yml"))
	if err == nil {
		t.Fatal("expected read error")
	}
}

func TestFetchURL_BadURL(t *testing.T) {
	_, _, err := fetchURL("http://example.com/\x7f")
	if err == nil {
		t.Fatal("expected url parse error")
	}
}

func TestFetchURL_BadScheme(t *testing.T) {
	_, _, err := fetchURL("http://127.0.0.1:1/never")
	if err == nil || !strings.Contains(err.Error(), "fetch") {
		t.Fatalf("expected fetch failure, got %v", err)
	}
}

func TestFetchURL_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()
	_, _, err := fetchURL(srv.URL + "/x.yml")
	if err == nil || !strings.Contains(err.Error(), "418") {
		t.Fatalf("expected HTTP 418 error, got %v", err)
	}
}

func TestFetchURL_RootPathFallsBackToDefaultName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sampleYAML))
	}))
	defer srv.Close()
	_, name, err := fetchURL(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if name != "workflow.yml" {
		t.Errorf("expected fallback name workflow.yml, got %q", name)
	}
}

func TestValidateWorkflowName_Empty(t *testing.T) {
	if err := validateWorkflowName(""); err == nil {
		t.Fatal("expected empty-name error")
	}
}

func TestValidateWorkflowName_DotPrefix(t *testing.T) {
	err := validateWorkflowName(".secret.yml")
	if err == nil || !strings.Contains(err.Error(), "must not start with '.'") {
		t.Fatalf("expected dotfile rejection, got %v", err)
	}
}

func TestAdd_MkdirFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permissions")
	}
	cwd, _ := chdirAndFakeHome(t)
	resetAddFlags(t)

	src := filepath.Join(cwd, "x.yml")
	writeFile(t, src, sampleYAML)
	if err := os.Chmod(cwd, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cwd, 0o755) })

	if err := addCmd.RunE(addCmd, []string{src}); err == nil {
		t.Fatal("expected mkdir failure")
	}
}

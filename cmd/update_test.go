package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.1.0", 0},
		{"0.1.0", "0.1.1", -1},
		{"0.1.1", "0.1.0", 1},
		{"0.2.0", "0.1.9", 1},
		{"1.0.0", "0.9.9", 1},
		{"v0.1.0", "0.1.0", 0},
		{"0.1.0-rc1", "0.1.0", 0}, // pre-release stripped
		{"0.1.0+build", "0.1.0", 0},
	}
	for _, tc := range cases {
		got, err := compareSemver(tc.a, tc.b)
		if err != nil {
			t.Fatalf("compareSemver(%q, %q): %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("compareSemver(%q, %q) = %d; want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCompareSemver_InvalidInput(t *testing.T) {
	if _, err := compareSemver("not-a-version", "1.0.0"); err == nil {
		t.Error("expected error for invalid semver a")
	}
	if _, err := compareSemver("1.0.0", "not-a-version"); err == nil {
		t.Error("expected error for invalid semver b")
	}
}

func TestCurrentVersion(t *testing.T) {
	orig := rootCmd.Version
	t.Cleanup(func() { rootCmd.Version = orig })

	rootCmd.Version = ""
	if got := currentVersion(); got != "dev" {
		t.Errorf("empty version should be dev; got %q", got)
	}

	rootCmd.Version = "v0.1.0 (commit abc123, built 2026-04-14)"
	if got := currentVersion(); got != "0.1.0" {
		t.Errorf("got %q; want 0.1.0", got)
	}

	rootCmd.Version = "0.2.0"
	if got := currentVersion(); got != "0.2.0" {
		t.Errorf("got %q; want 0.2.0", got)
	}
}

func TestFetchLatestTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name": "v1.2.3"}`))
	}))
	defer srv.Close()

	got, err := fetchLatestTag(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2.3" {
		t.Errorf("got %q; want 1.2.3 (v prefix stripped)", got)
	}
}

func TestFetchLatestTag_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchLatestTag(srv.URL)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestFetchLatestTag_MissingTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := fetchLatestTag(srv.URL)
	if err == nil || !strings.Contains(err.Error(), "tag_name") {
		t.Errorf("expected tag_name error; got %v", err)
	}
}

func TestRunUpdateCheck_UpToDate(t *testing.T) {
	origVersion := rootCmd.Version
	origURL := releasesAPIURL
	t.Cleanup(func() {
		rootCmd.Version = origVersion
		releasesAPIURL = origURL
	})

	rootCmd.Version = "v1.0.0 (commit abc, built today)"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name": "v1.0.0"}`))
	}))
	defer srv.Close()
	releasesAPIURL = srv.URL

	var stdout, stderr bytes.Buffer
	if err := runUpdateCheck(&stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Errorf("expected 'up to date', got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty; got %q", stderr.String())
	}
}

func TestRunUpdateCheck_UpdateAvailable(t *testing.T) {
	origVersion := rootCmd.Version
	origURL := releasesAPIURL
	t.Cleanup(func() {
		rootCmd.Version = origVersion
		releasesAPIURL = origURL
	})

	rootCmd.Version = "v0.1.0 (commit abc, built today)"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name": "v0.2.0"}`))
	}))
	defer srv.Close()
	releasesAPIURL = srv.URL

	var stdout, stderr bytes.Buffer
	if err := runUpdateCheck(&stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "update available") {
		t.Errorf("expected 'update available', got %q", out)
	}
	if !strings.Contains(out, "0.1.0") || !strings.Contains(out, "0.2.0") {
		t.Errorf("expected both versions in output; got %q", out)
	}
}

func TestRunUpdateCheck_DevBuild(t *testing.T) {
	origVersion := rootCmd.Version
	t.Cleanup(func() { rootCmd.Version = origVersion })

	rootCmd.Version = "" // dev build
	var stdout, stderr bytes.Buffer
	if err := runUpdateCheck(&stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "go install") {
		t.Errorf("expected go install hint; got %q", stdout.String())
	}
}

func TestRunUpdateCheck_APIErrorIsNonFatal(t *testing.T) {
	origVersion := rootCmd.Version
	origURL := releasesAPIURL
	t.Cleanup(func() {
		rootCmd.Version = origVersion
		releasesAPIURL = origURL
	})

	rootCmd.Version = "v0.1.0 (commit abc, built today)"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	releasesAPIURL = srv.URL

	var stdout, stderr bytes.Buffer
	if err := runUpdateCheck(&stdout, &stderr); err != nil {
		t.Fatalf("API error should not fail the command; got %v", err)
	}
	if !strings.Contains(stderr.String(), "warning") {
		t.Errorf("expected warning on stderr; got %q", stderr.String())
	}
}

func TestChooseInstallScript_SupportedOS(t *testing.T) {
	// Can't easily swap runtime.GOOS; just verify the current OS resolves.
	s, err := chooseInstallScript()
	if err != nil {
		t.Fatalf("current OS should be supported; got %v", err)
	}
	if s.program == "" || len(s.args) == 0 {
		t.Errorf("incomplete install script: %+v", s)
	}
}

func TestRunUpdate_DevBuildIsRejected(t *testing.T) {
	origVersion := rootCmd.Version
	t.Cleanup(func() { rootCmd.Version = origVersion })

	rootCmd.Version = "" // dev
	var out bytes.Buffer
	err := runUpdate(&out, "")
	if err == nil || !strings.Contains(err.Error(), "go install") {
		t.Errorf("expected dev-build rejection; got %v", err)
	}
}

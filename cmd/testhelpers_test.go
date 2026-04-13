package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"
)

const sampleYAML = `name: Sample
steps:
  - name: one
    run: echo hi
`

const invalidYAML = `name: Bad
steps:
  - name: BAD NAME
    run: echo nope
`

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// captureStdout replaces os.Stdout for the duration of fn.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

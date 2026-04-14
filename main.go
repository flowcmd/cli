// Package main is the flowcmd CLI entrypoint.
package main

import "github.com/flowcmd/cli/cmd"

// Populated by goreleaser at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}

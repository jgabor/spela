// Command goref writes the Go reference fixtures for the Rust rewrite.
//
// It runs the real Go code of this module (internal/config, internal/profile,
// internal/game, internal/denylist, internal/dll, internal/steam and
// internal/release) against temporary XDG trees, then copies what that code
// wrote into tests/fixtures/go. It also runs the read-only commands of a built
// Go binary against fixture sandboxes and records their output.
//
// Usage, from the repository root:
//
//	GOPROXY=off GOFLAGS=-mod=mod go run ./tools/goref fixtures
//	GOPROXY=off GOFLAGS=-mod=mod go run ./tools/goref transcripts -bin /path/to/spela-go
//	GOPROXY=off GOFLAGS=-mod=mod go run ./tools/goref pe-version FILE...
//
// Before it calls any package, goref points HOME and every XDG variable at a
// temporary folder, so a run can never read or write the real spela files.
// It never runs pkexec or sudo, never changes hardware and contacts no remote
// host. The only network use is a loopback test server for the release check.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := isolateProcess(); err != nil {
		fmt.Fprintln(os.Stderr, "goref:", err)
		os.Exit(1)
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "fixtures":
		err = runFixtures(os.Args[2:])
	case "transcripts":
		err = runTranscripts(os.Args[2:])
	case "pe-version":
		err = runPEVersion(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if guardRoot != "" {
		_ = os.RemoveAll(guardRoot)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "goref:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: goref fixtures [-out DIR] | transcripts -bin SPELA [-fixtures DIR] | pe-version FILE...")
}

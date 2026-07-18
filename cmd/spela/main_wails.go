//go:build wails

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jgabor/spela/internal/gui"
)

func main() {
	if code := runWails(gui.Run, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func runWails(run func() error, stderr io.Writer) int {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

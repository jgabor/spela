//go:build wails
// +build wails

package main

import (
	"fmt"
	"os"

	"github.com/jgabor/spela/internal/gui"
)

func main() {
	if err := gui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

//go:build production

package gui

import (
	"strings"
	"testing"
)

func assertProductionIndex(t *testing.T, index []byte) {
	t.Helper()
	contents := string(index)
	for _, marker := range []string{"<!doctype html>", "<title>Spela</title>", `<div id="app">`} {
		if !strings.Contains(contents, marker) {
			t.Errorf("production index missing %q", marker)
		}
	}
}

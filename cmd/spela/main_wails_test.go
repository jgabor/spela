//go:build wails

package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestRunWailsExitAndDiagnosticContract(t *testing.T) {
	var stderr bytes.Buffer
	if code := runWails(func() error { return nil }, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("successful run = code %d, stderr %q", code, stderr.String())
	}
	want := errors.New("startup failed")
	if code := runWails(func() error { return want }, &stderr); code != 1 || stderr.String() != "startup failed\n" {
		t.Fatalf("failed run = code %d, stderr %q", code, stderr.String())
	}
}

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProfilePathsCollectRepeatedFlags(t *testing.T) {
	var paths profilePaths
	if err := paths.Set("first.out"); err != nil {
		t.Fatal(err)
	}
	if err := paths.Set("second.out"); err != nil {
		t.Fatal(err)
	}
	if got := paths.String(); got != "first.out,second.out" {
		t.Fatalf("profile paths = %q", got)
	}
}

func TestRunParsesProfilesAndRejectsInvalidFlags(t *testing.T) {
	root, profile := inventoryFixture(t)
	if err := run([]string{"--root", root, "--profile", profile}); err != nil {
		t.Fatalf("run valid inventory: %v", err)
	}
	if err := run([]string{"--unknown"}); err == nil {
		t.Fatal("run accepted unknown flag")
	}
}

func TestVerifyInventoryRejectsUnmeasuredTaggedExecutableSource(t *testing.T) {
	root, profile := inventoryFixture(t)
	writeFixture(t, filepath.Join(root, "internal", "feature", "tagged.go"), "//go:build special\n\npackage feature\nfunc tagged() { println(\"run\") }\n")

	err := verifyInventory(root, []string{profile})
	if err == nil || !strings.Contains(err.Error(), "internal/feature/tagged.go") {
		t.Fatalf("verifyInventory error = %v, want missing tagged source", err)
	}
}

func TestVerifyInventoryAllowsUnmeasuredDeclarationOnlySource(t *testing.T) {
	root, profile := inventoryFixture(t)
	writeFixture(t, filepath.Join(root, "internal", "feature", "declarations.go"), "package feature\ntype State struct { Ready bool }\nfunc external()\n")

	if err := verifyInventory(root, []string{profile}); err != nil {
		t.Fatalf("verifyInventory rejected declaration-only source: %v", err)
	}
}

func TestInventoryRecognizesFunctionLiteralsAsExecutable(t *testing.T) {
	root, profile := inventoryFixture(t)
	writeFixture(t, filepath.Join(root, "tools", "callback.go"), "package tools\nvar callback = func() { println(\"run\") }\n")

	err := verifyInventory(root, []string{profile})
	if err == nil || !strings.Contains(err.Error(), "tools/callback.go") {
		t.Fatalf("verifyInventory error = %v, want missing function literal source", err)
	}
}

func TestInventoryReportsInvalidInputs(t *testing.T) {
	root, profile := inventoryFixture(t)
	tests := []struct {
		name     string
		prepare  func() (string, []string)
		contains string
	}{
		{"no profiles", func() (string, []string) { return root, nil }, "at least one --profile"},
		{"missing go.mod", func() (string, []string) { return t.TempDir(), []string{profile} }, "open go.mod"},
		{"missing module declaration", func() (string, []string) {
			fixtureRoot := t.TempDir()
			writeFixture(t, filepath.Join(fixtureRoot, "go.mod"), "go 1.25\n")
			return fixtureRoot, []string{profile}
		}, "module declaration"},
		{"missing production roots", func() (string, []string) {
			fixtureRoot := t.TempDir()
			writeFixture(t, filepath.Join(fixtureRoot, "go.mod"), "module example.test/project\n")
			return fixtureRoot, []string{profile}
		}, "inventory cmd"},
		{"missing profile", func() (string, []string) { return root, []string{filepath.Join(root, "missing.out")} }, "open coverprofile"},
		{"malformed profile", func() (string, []string) {
			invalid := filepath.Join(root, "invalid.out")
			writeFixture(t, invalid, "mode: atomic\nnot a record\n")
			return root, []string{invalid}
		}, "invalid coverprofile record"},
		{"malformed source", func() (string, []string) {
			writeFixture(t, filepath.Join(root, "internal", "broken.go"), "package internal\nfunc broken(\n")
			return root, []string{profile}
		}, "parse"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixtureRoot, profiles := test.prepare()
			if err := verifyInventory(fixtureRoot, profiles); err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("verifyInventory error = %v, want %q", err, test.contains)
			}
		})
	}
}

func TestReadProfileSourcesIgnoresAbsoluteExternalSources(t *testing.T) {
	root, profile := inventoryFixture(t)
	writeFixture(t, profile, "mode: atomic\n/tmp/external.go:1.1,1.2 1 1\n"+filepath.Join(root, "cmd", "app", "main.go")+":2.13,2.31 1 1\n")
	sources := make(map[string]bool)
	if err := readProfileSources(profile, root, "example.test/project", sources); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sources, map[string]bool{"cmd/app/main.go": true}) {
		t.Fatalf("profile sources = %v", sources)
	}
}

func TestInventoryReadersReportOversizedRecords(t *testing.T) {
	oversized := strings.Repeat("x", 70_000)
	moduleFile := filepath.Join(t.TempDir(), "go.mod")
	writeFixture(t, moduleFile, oversized)
	if _, err := readModulePath(moduleFile); err == nil || !strings.Contains(err.Error(), "read go.mod") {
		t.Fatalf("readModulePath oversized error = %v", err)
	}
	profile := filepath.Join(t.TempDir(), "coverage.out")
	writeFixture(t, profile, oversized)
	if err := readProfileSources(profile, t.TempDir(), "example.test/project", make(map[string]bool)); err == nil || !strings.Contains(err.Error(), "read coverprofile") {
		t.Fatalf("readProfileSources oversized error = %v", err)
	}
}

func inventoryFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for _, directory := range productionRoots {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture(t, filepath.Join(root, "go.mod"), "module example.test/project\n")
	writeFixture(t, filepath.Join(root, "cmd", "app", "main.go"), "package main\nfunc main() { println(\"run\") }\n")
	profile := filepath.Join(root, "coverage.out")
	writeFixture(t, profile, "mode: atomic\nexample.test/project/cmd/app/main.go:2.13,2.31 1 1\n")
	return root, profile
}

func writeFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/nav"
)

func TestGeneratedNavigationContractMatchesTrackedFrontendAdapter(t *testing.T) {
	generated := generate()
	tracked, err := os.ReadFile(filepath.Join("testdata", "navContract.generated.js"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(generated) != strings.TrimSpace(string(tracked)) {
		t.Fatal("tracked navigation adapter is stale; run mage genNavJS")
	}
	for _, fragment := range []string{"Destination", "Aspect", "destinationFromHotkey", "initialNavState"} {
		if !strings.Contains(generated, fragment) {
			t.Errorf("generated adapter missing %q", fragment)
		}
	}
}

func TestNavigationJavaScriptFormattingHelpers(t *testing.T) {
	for destination, want := range map[nav.Destination]string{
		nav.DestinationLibrary: "Library", nav.DestinationDLLCatalog: "DLLCatalog",
		nav.DestinationMonitor: "Monitor", nav.DestinationSettings: "Settings",
		nav.Destination(99): "Library",
	} {
		if got := destinationName(destination); got != want {
			t.Errorf("destinationName(%d) = %q, want %q", destination, got, want)
		}
	}
	if got := jsStringArray([]string{"one", "two"}); got != `"one", "two"` {
		t.Fatalf("jsStringArray = %q", got)
	}
	if got := jsObject(nav.DefaultGUIState()); !strings.Contains(got, "scopeGlobal: true") || !strings.Contains(got, "breadcrumb:") {
		t.Fatalf("jsObject = %q", got)
	}
}

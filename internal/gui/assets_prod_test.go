//go:build embed_assets

package gui

import (
	"io/fs"
	"testing"
)

func TestEmbeddedProductionAssetsServeBuiltFrontendIndex(t *testing.T) {
	index, err := fs.ReadFile(getAssets(), "frontend/dist/index.html")
	if err != nil {
		t.Fatalf("read embedded production index: %v", err)
	}
	assertProductionIndex(t, index)
	if getDevHandler() != nil {
		t.Fatal("embedded production assets unexpectedly use the development handler")
	}
}

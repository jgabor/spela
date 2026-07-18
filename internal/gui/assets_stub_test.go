//go:build production && !embed_assets

package gui

import (
	"io/fs"
	"path/filepath"
	"testing"
)

func TestProductionStubAssetsServeBuiltFrontendIndex(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	index, err := fs.ReadFile(getAssets(), "index.html")
	if err != nil {
		t.Fatalf("read production stub index: %v", err)
	}
	assertProductionIndex(t, index)
	if getDevHandler() != nil {
		t.Fatal("production stub assets unexpectedly use the development handler")
	}
}

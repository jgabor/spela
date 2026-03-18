//go:build production && !embed_assets

package gui

import (
	"io/fs"
	"net/http"
	"os"
)

func getAssets() fs.FS {
	return os.DirFS("internal/gui/frontend/dist")
}

func getDevHandler() http.Handler {
	return nil
}

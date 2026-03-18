//go:build !embed_assets && !dev && (production || bindings)

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

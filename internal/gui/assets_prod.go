//go:build !dev

package gui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:frontend/dist
var embeddedAssets embed.FS

func getAssets() fs.FS {
	return embeddedAssets
}

func getDevHandler() http.Handler {
	return nil
}

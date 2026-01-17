//go:build dev

package gui

import (
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func getAssets() fs.FS {
	return nil
}

func getDevHandler() http.Handler {
	target, _ := url.Parse("http://localhost:5173")
	return httputil.NewSingleHostReverseProxy(target)
}

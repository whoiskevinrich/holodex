//go:build production

package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web/dist
var embeddedFrontend embed.FS

func frontendFS() http.FileSystem {
	dist, err := fs.Sub(embeddedFrontend, "web/dist")
	if err != nil {
		panic("web/dist embed: " + err.Error())
	}
	return http.FS(dist)
}

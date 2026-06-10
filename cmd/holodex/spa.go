package main

import (
	"net/http"
	"os"
)

// spaHandler serves a SvelteKit SPA. Files that exist in the embedded FS are
// served directly; any other path falls back to index.html so the client-side
// router can handle it.
type spaHandler struct{ fs http.FileSystem }

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f, err := h.fs.Open(r.URL.Path)
	if os.IsNotExist(err) {
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		http.FileServer(h.fs).ServeHTTP(w, r2)
		return
	}
	if err == nil {
		f.Close()
	}
	http.FileServer(h.fs).ServeHTTP(w, r)
}

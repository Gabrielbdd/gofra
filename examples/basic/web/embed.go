package basicweb

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html src
var files embed.FS

func Handler() http.Handler {
	staticFS, err := fs.Sub(files, ".")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		})
	}

	fileServer := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeFileFS(w, r, files, "index.html")
		default:
			fileServer.ServeHTTP(w, r)
		}
	})
}

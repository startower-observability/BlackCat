//go:build !nospa

package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

//go:embed all:dist
var spaFS embed.FS

// spaAvailable returns true if the embedded dist contains a built React app
// (not just the placeholder comment).
func spaAvailable() bool {
	data, err := spaFS.ReadFile("dist/index.html")
	if err != nil {
		return false
	}
	return len(data) > 4 && string(data[:4]) != "<!--"
}

// SPAHandler returns an http.Handler that serves the embedded React SPA.
func SPAHandler() http.Handler {
	sub, err := fs.Sub(spaFS, "dist")
	if err != nil {
		panic("spa: failed to sub embed.FS: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip /dashboard prefix — the embed FS is rooted at dist/
		path := strings.TrimPrefix(r.URL.Path, "/dashboard")
		if path == "" || path == "/" {
			path = "/index.html"
		}

		if strings.HasPrefix(path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		// Check if path exists in the FS
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath != "index.html" {
			if _, err := fs.Stat(sub, cleanPath); err == nil {
				// Exists — rewrite URL and serve
				r2 := r.Clone(r.Context())
				r2.URL.Path = path
				fileServer.ServeHTTP(w, r2)
				return
			}
		}

		// Not found — SPA fallback: serve index.html
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		content, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.ServeContent(w, r, "index.html", time.Time{}, strings.NewReader(string(content)))
	})
}

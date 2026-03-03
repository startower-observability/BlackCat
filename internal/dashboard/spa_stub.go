//go:build nospa

package dashboard

import "net/http"

// spaFS is nil when built with -tags nospa.
var spaFS interface{}

func spaAvailable() bool { return false }

// SPAHandler returns a handler that reports the SPA is not available.
func SPAHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "SPA not available (built with nospa tag)", http.StatusNotFound)
	})
}

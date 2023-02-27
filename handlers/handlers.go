package handlers

import (
	"net/http"
)

// Audio stream handler.

// Home route handler.
func Index(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

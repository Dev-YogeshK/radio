package main

import (
	"net/http"
	"stream/handlers"
	"stream/hub"

	"github.com/gorilla/mux"
)

func main() {
	// create a new mux router
	r := mux.NewRouter()

	h := hub.NewHub()
	go h.Start()

	// routes
	r.HandleFunc("/", handlers.Index).Methods(http.MethodGet)
	r.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Cache-Control", "no-cache")

		c := &hub.Client{
			Bytes: make(chan []byte),
		}
		h.Add(c)

		ctx := r.Context()

		go func() {
			<-ctx.Done()
			h.Remove(c)
		}()

		for b := range c.Bytes {
			_, err := w.Write(b)
			if err != nil {
				h.Remove(c)
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}).Methods(http.MethodGet)

	// listen on port
	http.ListenAndServe(":8004", r)
}

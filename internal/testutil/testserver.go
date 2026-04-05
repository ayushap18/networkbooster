package testutil

import (
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
)

func NewTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		io.CopyN(w, rand.Reader, 10*1024*1024)
	})

	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return httptest.NewServer(mux)
}

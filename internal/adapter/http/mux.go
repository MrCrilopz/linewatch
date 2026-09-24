package http

import (
	"encoding/json"
	stdhttp "net/http"
)

func NewMux() *stdhttp.ServeMux {
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("GET /health", health)
	return mux
}

func health(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

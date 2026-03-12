package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed ui/*
var uiFiles embed.FS

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/healthz" {
		http.NotFound(w, r)
		return
	}

	sub, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch r.URL.Path {
	case "/", "":
		http.ServeFileFS(w, r, sub, "index.html")
	case "/favicon.ico":
		w.WriteHeader(http.StatusNoContent)
	case "/app.js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		http.ServeFileFS(w, r, sub, "app.js")
	case "/styles.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		http.ServeFileFS(w, r, sub, "styles.css")
	default:
		http.NotFound(w, r)
	}
}

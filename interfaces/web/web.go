// Package web provides a Web UI plugin for xs-memory.
// Built only when the "webui" build tag is used. See design §13.3.
//
//go:build webui

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/xs-memory/xs-memory/xsmem"
)

// Server is the Web UI HTTP server. See design §13.3.
type Server struct {
	store  *xsmem.Store
	logger *slog.Logger
}

// NewServer creates a new Web UI server.
func NewServer(store *xsmem.Store) *Server {
	return &Server{store: store, logger: slog.Default()}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/memories", s.handleList)
	mux.HandleFunc("/api/stats", s.handleStats)

	s.logger.Info("web UI listening", "addr", addr)
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	return srv.ListenAndServe()
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	results, err := s.store.Search(r.Context(), xsmem.SearchOpts{
		Text: query,
		Mode: xsmem.Hybrid,
		TopK: 10,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	col := r.URL.Query().Get("collection")
	if col == "" {
		col = "default"
	}
	mems, err := s.store.List(r.Context(), col)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mems)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.store.Stats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func init() {
	fmt.Println("Web UI plugin loaded")
}

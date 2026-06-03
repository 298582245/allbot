package web

import (
	"net/http"

	_ "github.com/allbot/allbot/core/adapter/_loader"
	"github.com/allbot/allbot/core/adapter/_registry"
)

func (s *Server) handleAdapterPlatforms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.jsonResponse(w, registry.List())
}

package web

import (
	"net/http"

	_ "github.com/allbot/allbot/core/adapter/_loader"
	"github.com/allbot/allbot/core/adapter/_registry"
)

func (s *Server) handleAdapterPlatforms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	platforms := registry.List()
	pins := map[string]bool{}
	runtimeProfiles := map[string]string{}
	if s.adapterManager != nil {
		pins = s.adapterManager.AdapterPlatformPins()
		runtimeProfiles = s.adapterManager.AdapterPlatformRuntimeProfiles()
	}
	for i := range platforms {
		platforms[i].Pinned = pins[platforms[i].Platform]
		platforms[i].RuntimeProfile = runtimeProfiles[platforms[i].Platform]
	}
	s.jsonResponse(w, platforms)
}

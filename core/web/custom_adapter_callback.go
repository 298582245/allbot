package web

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"

	"github.com/allbot/allbot/core/adapter/_contract"
)

const customAdapterCallbackPrefix = "/api/open/adapters/custom/"

func (s *Server) handleCustomAdapterCallback(w http.ResponseWriter, r *http.Request) {
	id, token, relativePath, ok := parseCustomAdapterCallbackPath(r.URL.Path)
	if !ok || s.adapterManager == nil {
		http.NotFound(w, r)
		return
	}
	configuredToken, err := s.adapterManager.AdapterCallbackToken(id)
	if err != nil || len(token) != len(configuredToken) || subtle.ConstantTimeCompare([]byte(token), []byte(configuredToken)) != 1 {
		http.NotFound(w, r)
		return
	}
	adp := s.adapterManager.GetAdapterByID(id)
	if adp == nil {
		http.Error(w, "Adapter is not running", http.StatusServiceUnavailable)
		return
	}
	handler, ok := adp.(contract.HTTPCallbackHandler)
	if !ok {
		http.Error(w, "Adapter does not support callbacks", http.StatusNotFound)
		return
	}
	handler.HandleHTTPCallback(relativePath, w, r)
}

func parseCustomAdapterCallbackPath(path string) (int64, string, string, bool) {
	rest := strings.TrimPrefix(path, customAdapterCallbackPrefix)
	if rest == path {
		return 0, "", "", false
	}
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 {
		return 0, "", "", false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 || len(parts[1]) != 64 {
		return 0, "", "", false
	}
	relativePath := ""
	if len(parts) == 3 {
		relativePath = strings.Trim(parts[2], "/")
	}
	return id, parts[1], relativePath, true
}

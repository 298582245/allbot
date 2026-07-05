package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/allbot/allbot/core/adapter/_contract"
)

const feishuCallbackPrefix = "/api/open/adapters/feishu/"

func (s *Server) handleFeishuAdapterCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	adapterID, relativePath, ok := parseFeishuCallbackPath(r.URL.Path)
	if !ok {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if s.adapterManager == nil {
		http.NotFound(w, r)
		return
	}
	adp := s.adapterManager.GetAdapterByID(adapterID)
	if adp == nil {
		http.NotFound(w, r)
		return
	}
	if adp.GetPlatform() != "feishu" {
		http.NotFound(w, r)
		return
	}
	callbackHandler, ok := adp.(contract.HTTPCallbackHandler)
	if !ok {
		http.Error(w, "Adapter does not support HTTP callback", http.StatusInternalServerError)
		return
	}
	callbackHandler.HandleHTTPCallback(relativePath, w, r)
}

func parseFeishuCallbackPath(path string) (int64, string, bool) {
	rest := strings.TrimPrefix(path, feishuCallbackPrefix)
	if rest == path || rest == "" {
		return 0, "", false
	}
	parts := strings.SplitN(rest, "/", 2)
	adapterID, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || adapterID <= 0 {
		return 0, "", false
	}
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		return 0, "", false
	}
	return adapterID, strings.Trim(parts[1], "/"), true
}

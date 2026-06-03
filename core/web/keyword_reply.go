package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/allbot/allbot/core/config"
)

func (s *Server) handleKeywordReplies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.adapterManager.GetDatabase().ListKeywordReplies()
		if err != nil {
			s.jsonError(w, "获取关键字回复失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, items)
	case http.MethodPost:
		var item config.KeywordReply
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		if err := s.adapterManager.GetDatabase().SaveKeywordReply(&item); err != nil {
			s.jsonError(w, "保存关键字回复失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "保存成功"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleKeywordReplyDetail(w http.ResponseWriter, r *http.Request) {
	idText := strings.TrimPrefix(r.URL.Path, "/api/replies/keywords/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		s.jsonError(w, "关键字回复 ID 无效", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var item config.KeywordReply
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		item.ID = id
		if err := s.adapterManager.GetDatabase().SaveKeywordReply(&item); err != nil {
			s.jsonError(w, "保存关键字回复失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "保存成功"})
	case http.MethodDelete:
		if err := s.adapterManager.GetDatabase().DeleteKeywordReply(id); err != nil {
			s.jsonError(w, "删除关键字回复失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "删除成功"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

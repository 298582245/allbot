package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/allbot/allbot/core/config"
)

const (
	defaultUserPageSize = 20
	maxUserPageSize     = 200
	maxPointAdjustment  = int64(999999999)
)

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit, offset, ok := s.parseUserPagination(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	disabled, ok := parseOptionalBool(query.Get("disabled"))
	if !ok {
		s.jsonError(w, "disabled 必须是 true 或 false", http.StatusBadRequest)
		return
	}
	items, total, err := s.adapterManager.GetDatabase().ListUsers(config.UserQuery{
		Keyword: strings.TrimSpace(query.Get("keyword")), Platform: strings.TrimSpace(query.Get("platform")),
		Disabled: disabled, Limit: limit, Offset: offset,
	})
	if err != nil {
		s.jsonError(w, "获取用户列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"items": items, "total": total})
}

func (s *Server) handleUserDetail(w http.ResponseWriter, r *http.Request) {
	unionID, action, ok := parseUserAdminPath(r.URL.EscapedPath())
	if !ok {
		s.jsonError(w, "用户路径不存在", http.StatusNotFound)
		return
	}
	switch action {
	case "":
		s.handleUserGet(w, r, unionID)
	case "status":
		s.handleUserStatus(w, r, unionID)
	case "points/adjust":
		s.handleUserPointsAdjust(w, r, unionID)
	case "point-transactions":
		s.handleUserPointTransactions(w, r, unionID)
	default:
		s.jsonError(w, "用户路径不存在", http.StatusNotFound)
	}
}

func (s *Server) handleUserGet(w http.ResponseWriter, r *http.Request, unionID string) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	detail, err := s.adapterManager.GetDatabase().GetUserDetail(unionID)
	if writeUserError(s, w, err, "获取用户详情失败") {
		return
	}
	s.jsonResponse(w, detail)
}

func (s *Server) handleUserStatus(w http.ResponseWriter, r *http.Request, unionID string) {
	if r.Method != http.MethodPatch {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		Disabled *bool `json:"disabled"`
	}
	if !s.decodeUserJSON(w, r, &request) {
		return
	}
	if request.Disabled == nil {
		s.jsonError(w, "disabled 不能为空", http.StatusBadRequest)
		return
	}
	item, err := s.adapterManager.GetDatabase().SetUserDisabled(unionID, *request.Disabled)
	if writeUserError(s, w, err, "更新用户状态失败") {
		return
	}
	s.jsonResponse(w, item)
}

func (s *Server) handleUserPointsAdjust(w http.ResponseWriter, r *http.Request, unionID string) {
	if r.Method != http.MethodPost {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		Delta       int64  `json:"delta"`
		Description string `json:"description"`
	}
	if !s.decodeUserJSON(w, r, &request) {
		return
	}
	request.Description = strings.TrimSpace(request.Description)
	if request.Delta == 0 || request.Delta < -maxPointAdjustment || request.Delta > maxPointAdjustment {
		s.jsonError(w, "积分调整值必须是范围内的非零整数", http.StatusBadRequest)
		return
	}
	if len([]rune(request.Description)) > 500 {
		s.jsonError(w, "说明不能超过 500 个字符", http.StatusBadRequest)
		return
	}
	adjustment, err := s.adapterManager.GetDatabase().AdjustUserPoints(unionID, request.Delta, request.Description)
	if errors.Is(err, config.ErrUserNotFound) || errors.Is(err, sql.ErrNoRows) {
		s.jsonError(w, "用户不存在", http.StatusNotFound)
		return
	}
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "积分不足") {
			status = http.StatusConflict
		}
		s.jsonError(w, err.Error(), status)
		return
	}
	s.jsonResponse(w, adjustment)
}

func (s *Server) handleUserPointTransactions(w http.ResponseWriter, r *http.Request, unionID string) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit, offset, ok := s.parseUserPagination(w, r)
	if !ok {
		return
	}
	database := s.adapterManager.GetDatabase()
	exists, err := database.UserUnionExists(unionID)
	if err != nil {
		s.jsonError(w, "获取用户详情失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		s.jsonError(w, "用户不存在", http.StatusNotFound)
		return
	}
	items, total, err := database.ListPointTransactions(config.PointTransactionQuery{UnionID: unionID, Limit: limit, Offset: offset})
	if err != nil {
		s.jsonError(w, "获取积分流水失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"items": items, "total": total})
}

func (s *Server) handleUserAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit, offset, ok := s.parseUserPagination(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	platform := strings.TrimSpace(query.Get("platform"))
	if platform == "" {
		s.jsonError(w, "platform 不能为空", http.StatusBadRequest)
		return
	}
	disabled, ok := parseOptionalBool(query.Get("disabled"))
	if !ok {
		s.jsonError(w, "disabled 必须是 true 或 false", http.StatusBadRequest)
		return
	}
	items, total, err := s.adapterManager.GetDatabase().ListUserAccounts(config.UserQuery{
		Keyword: strings.TrimSpace(query.Get("keyword")), Platform: platform, Disabled: disabled, Limit: limit, Offset: offset,
	})
	if err != nil {
		s.jsonError(w, "获取平台账号列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"items": items, "total": total})
}

func parseUserAdminPath(escapedPath string) (string, string, bool) {
	const prefix = "/api/users/"
	if !strings.HasPrefix(escapedPath, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(escapedPath, prefix)
	if rest == "" || strings.HasSuffix(rest, "/") {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 1 || len(parts) > 3 {
		return "", "", false
	}
	unionID, err := url.PathUnescape(parts[0])
	if err != nil {
		return "", "", false
	}
	unionID = strings.TrimSpace(unionID)
	if unionID == "" || strings.ContainsAny(unionID, "/\\") {
		return "", "", false
	}
	action := strings.Join(parts[1:], "/")
	switch action {
	case "", "status", "points/adjust", "point-transactions":
		return unionID, action, true
	default:
		return "", "", false
	}
}

func (s *Server) parseUserPagination(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	limit, err := parseStrictNonNegativeInt(r.URL.Query().Get("limit"), defaultUserPageSize)
	if err != nil || limit < 1 || limit > maxUserPageSize {
		s.jsonError(w, fmt.Sprintf("limit 必须是 1 到 %d 的整数", maxUserPageSize), http.StatusBadRequest)
		return 0, 0, false
	}
	offset, err := parseStrictNonNegativeInt(r.URL.Query().Get("offset"), 0)
	if err != nil {
		s.jsonError(w, "offset 必须是非负整数", http.StatusBadRequest)
		return 0, 0, false
	}
	return limit, offset, true
}

func parseStrictNonNegativeInt(value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("参数无效")
	}
	return parsed, nil
}

func parseOptionalBool(value string) (*bool, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil || (value != "true" && value != "false") {
		return nil, false
	}
	return &parsed, true
}

func writeUserError(s *Server, w http.ResponseWriter, err error, prefix string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, config.ErrUserNotFound) || errors.Is(err, sql.ErrNoRows) {
		s.jsonError(w, "用户不存在", http.StatusNotFound)
		return true
	}
	s.jsonError(w, prefix+": "+err.Error(), http.StatusInternalServerError)
	return true
}

func (s *Server) decodeUserJSON(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return false
	}
	return true
}

package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
)

func (s *Server) handleDataTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tables, err := s.adapterManager.GetDatabase().ListTables()
	if err != nil {
		s.jsonError(w, "获取数据表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, tables)
}

func (s *Server) handleDataViews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var view config.DataViewConfig
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := s.adapterManager.GetDatabase().SaveDataView(view); err != nil {
		s.jsonError(w, "保存视图失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "视图已保存"})
}

func (s *Server) handleDataTableDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/data/tables/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		s.jsonError(w, "表名不能为空", http.StatusBadRequest)
		return
	}
	table := parts[0]

	if len(parts) == 1 {
		s.handleDataTableAction(w, r, table)
		return
	}
	if len(parts) == 2 && parts[1] == "rename" {
		s.handleDataTableRename(w, r, table)
		return
	}
	if len(parts) == 2 && parts[1] == "clear" {
		s.handleDataTableClear(w, r, table)
		return
	}
	if len(parts) == 2 && parts[1] == "rows" {
		s.handleDataRows(w, r, table)
		return
	}
	if len(parts) == 3 && parts[1] == "rows" {
		rowID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			s.jsonError(w, "行 ID 无效", http.StatusBadRequest)
			return
		}
		s.handleDataRow(w, r, table, rowID)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleDataTableAction(w http.ResponseWriter, r *http.Request, table string) {
	if r.Method != http.MethodDelete {
		s.handleDataRows(w, r, table)
		return
	}
	if err := s.adapterManager.GetDatabase().DropTable(table); err != nil {
		s.jsonError(w, "删除表失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "表已删除"})
}

func (s *Server) handleDataTableRename(w http.ResponseWriter, r *http.Request, table string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := s.adapterManager.GetDatabase().RenameTable(table, req.Name); err != nil {
		s.jsonError(w, "重命名表失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "表已重命名"})
}

func (s *Server) handleDataTableClear(w http.ResponseWriter, r *http.Request, table string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.adapterManager.GetDatabase().ClearTable(table); err != nil {
		s.jsonError(w, "清空表失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "表数据已清空"})
}

func (s *Server) handleDataRows(w http.ResponseWriter, r *http.Request, table string) {
	switch r.Method {
	case http.MethodGet:
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("size"))
		search := strings.TrimSpace(r.URL.Query().Get("search"))
		rows, err := s.adapterManager.GetDatabase().QueryTableRows(table, page, size, search)
		if err != nil {
			s.jsonError(w, "查询表数据失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, rows)
	case http.MethodPost:
		var req struct {
			Values map[string]interface{} `json:"values"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		if err := s.adapterManager.GetDatabase().InsertTableRow(table, req.Values); err != nil {
			s.jsonError(w, "新增数据失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "新增成功"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDataRow(w http.ResponseWriter, r *http.Request, table string, rowID int64) {
	switch r.Method {
	case http.MethodPut:
		var req struct {
			Values map[string]interface{} `json:"values"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		if err := s.adapterManager.GetDatabase().UpdateTableRow(table, rowID, req.Values); err != nil {
			s.jsonError(w, "更新数据失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "更新成功"})
	case http.MethodDelete:
		if err := s.adapterManager.GetDatabase().DeleteTableRow(table, rowID); err != nil {
			s.jsonError(w, "删除数据失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "删除成功"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDataExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := s.adapterManager.GetDatabase().ExportTables(r.URL.Query().Get("table"))
	if err != nil {
		s.jsonError(w, "导出数据失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	filename := fmt.Sprintf("allbot-data-%s.json", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) handleDataImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	replace := r.URL.Query().Get("replace") == "true"
	data, err := io.ReadAll(io.LimitReader(r.Body, 20*1024*1024))
	if err != nil {
		s.jsonError(w, "读取导入数据失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.adapterManager.GetDatabase().ImportTables(data, replace); err != nil {
		s.jsonError(w, "导入数据失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "导入成功"})
}

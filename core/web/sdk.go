package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) handleSDKFiles(w http.ResponseWriter, r *http.Request) {
	root := "sdk"
	switch r.Method {
	case http.MethodGet:
		filePath := strings.TrimSpace(r.URL.Query().Get("path"))
		if filePath == "" {
			tree, err := buildPluginFileTree(root, "")
			if err != nil {
				s.jsonError(w, "读取 SDK 目录失败: "+err.Error(), http.StatusInternalServerError)
				return
			}
			s.jsonResponse(w, map[string]interface{}{"tree": tree})
			return
		}
		fullPath, err := safeSDKPath(root, filePath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			s.jsonError(w, "SDK 文件不存在或不是文件", http.StatusNotFound)
			return
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			s.jsonError(w, "读取 SDK 文件失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"path": filepath.ToSlash(filePath), "code": string(data), "editable": true})
	case http.MethodPut:
		var req struct {
			Path string `json:"path"`
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "请求体无效: "+err.Error(), http.StatusBadRequest)
			return
		}
		fullPath, err := safeSDKPath(root, req.Path)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(fullPath, []byte(req.Code), 0644); err != nil {
			s.jsonError(w, "保存 SDK 文件失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "SDK 文件已保存"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSDKReference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"content": sdkReferenceText()})
}

func safeSDKPath(root, filePath string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(filePath))
	if clean == "." || clean == string(filepath.Separator) || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", errBadSDKPath()
	}
	fullPath := filepath.Join(root, clean)
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	fullAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	if fullAbs != rootAbs && !strings.HasPrefix(fullAbs, rootAbs+string(filepath.Separator)) {
		return "", errBadSDKPath()
	}
	return fullPath, nil
}

func errBadSDKPath() error { return &sdkPathError{} }

type sdkPathError struct{}

func (e *sdkPathError) Error() string { return "SDK 文件路径无效" }

func sdkReferenceText() string {
	return `# SDK 参考示例和函数说明

## Node.js 基础 SDK

- 文件：sdk/nodejs/allbot_direct.js
- 入口：runDirect(async (ctx) => {})
- 常用：ctx.reply、ctx.listen、ctx.config、ctx.setScheduledTask、ctx.runScript、ctx.runQLScript

## Node.js 账号青龙插件封装

- 文件：sdk/nodejs/account_ql_plugin.js
- 入口：createAccountQLPlugin(options)
- 插件作者需要提供：登录解析、账号唯一键、查询、CK 检测。
- 框架负责：账号菜单、内置积分授权、定时任务、脚本运行、运行后查询、CK 检测通知。

示例：plugins/xyyx/main.js、plugins/fxsh/main.js

## Python 基础 SDK

- 文件：sdk/python/allbot_direct.py
- 入口：run_direct(async def handle(ctx): ...)
- 常用：ctx.reply、ctx.listen、ctx.config、ctx.set_scheduled_task、ctx.run_script、ctx.run_ql_script

## Python 账号青龙插件封装

- 文件：sdk/python/account_ql_plugin.py
- 入口：create_account_ql_plugin(options)
- 示例：plugins/python_ql_demo/main.py

## runQLScript / run_ql_script

- runtime：nodejs 或 python
- script：插件目录内相对路径
- envName/env_name：脚本读取的环境变量名
- accounts：账号列表，框架会把 env_value 按换行拼接注入环境变量
- runMode/run_mode：all_authorized、current_user、single_account
- wait：是否等待脚本结束
`
}

package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/deps"
	"github.com/allbot/allbot/core/types"
)

type Manager struct {
	plugins        map[string]*PluginProcess
	runningScripts map[int64]context.CancelFunc
	scriptDone     map[int64]chan ScriptRunResult
	mu             sync.RWMutex
	pluginDir      string
	depsManager    *deps.Manager
	database       *config.Database
}

type PluginProcess struct {
	Plugin *types.Plugin
	Cmd    *exec.Cmd
	Port   int
	Status string
}

type PluginDBAction struct {
	Action    string
	RequestID string
	Table     string
	Columns   []config.PluginTableColumn
	Values    map[string]interface{}
	RowID     int64
	Query     config.PluginDBQuery
}

type PluginDBResult struct {
	Success bool
	Error   string
	Data    interface{}
}

type PluginUserResult struct {
	Success bool
	Error   string
	Data    interface{}
}

type FakeMessageAction struct {
	Platform  string
	AdapterID string
	UserID    string
	GroupID   string
	Content   string
}

type SendMessageAction struct {
	Platform  string
	AdapterID string
	UserID    string
	GroupID   string
	UnionID   string
	Text      string
}

type PluginConfigAction struct {
	AccessControl types.AccessControlConfig
}

type ScheduledTaskAction struct {
	TaskKey     string
	Name        string
	Description string
	Enabled     bool
	Pinned      bool
	Cron        string
	Platform    string
	AdapterID   string
	UserID      string
	GroupID     string
	Content     string
	MaxCount    int
}

type PluginAccountAction struct {
	Action      string
	RequestID   string
	TableName   string
	Scope       string
	ID          int64
	UnionID     string
	Platform    string
	UserID      string
	AccountName string
	EnvName     string
	EnvValue    string
	Remark      string
	Status      string
	Metadata    map[string]interface{}
	ExpiresAt   string
}

type ScriptRunAction struct {
	RequestID string
	PluginID  string
	Runtime   string
	Script    string
	Cwd       string
	Env       map[string]string
	Timeout   int
	Wait      bool
	RunMode   string
	UnionID   string
}

type ScriptRunResult struct {
	Status     string    `json:"status"`
	Output     string    `json:"output"`
	Error      string    `json:"error"`
	FinishedAt time.Time `json:"finished_at"`
}

type ScriptRunTask struct {
	ID         int64     `json:"id"`
	PluginID   string    `json:"plugin_id"`
	UnionID    string    `json:"union_id"`
	ScriptPath string    `json:"script_path"`
	Runtime    string    `json:"runtime"`
	RunMode    string    `json:"run_mode"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
}

type PluginAuthorizationAction struct {
	Action    string
	RequestID string
	TableName string
	UnionID   string
	Amount    int64
	Status    string
	Plan      string
	Source    string
	Metadata  map[string]interface{}
	ExpiresAt string
}

func NewManager(pluginDir string, depsManager *deps.Manager) *Manager {
	return &Manager{plugins: make(map[string]*PluginProcess), runningScripts: make(map[int64]context.CancelFunc), scriptDone: make(map[int64]chan ScriptRunResult), pluginDir: pluginDir, depsManager: depsManager}
}

func (m *Manager) SetDatabase(database *config.Database) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.database = database
}

func (m *Manager) GetDepsManager() *deps.Manager { return m.depsManager }

func (m *Manager) LoadPlugin(pluginPath string) (*types.Plugin, error) {
	plugin, err := m.loadPluginConfig(pluginPath)
	if err != nil {
		return nil, err
	}

	m.installDeps(plugin)

	m.mu.Lock()
	m.plugins[plugin.ID] = &PluginProcess{Plugin: plugin, Status: "ready"}
	m.mu.Unlock()
	return plugin, nil
}

func (m *Manager) LoadAllPlugins() ([]*types.Plugin, error) {
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return nil, err
	}

	plugins := make([]*types.Plugin, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginPath := filepath.Join(m.pluginDir, entry.Name())
		plugin, err := m.LoadPlugin(pluginPath)
		if err != nil {
			log.Printf("[SYSTEM] 加载插件失败 %s: %v", entry.Name(), err)
			continue
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

func (m *Manager) GetPlugin(pluginID string) *PluginProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.plugins[pluginID]
}

func (m *Manager) GetAllPlugins() []*PluginProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*PluginProcess, 0, len(m.plugins))
	for _, process := range m.plugins {
		result = append(result, process)
	}
	return result
}

func (m *Manager) TogglePlugin(pluginID string, enabled bool) error {
	m.mu.Lock()
	process, ok := m.plugins[pluginID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	configPath := filepath.Join(m.pluginDir, pluginID, "plugin.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	config["enabled"] = enabled
	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, newData, 0644); err != nil {
		return err
	}

	process.Plugin.Enabled = enabled
	return nil
}

func (m *Manager) ReloadPlugin(pluginID string) error {
	pluginPath := filepath.Join(m.pluginDir, pluginID)
	plugin, err := m.loadPluginConfig(pluginPath)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if process, ok := m.plugins[pluginID]; ok {
		process.Plugin = plugin
		process.Status = "ready"
	} else {
		m.plugins[pluginID] = &PluginProcess{Plugin: plugin, Status: "ready"}
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) SavePluginAccessControl(pluginID string, accessControl types.AccessControlConfig) error {
	configPath := filepath.Join(m.pluginDir, pluginID, "plugin.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	raw["access_control"] = accessControl
	updated, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, updated, 0644); err != nil {
		return err
	}
	return m.ReloadPlugin(pluginID)
}

func (m *Manager) StopPlugin(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	process, ok := m.plugins[pluginID]
	if !ok || process.Cmd == nil || process.Cmd.Process == nil {
		return nil
	}
	_ = process.Cmd.Process.Kill()
	process.Cmd = nil
	process.Status = "ready"
	return nil
}

func (m *Manager) StartPluginByID(pluginID string) error {
	return m.ReloadPlugin(pluginID)
}

func (m *Manager) ExecutePlugin(plugin *types.Plugin, pluginPath string, messageJSON []byte, replyFunc func(string) error, imageFunc func(string) error, fileFunc func(string) error, listenFunc func(timeout int) string, dataViewFunc func(config.DataViewConfig) error, dbFunc func(pluginID string, action PluginDBAction) PluginDBResult, fakeMessageFunc func(pluginID string, action FakeMessageAction) error, sendMessageFunc func(pluginID string, action SendMessageAction) PluginUserResult, userFunc func() PluginUserResult, configFunc func(pluginID string, action PluginConfigAction) PluginUserResult, scheduleFunc func(pluginID string, action ScheduledTaskAction) PluginUserResult, accountFunc func(pluginID string, action PluginAccountAction) PluginUserResult, authFunc func(pluginID string, action PluginAuthorizationAction) PluginUserResult, scriptFunc func(pluginID string, action ScriptRunAction) PluginUserResult) error {
	cmd, err := m.newDirectCommand(plugin, pluginPath)
	if err != nil {
		return err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		output, _ := io.ReadAll(stderr)
		if len(output) > 0 {
			log.Printf("[SYSTEM] Plugin %s stderr: %s", plugin.ID, string(output))
		}
	}()

	messageJSON = append(messageJSON, '\n')
	if _, err := stdin.Write(messageJSON); err != nil {
		_ = cmd.Process.Kill()
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var action struct {
			Action        string                     `json:"action"`
			RequestID     string                     `json:"request_id"`
			Text          string                     `json:"text"`
			URL           string                     `json:"url"`
			Path          string                     `json:"path"`
			Timeout       int                        `json:"timeout"`
			Success       bool                       `json:"success"`
			Error         string                     `json:"error"`
			TableName     string                     `json:"table_name"`
			ViewName      string                     `json:"view_name"`
			GroupName     string                     `json:"group_name"`
			Description   string                     `json:"description"`
			Columns       []string                   `json:"columns"`
			DBTable       string                     `json:"table"`
			DBColumns     []config.PluginTableColumn `json:"db_columns"`
			Values        map[string]interface{}     `json:"values"`
			RowID         int64                      `json:"row_id"`
			Query         config.PluginDBQuery       `json:"query"`
			AccessControl types.AccessControlConfig  `json:"access_control"`
			TaskKey       string                     `json:"task_key"`
			Name          string                     `json:"name"`
			Enabled       bool                       `json:"enabled"`
			Pinned        bool                       `json:"pinned"`
			MaxCount      int                        `json:"max_count"`
			Platform      string                     `json:"platform"`
			AdapterID     string                     `json:"adapter_id"`
			UserID        string                     `json:"user_id"`
			GroupID       string                     `json:"group_id"`
			Content       string                     `json:"content"`
			TextMessage   string                     `json:"text_message"`
			Cron          string                     `json:"cron"`
			Scope         string                     `json:"scope"`
			ID            int64                      `json:"id"`
			UnionID       string                     `json:"union_id"`
			AccountName   string                     `json:"account_name"`
			EnvName       string                     `json:"env_name"`
			EnvValue      string                     `json:"env_value"`
			Remark        string                     `json:"remark"`
			Status        string                     `json:"status"`
			Plan          string                     `json:"plan"`
			Source        string                     `json:"source"`
			Amount        int64                      `json:"amount"`
			Metadata      map[string]interface{}     `json:"metadata"`
			Runtime       string                     `json:"runtime"`
			Script        string                     `json:"script"`
			Cwd           string                     `json:"cwd"`
			Env           map[string]string          `json:"env"`
			Wait          bool                       `json:"wait"`
			RunMode       string                     `json:"run_mode"`
			ExpiresAt     string                     `json:"expires_at"`
		}
		if err := json.Unmarshal([]byte(line), &action); err != nil {
			log.Printf("[SYSTEM] Plugin %s invalid output line: %s", plugin.ID, line)
			continue
		}

		switch action.Action {
		case "reply":
			if replyFunc != nil && action.Text != "" {
				_ = replyFunc(action.Text)
			}
		case "send_image":
			if imageFunc != nil && action.URL != "" {
				_ = imageFunc(action.URL)
			}
		case "send_file":
			if fileFunc != nil && action.Path != "" {
				_ = fileFunc(action.Path)
			}
		case "listen":
			timeout := action.Timeout
			if timeout <= 0 {
				timeout = 60
			}
			content := ""
			if listenFunc != nil {
				content = listenFunc(timeout)
			}
			response, _ := json.Marshal(map[string]string{"action": "listen_response", "content": content})
			response = append(response, '\n')
			_, _ = stdin.Write(response)
		case "set_data_view":
			if dataViewFunc != nil && action.TableName != "" {
				view := config.DataViewConfig{PluginID: plugin.ID, TableName: action.TableName, ViewName: action.ViewName, GroupName: action.GroupName, Description: action.Description, Columns: action.Columns}
				if err := dataViewFunc(view); err != nil {
					log.Printf("[SYSTEM] Plugin %s set data view failed: %v", plugin.ID, err)
				}
			}
		case "db_create_table", "db_query", "db_insert", "db_update", "db_delete", "db_clear":
			result := PluginDBResult{Success: false, Error: "数据库执行器不可用"}
			if dbFunc != nil {
				result = dbFunc(plugin.ID, PluginDBAction{Action: action.Action, RequestID: action.RequestID, Table: action.DBTable, Columns: action.DBColumns, Values: action.Values, RowID: action.RowID, Query: action.Query})
			}
			response, _ := json.Marshal(map[string]interface{}{"action": "db_response", "request_id": action.RequestID, "success": result.Success, "error": result.Error, "data": result.Data})
			response = append(response, '\n')
			_, _ = stdin.Write(response)
		case "fake_message":
			responseData := map[string]interface{}{"action": "fake_message_response", "request_id": action.RequestID, "success": true, "error": ""}
			if fakeMessageFunc == nil {
				responseData["success"] = false
				responseData["error"] = "伪造消息执行器不可用"
			} else if err := fakeMessageFunc(plugin.ID, FakeMessageAction{Platform: action.Platform, AdapterID: action.AdapterID, UserID: action.UserID, GroupID: action.GroupID, Content: action.Content}); err != nil {
				responseData["success"] = false
				responseData["error"] = err.Error()
			}
			response, _ := json.Marshal(responseData)
			response = append(response, '\n')
			_, _ = stdin.Write(response)
		case "send_message":
			result := PluginUserResult{Success: false, Error: "消息发送器不可用"}
			if sendMessageFunc != nil {
				text := action.Text
				if text == "" {
					text = action.TextMessage
				}
				result = sendMessageFunc(plugin.ID, SendMessageAction{Platform: action.Platform, AdapterID: action.AdapterID, UserID: action.UserID, GroupID: action.GroupID, UnionID: action.UnionID, Text: text})
			}
			response, _ := json.Marshal(map[string]interface{}{"action": "send_message_response", "request_id": action.RequestID, "success": result.Success, "error": result.Error, "data": result.Data})
			response = append(response, '\n')
			_, _ = stdin.Write(response)
		case "get_union_id":
			result := PluginUserResult{Success: false, Error: "用户身份执行器不可用"}
			if userFunc != nil {
				result = userFunc()
			}
			response, _ := json.Marshal(map[string]interface{}{"action": "union_id_response", "request_id": action.RequestID, "success": result.Success, "error": result.Error, "data": result.Data})
			response = append(response, '\n')
			_, _ = stdin.Write(response)
		case "set_access_control":
			result := PluginUserResult{Success: false, Error: "插件配置执行器不可用"}
			if configFunc != nil {
				result = configFunc(plugin.ID, PluginConfigAction{AccessControl: action.AccessControl})
			}
			response, _ := json.Marshal(map[string]interface{}{"action": "access_control_response", "request_id": action.RequestID, "success": result.Success, "error": result.Error, "data": result.Data})
			response = append(response, '\n')
			_, _ = stdin.Write(response)
		case "set_scheduled_task":
			result := PluginUserResult{Success: false, Error: "定时任务执行器不可用"}
			if scheduleFunc != nil {
				result = scheduleFunc(plugin.ID, ScheduledTaskAction{TaskKey: action.TaskKey, Name: action.Name, Description: action.Description, Enabled: action.Enabled, Pinned: action.Pinned, Cron: action.Cron, Platform: action.Platform, AdapterID: action.AdapterID, UserID: action.UserID, GroupID: action.GroupID, Content: action.Content, MaxCount: action.MaxCount})
			}
			response, _ := json.Marshal(map[string]interface{}{"action": "scheduled_task_response", "request_id": action.RequestID, "success": result.Success, "error": result.Error, "data": result.Data})
			response = append(response, '\n')
			_, _ = stdin.Write(response)
		case "account_save", "account_list", "account_delete":
			result := PluginUserResult{Success: false, Error: "账号执行器不可用"}
			if accountFunc != nil {
				result = accountFunc(plugin.ID, PluginAccountAction{Action: action.Action, RequestID: action.RequestID, TableName: action.TableName, Scope: action.Scope, ID: action.ID, UnionID: action.UnionID, Platform: action.Platform, UserID: action.UserID, AccountName: action.AccountName, EnvName: action.EnvName, EnvValue: action.EnvValue, Remark: action.Remark, Status: action.Status, Metadata: action.Metadata, ExpiresAt: action.ExpiresAt})
			}
			response, _ := json.Marshal(map[string]interface{}{"action": "account_response", "request_id": action.RequestID, "success": result.Success, "error": result.Error, "data": result.Data})
			response = append(response, '\n')
			_, _ = stdin.Write(response)
		case "auth_check", "auth_grant", "auth_revoke", "points_consume", "points_add":
			result := PluginUserResult{Success: false, Error: "授权执行器不可用"}
			if authFunc != nil {
				result = authFunc(plugin.ID, PluginAuthorizationAction{Action: action.Action, RequestID: action.RequestID, TableName: action.TableName, UnionID: action.UnionID, Amount: action.Amount, Status: action.Status, Plan: action.Plan, Source: action.Source, Metadata: action.Metadata, ExpiresAt: action.ExpiresAt})
			}
			response, _ := json.Marshal(map[string]interface{}{"action": "auth_response", "request_id": action.RequestID, "success": result.Success, "error": result.Error, "data": result.Data})
			response = append(response, '\n')
			_, _ = stdin.Write(response)
		case "run_script":
			result := PluginUserResult{Success: false, Error: "脚本执行器不可用"}
			if scriptFunc != nil {
				result = scriptFunc(plugin.ID, ScriptRunAction{RequestID: action.RequestID, Runtime: action.Runtime, Script: action.Script, Cwd: action.Cwd, Env: action.Env, Timeout: action.Timeout, Wait: action.Wait, RunMode: action.RunMode, UnionID: action.UnionID})
			}
			response, _ := json.Marshal(map[string]interface{}{"action": "script_response", "request_id": action.RequestID, "success": result.Success, "error": result.Error, "data": result.Data})
			response = append(response, '\n')
			_, _ = stdin.Write(response)
		case "done":
			if !action.Success && action.Error != "" {
				log.Printf("[SYSTEM] Plugin %s error: %s", plugin.ID, action.Error)
			}
			_ = stdin.Close()
			_ = cmd.Wait()
			return nil
		}
	}

	_ = stdin.Close()
	_ = cmd.Wait()
	return scanner.Err()
}

func (m *Manager) ExecutePluginOpenAPI(plugin *types.Plugin, pluginPath string, request types.OpenAPIRequest) (types.OpenAPIResponse, error) {
	endpoint := types.OpenAPIEndpoint{ID: plugin.ID, Name: plugin.Name, Path: plugin.OpenAPI.Path, Method: plugin.OpenAPI.Method, Enabled: plugin.OpenAPI.Enabled, Token: plugin.OpenAPI.Token, Runtime: plugin.Runtime, Entry: plugin.Entry}
	return m.ExecuteOpenAPI(endpoint, pluginPath, request, nil, nil)
}

func (m *Manager) ExecuteOpenAPI(endpoint types.OpenAPIEndpoint, workDir string, request types.OpenAPIRequest, dbFunc func(string, PluginDBAction) PluginDBResult, sendMessageFunc func(string, SendMessageAction) PluginUserResult) (types.OpenAPIResponse, error) {
	maskedEndpoint := endpoint
	if maskedEndpoint.Token != "" {
		maskedEndpoint.Token = "***"
	}
	payload, err := json.Marshal(map[string]interface{}{
		"event":     "open_api_request",
		"plugin_id": endpoint.ID,
		"open_api":  maskedEndpoint,
		"request":   request,
	})
	if err != nil {
		return types.OpenAPIResponse{}, err
	}
	cmd, err := m.newOpenAPICommand(endpoint, workDir)
	if err != nil {
		return types.OpenAPIResponse{}, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return types.OpenAPIResponse{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return types.OpenAPIResponse{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return types.OpenAPIResponse{}, err
	}
	if err := cmd.Start(); err != nil {
		return types.OpenAPIResponse{}, err
	}
	go func() {
		output, _ := io.ReadAll(stderr)
		if len(output) > 0 {
			log.Printf("[SYSTEM] OpenAPI %s stderr: %s", endpoint.ID, string(output))
		}
	}()
	if _, err := stdin.Write(append(payload, '\n')); err != nil {
		_ = cmd.Process.Kill()
		return types.OpenAPIResponse{}, err
	}

	response := types.OpenAPIResponse{Status: 200, Headers: map[string]string{"Content-Type": "application/json; charset=utf-8"}}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var action struct {
			Action    string                     `json:"action"`
			RequestID string                     `json:"request_id"`
			Status    int                        `json:"status"`
			Headers   map[string]string          `json:"headers"`
			Body      string                     `json:"body"`
			JSON      interface{}                `json:"json"`
			Data      map[string]interface{}     `json:"data"`
			Success   bool                       `json:"success"`
			Error     string                     `json:"error"`
			Platform  string                     `json:"platform"`
			AdapterID string                     `json:"adapter_id"`
			UserID    string                     `json:"user_id"`
			GroupID   string                     `json:"group_id"`
			UnionID   string                     `json:"union_id"`
			Text      string                     `json:"text"`
			TextMsg   string                     `json:"text_message"`
			DBTable   string                     `json:"table"`
			DBColumns []config.PluginTableColumn `json:"db_columns"`
			Values    map[string]interface{}     `json:"values"`
			RowID     int64                      `json:"row_id"`
			Query     config.PluginDBQuery       `json:"query"`
		}
		if err := json.Unmarshal([]byte(line), &action); err != nil {
			log.Printf("[SYSTEM] OpenAPI %s invalid output line: %s", endpoint.ID, line)
			continue
		}
		switch action.Action {
		case "http_response":
			if action.Status > 0 {
				response.Status = action.Status
			}
			if action.Headers != nil {
				response.Headers = action.Headers
			}
			response.Body = action.Body
			response.JSON = action.JSON
			response.Data = action.Data
			_ = stdin.Close()
			_ = cmd.Wait()
			return response, nil
		case "db_create_table", "db_query", "db_insert", "db_update", "db_delete", "db_clear":
			result := PluginDBResult{Success: false, Error: "数据库执行器不可用"}
			if dbFunc != nil {
				result = dbFunc(endpoint.ID, PluginDBAction{Action: action.Action, RequestID: action.RequestID, Table: action.DBTable, Columns: action.DBColumns, Values: action.Values, RowID: action.RowID, Query: action.Query})
			}
			reply, _ := json.Marshal(map[string]interface{}{"action": "db_response", "request_id": action.RequestID, "success": result.Success, "error": result.Error, "data": result.Data})
			reply = append(reply, '\n')
			_, _ = stdin.Write(reply)
		case "send_message":
			result := PluginUserResult{Success: false, Error: "消息发送器不可用"}
			if sendMessageFunc != nil {
				text := action.Text
				if text == "" {
					text = action.TextMsg
				}
				result = sendMessageFunc(endpoint.ID, SendMessageAction{Platform: action.Platform, AdapterID: action.AdapterID, UserID: action.UserID, GroupID: action.GroupID, UnionID: action.UnionID, Text: text})
			}
			reply, _ := json.Marshal(map[string]interface{}{"action": "send_message_response", "request_id": action.RequestID, "success": result.Success, "error": result.Error, "data": result.Data})
			reply = append(reply, '\n')
			_, _ = stdin.Write(reply)
		case "done":
			if !action.Success && action.Error != "" {
				_ = cmd.Wait()
				return types.OpenAPIResponse{}, fmt.Errorf("%s", action.Error)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		_ = cmd.Process.Kill()
		return types.OpenAPIResponse{}, err
	}
	_ = stdin.Close()
	_ = cmd.Wait()
	return types.OpenAPIResponse{}, fmt.Errorf("Open API 未返回 http_response")
}

func (m *Manager) newOpenAPICommand(endpoint types.OpenAPIEndpoint, workDir string) (*exec.Cmd, error) {
	sdkRoot, err := filepath.Abs("sdk")
	if err != nil {
		return nil, err
	}
	switch endpoint.Runtime {
	case "python":
		pythonPath := m.depsManager.GetPythonPath()
		if _, err := os.Stat(pythonPath); err != nil {
			return nil, fmt.Errorf("Python 解释器不可用: %s", pythonPath)
		}
		cmd := exec.Command(pythonPath, filepath.Join(sdkRoot, "python", "allbot_direct.py"), "openapi", endpoint.Entry)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), fmt.Sprintf("ALLBOT_PLUGIN_ID=%s", endpoint.ID), "PYTHONUTF8=1")
		return cmd, nil
	case "nodejs":
		cmd := exec.Command("node", filepath.Join(sdkRoot, "nodejs", "allbot_direct.js"), "openapi", endpoint.Entry)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), fmt.Sprintf("ALLBOT_PLUGIN_ID=%s", endpoint.ID), fmt.Sprintf("NODE_PATH=%s", m.depsManager.GetNodePath()))
		return cmd, nil
	default:
		return nil, fmt.Errorf("不支持的运行时: %s", endpoint.Runtime)
	}
}

func (m *Manager) newDirectCommand(plugin *types.Plugin, pluginPath string) (*exec.Cmd, error) {
	entryPath, err := pluginEntryPath(pluginPath, plugin.Runtime, plugin.Entry)
	if err != nil {
		return nil, err
	}
	switch plugin.Runtime {
	case "python":
		pythonPath := m.depsManager.GetPythonPath()
		if _, err := os.Stat(pythonPath); err != nil {
			return nil, fmt.Errorf("Python 解释器不可用: %s", pythonPath)
		}
		cmd := exec.Command(pythonPath, entryPath)
		cmd.Dir = pluginPath
		cmd.Env = append(os.Environ(), fmt.Sprintf("ALLBOT_PLUGIN_ID=%s", plugin.ID), "PYTHONUTF8=1")
		return cmd, nil
	case "nodejs":
		cmd := exec.Command("node", entryPath)
		cmd.Dir = pluginPath
		cmd.Env = append(os.Environ(), fmt.Sprintf("ALLBOT_PLUGIN_ID=%s", plugin.ID), fmt.Sprintf("NODE_PATH=%s", m.depsManager.GetNodePath()))
		return cmd, nil
	default:
		return nil, fmt.Errorf("不支持的运行时: %s", plugin.Runtime)
	}
}

func (m *Manager) RunPluginScript(pluginPath string, action ScriptRunAction) PluginUserResult {
	runtimeName, fullScript, workDir, err := m.preparePluginScript(pluginPath, action)
	if err != nil {
		return PluginUserResult{Success: false, Error: err.Error()}
	}
	m.mu.RLock()
	database := m.database
	m.mu.RUnlock()
	if database == nil {
		return PluginUserResult{Success: false, Error: "数据库未初始化，无法创建脚本任务"}
	}
	scriptPath := filepath.ToSlash(action.Script)
	runMode := strings.TrimSpace(action.RunMode)
	if runMode == "" {
		runMode = "manual"
	}
	m.mu.Lock()
	if existing, err := database.FindRunningScriptRunLog(action.PluginID, scriptPath, runMode, action.UnionID); err != nil {
		m.mu.Unlock()
		return PluginUserResult{Success: false, Error: err.Error()}
	} else if existing != nil {
		data := map[string]interface{}{"log_id": existing.ID, "task_id": existing.ID, "status": existing.Status, "runtime": existing.Runtime, "script": existing.ScriptPath, "already_running": true}
		m.mu.Unlock()
		if action.Wait {
			return m.waitScriptRun(existing.ID, data, action.Timeout)
		}
		return PluginUserResult{Success: true, Data: data}
	}
	startedAt := time.Now()
	logID, reused, err := database.UpsertScriptRunLog(config.ScriptRunLog{PluginID: action.PluginID, UnionID: action.UnionID, ScriptPath: scriptPath, Runtime: runtimeName, RunMode: runMode, Status: "running", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		m.mu.Unlock()
		return PluginUserResult{Success: false, Error: err.Error()}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan ScriptRunResult, 1)
	m.runningScripts[logID] = cancel
	m.scriptDone[logID] = done
	go m.runPluginScriptTask(ctx, logID, runtimeName, fullScript, workDir, action)
	data := map[string]interface{}{"log_id": logID, "task_id": logID, "status": "running", "runtime": runtimeName, "script": scriptPath, "already_running": false, "reused": reused}
	m.mu.Unlock()
	if action.Wait {
		return m.waitScriptRun(logID, data, action.Timeout)
	}
	return PluginUserResult{Success: true, Data: data}
}

func (m *Manager) waitScriptRun(logID int64, data map[string]interface{}, timeoutSeconds int) PluginUserResult {
	m.mu.RLock()
	done := m.scriptDone[logID]
	m.mu.RUnlock()
	if done == nil {
		return PluginUserResult{Success: true, Data: data}
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 600
	}
	select {
	case result := <-done:
		data["status"] = result.Status
		data["output"] = result.Output
		data["error"] = result.Error
		data["finished_at"] = result.FinishedAt
		return PluginUserResult{Success: result.Status == "success", Error: result.Error, Data: data}
	case <-time.After(time.Duration(timeoutSeconds) * time.Second):
		data["status"] = "running"
		data["timeout"] = true
		return PluginUserResult{Success: true, Data: data}
	}
}

func (m *Manager) preparePluginScript(pluginPath string, action ScriptRunAction) (string, string, string, error) {
	runtimeName := strings.TrimSpace(action.Runtime)
	if runtimeName == "" {
		if strings.EqualFold(filepath.Ext(action.Script), ".py") {
			runtimeName = "python"
		} else {
			runtimeName = "nodejs"
		}
	}
	if runtimeName == "node" {
		runtimeName = "nodejs"
	}
	if runtimeName == "py" || runtimeName == "python3" {
		runtimeName = "python"
	}
	if runtimeName != "nodejs" && runtimeName != "python" {
		return "", "", "", fmt.Errorf("仅支持 nodejs/python 脚本运行时")
	}
	fullScript, err := safeRelativePath(pluginPath, action.Script)
	if err != nil {
		return "", "", "", err
	}
	if info, err := os.Stat(fullScript); err != nil || info.IsDir() {
		return "", "", "", fmt.Errorf("脚本文件不存在或不是文件")
	}
	workDir := filepath.Dir(fullScript)
	if strings.TrimSpace(action.Cwd) != "" {
		workDir, err = safeRelativePath(pluginPath, action.Cwd)
		if err != nil {
			return "", "", "", err
		}
	}
	return runtimeName, fullScript, workDir, nil
}

func (m *Manager) runPluginScriptTask(ctx context.Context, logID int64, runtimeName, fullScript, workDir string, action ScriptRunAction) {
	cmdName := "node"
	if runtimeName == "python" {
		cmdName = m.depsManager.GetPythonPath()
	}
	cmd := exec.CommandContext(ctx, cmdName, fullScript)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "ALLBOT_SCRIPT_RUN=1")
	if runtimeName == "nodejs" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("NODE_PATH=%s", m.depsManager.GetNodePath()))
	} else {
		cmd.Env = append(cmd.Env, "PYTHONUTF8=1")
	}
	for key, value := range action.Env {
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, "=\x00") {
			continue
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
	output := &scriptOutputBuffer{}
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		m.finishScriptRun(logID, "failed", output.String(), err.Error(), time.Now())
		return
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var err error
	for {
		select {
		case err = <-done:
			goto finished
		case <-ticker.C:
			m.updateScriptRunOutput(logID, output.String())
		}
	}

finished:
	finishedAt := time.Now()
	outputText := output.String()
	status := "success"
	errorText := ""
	if err != nil {
		if ctx.Err() == context.Canceled {
			status = "paused"
			errorText = "脚本任务已暂停"
		} else {
			status = "failed"
			errorText = err.Error()
		}
	}
	m.finishScriptRun(logID, status, outputText, errorText, finishedAt)
}

func (m *Manager) updateScriptRunOutput(logID int64, outputText string) {
	m.mu.RLock()
	database := m.database
	m.mu.RUnlock()
	if database != nil {
		_ = database.UpdateScriptRunLog(logID, "running", outputText, "", time.Time{})
	}
}

func (m *Manager) finishScriptRun(logID int64, status, outputText, errorText string, finishedAt time.Time) {
	m.mu.RLock()
	database := m.database
	m.mu.RUnlock()
	if database != nil {
		_ = database.UpdateScriptRunLog(logID, status, outputText, errorText, finishedAt)
	}
	m.mu.Lock()
	if done := m.scriptDone[logID]; done != nil {
		select {
		case done <- ScriptRunResult{Status: status, Output: outputText, Error: errorText, FinishedAt: finishedAt}:
		default:
		}
		close(done)
		delete(m.scriptDone, logID)
	}
	delete(m.runningScripts, logID)
	m.mu.Unlock()
}

type scriptOutputBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *scriptOutputBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, data...)
	return len(data), nil
}

func (b *scriptOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

func (m *Manager) PauseScriptRun(logID int64) bool {
	m.mu.Lock()
	cancel := m.runningScripts[logID]
	if cancel != nil {
		delete(m.runningScripts, logID)
	}
	m.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func safeRelativePath(root, relative string) (string, error) {
	cleanRelative := filepath.Clean(strings.TrimPrefix(strings.ReplaceAll(relative, "\\", "/"), "/"))
	if cleanRelative == "." || strings.HasPrefix(cleanRelative, "..") || filepath.IsAbs(cleanRelative) {
		return "", fmt.Errorf("路径无效")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(filepath.Join(root, cleanRelative))
	if err != nil {
		return "", err
	}
	if fullPath != rootAbs && !strings.HasPrefix(fullPath, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界")
	}
	return fullPath, nil
}

func validatePluginEntry(root, runtimeName, entry string) (string, error) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return "", fmt.Errorf("入口文件不能为空")
	}
	entry = strings.ReplaceAll(entry, "\\", "/")
	if strings.HasPrefix(entry, "/") || filepath.IsAbs(entry) {
		return "", fmt.Errorf("入口文件必须是相对路径")
	}
	cleanEntry := filepath.Clean(entry)
	if cleanEntry == "." || cleanEntry == ".." || strings.HasPrefix(cleanEntry, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("入口文件路径越界")
	}
	if err := validatePluginEntryExt(runtimeName, cleanEntry); err != nil {
		return "", err
	}
	fullPath, err := safeRelativePath(root, cleanEntry)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("入口文件不存在: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("入口文件不能是目录")
	}
	return filepath.ToSlash(cleanEntry), nil
}

func validatePluginEntryExt(runtimeName, entry string) error {
	switch runtimeName {
	case "python":
		if strings.EqualFold(filepath.Ext(entry), ".py") {
			return nil
		}
		return fmt.Errorf("Python 插件入口文件必须是 .py")
	case "nodejs":
		ext := strings.ToLower(filepath.Ext(entry))
		if ext == ".js" || ext == ".mjs" || ext == ".cjs" {
			return nil
		}
		return fmt.Errorf("Node.js 插件入口文件必须是 .js、.mjs 或 .cjs")
	default:
		return fmt.Errorf("不支持的运行时: %s", runtimeName)
	}
}

func pluginEntryPath(root, runtimeName, entry string) (string, error) {
	cleanEntry, err := validatePluginEntry(root, runtimeName, entry)
	if err != nil {
		return "", err
	}
	return safeRelativePath(root, cleanEntry)
}

func (m *Manager) loadPluginConfig(pluginPath string) (*types.Plugin, error) {
	configPath := filepath.Join(pluginPath, "plugin.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config types.PluginConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	pluginID := filepath.Base(pluginPath)
	entry, err := validatePluginEntry(pluginPath, config.Runtime, config.Entry)
	if err != nil {
		return nil, fmt.Errorf("插件入口文件无效: %w", err)
	}
	return &types.Plugin{
		ID:                pluginID,
		Name:              config.Name,
		Version:           config.Version,
		Runtime:           config.Runtime,
		Entry:             entry,
		Platforms:         config.Platforms,
		AllowedAdapterIDs: config.AllowedAdapterIDs,
		Priority:          config.Priority,
		Trigger:           config.Trigger,
		Enabled:           config.Enabled,
		UserConfig:        config.UserConfig,
		AccessControl:     pluginAccessControl(config.AccessControl),
		OpenAPI:           normalizeOpenAPIConfig(config.OpenAPI, config.Runtime),
		Template:          config.Template,
		TemplateVersion:   config.TemplateVersion,
		TemplateMetadata:  config.TemplateMetadata,
	}, nil
}

func normalizeOpenAPIConfig(config types.OpenAPIConfig, runtime string) types.OpenAPIConfig {
	config.Path = strings.Trim(strings.TrimSpace(config.Path), "/")
	config.Method = strings.ToUpper(strings.TrimSpace(config.Method))
	if config.Method == "" {
		config.Method = "POST"
	}
	config.Token = strings.TrimSpace(config.Token)
	config.Runtime = strings.TrimSpace(config.Runtime)
	if config.Runtime == "" {
		config.Runtime = runtime
	}
	return config
}

func pluginAccessControl(config *types.AccessControlConfig) types.AccessControlConfig {
	if config == nil {
		return types.AccessControlConfig{InheritSystem: true}
	}
	return *config
}

func (m *Manager) installDeps(plugin *types.Plugin) {
	configPath := filepath.Join(m.pluginDir, plugin.ID, "plugin.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var config types.PluginConfig
	if err := json.Unmarshal(data, &config); err != nil || len(config.Dependencies) == 0 {
		return
	}
	switch config.Runtime {
	case "python":
		if err := m.depsManager.InstallPythonDeps(config.Dependencies); err != nil {
			log.Printf("[SYSTEM] 安装插件 %s Python 依赖失败: %v", config.Name, err)
		}
	case "nodejs":
		if err := m.depsManager.InstallNodeDeps(config.Dependencies); err != nil {
			log.Printf("[SYSTEM] 安装插件 %s Node.js 依赖失败: %v", config.Name, err)
		}
	}
}

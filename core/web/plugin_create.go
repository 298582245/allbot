package web

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/router"
	"github.com/allbot/allbot/core/types"
)

type createPluginRequest struct {
	ID               string                        `json:"id"`
	Name             string                        `json:"name"`
	Version          string                        `json:"version"`
	Runtime          string                        `json:"runtime"`
	Trigger          string                        `json:"trigger"`
	Priority         int                           `json:"priority"`
	Platforms        []string                      `json:"platforms"`
	Enabled          bool                          `json:"enabled"`
	UserConfigSchema []types.PluginUserConfigField `json:"user_config_schema"`
	UserConfig       map[string]interface{}        `json:"user_config"`
	Template         string                        `json:"template"`
	AccountQL        *createAccountQLRequest       `json:"account_ql"`
}

type createAccountQLRequest struct {
	Prefix                string                        `json:"prefix"`
	TableName             string                        `json:"table_name"`
	EnvName               string                        `json:"env_name"`
	TaskScript            string                        `json:"task_script"`
	ScriptRuntime         string                        `json:"script_runtime"`
	AuthPricePerMonth     int                           `json:"auth_price_per_month"`
	Cron                  string                        `json:"cron"`
	CKCheckCron           string                        `json:"ck_check_cron"`
	RunWaitTimeout        int                           `json:"run_wait_timeout"`
	ParseInputCode        string                        `json:"parse_input_code"`
	QueryCode             string                        `json:"query_code"`
	EnableCKCheck         *bool                         `json:"enable_ck_check"`
	CheckCKCode           string                        `json:"check_ck_code"`
	EnableExpireCheck     *bool                         `json:"enable_expire_check"`
	ExpireCheckCron       string                        `json:"expire_check_cron"`
	ExpireNotifyDays      string                        `json:"expire_notify_days"`
	ExpireDeleteAfterDays *int                          `json:"expire_delete_after_days"`
	Routes                []createAccountQLRouteRequest `json:"routes"`
}

type createAccountQLRouteRequest struct {
	Command      string `json:"command"`
	FunctionName string `json:"function_name"`
	Description  string `json:"description"`
	Code         string `json:"code"`
}

type createPluginPlan struct {
	PluginID        string
	Template        string
	TemplateVersion string
	Runtime         string
	Entry           string
	Trigger         string
	Commands        []string
	Config          types.PluginConfig
	AccountQL       *accountQLTemplate
	Metadata        map[string]interface{}
}

type createGeneratedFile struct {
	Path    string `json:"path"`
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
	Bytes   int    `json:"bytes"`
	Written bool   `json:"written,omitempty"`
	Error   string `json:"error,omitempty"`
}

type createValidationIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Tab     string `json:"tab"`
}

type createPluginDiagnostic struct {
	Step    string `json:"step"`
	Target  string `json:"target,omitempty"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type pluginTemplateInfo struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Runtime     string                 `json:"runtime"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Features    []string               `json:"features"`
	Defaults    map[string]interface{} `json:"defaults"`
}

type accountQLTemplate struct {
	PluginID              string
	PluginName            string
	Runtime               string
	ScriptRuntime         string
	Prefix                string
	TableName             string
	EnvName               string
	TaskScript            string
	AuthPricePerMonth     int
	Cron                  string
	CKCheckCron           string
	RunWaitTimeout        int
	ParseInputCode        string
	QueryCode             string
	EnableCKCheck         bool
	CheckCKCode           string
	EnableExpireCheck     bool
	ExpireCheckCron       string
	ExpireNotifyDays      string
	ExpireDeleteAfterDays int
	Routes                []accountQLRouteTemplate
}

type accountQLRouteTemplate struct {
	Command      string
	FunctionName string
	Description  string
	Code         string
}

const accountQLTemplateVersion = "3.0.0"

var defaultPluginPlatforms = []string{"qq", "qq_office", "telegram"}

var (
	accountQLIdentifierPattern     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	accountQLJSFunctionNamePattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)
	accountQLWindowsDrivePattern   = regexp.MustCompile(`^[A-Za-z]:`)
	pluginIDSanitizePattern        = regexp.MustCompile(`[^a-z0-9_\-]+`)
	configKeySanitizePattern       = regexp.MustCompile(`[^A-Za-z0-9_]+`)
)

func (s *Server) handleCreatePlugin(w http.ResponseWriter, r *http.Request) {
	var req createPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	plan, err := buildCreatePluginPlan(req)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if issues := validateCreatePluginPlan(plan); len(issues) > 0 {
		s.jsonError(w, issues[0].Message, http.StatusBadRequest)
		return
	}

	pluginPath := filepath.Join("plugins", plan.PluginID)
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0755); err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.Mkdir(pluginPath, 0755); err != nil {
		if os.IsExist(err) {
			s.jsonError(w, "插件目录已存在: "+plan.PluginID, http.StatusBadRequest)
			return
		}
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	metadataSaved := false
	cleanupOnError := func() {
		_ = os.RemoveAll(pluginPath)
		if metadataSaved {
			_ = s.deletePlanTemplateMetadata(plan.PluginID)
		}
	}
	diagnostics := make([]createPluginDiagnostic, 0)
	files := renderCreatePluginFiles(plan)
	for index := range files {
		file := &files[index]
		if err := writeGeneratedPluginFile(pluginPath, *file); err != nil {
			file.Error = err.Error()
			diagnostics = append(diagnostics, createPluginDiagnostic{Step: "write_file", Target: file.Path, OK: false, Message: err.Error()})
			cleanupOnError()
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		file.Written = true
		diagnostics = append(diagnostics, createPluginDiagnostic{Step: "write_file", Target: file.Path, OK: true, Message: "写入成功"})
	}

	if err := s.savePlanTemplateMetadata(plan); err != nil {
		diagnostics = append(diagnostics, createPluginDiagnostic{Step: "save_metadata", OK: false, Message: err.Error()})
		cleanupOnError()
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	metadataSaved = true
	diagnostics = append(diagnostics, createPluginDiagnostic{Step: "save_metadata", OK: true, Message: "模板元数据保存成功"})

	plugin, err := s.pluginManager.LoadPlugin(pluginPath)
	if err != nil {
		diagnostics = append(diagnostics, createPluginDiagnostic{Step: "load_plugin", OK: false, Message: err.Error()})
		cleanupOnError()
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	diagnostics = append(diagnostics, createPluginDiagnostic{Step: "load_plugin", OK: true, Message: "插件加载成功"})
	if err := s.router.RegisterPlugin(plugin); err != nil {
		diagnostics = append(diagnostics, createPluginDiagnostic{Step: "register_plugin", OK: false, Message: err.Error()})
		cleanupOnError()
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	diagnostics = append(diagnostics, createPluginDiagnostic{Step: "register_plugin", OK: true, Message: "路由注册成功"})

	response := createPluginPlanResponse(plan, files, nil, nil)
	response["message"] = "插件创建成功"
	response["id"] = plan.PluginID
	response["diagnostics"] = diagnostics
	s.jsonResponse(w, response)
}

func (s *Server) handlePluginTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.jsonResponse(w, pluginCreateTemplates())
}

func (s *Server) handlePluginCreatePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req createPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	plan, err := buildCreatePluginPlan(req)
	if err != nil {
		s.jsonResponse(w, map[string]interface{}{
			"errors":     []createValidationIssue{buildCreateValidationIssue(err)},
			"warnings":   []createValidationIssue{},
			"normalized": createRequestNormalized(req),
		})
		return
	}
	issues := validateCreatePluginPlan(plan)
	s.jsonResponse(w, createPluginPlanResponse(plan, renderCreatePluginFiles(plan), issues, nil))
}

func (s *Server) handlePluginCreateValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req createPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	plan, err := buildCreatePluginPlan(req)
	if err != nil {
		issue := buildCreateValidationIssue(err)
		s.jsonResponse(w, map[string]interface{}{"ok": false, "errors": []createValidationIssue{issue}, "warnings": []createValidationIssue{}, "normalized": createRequestNormalized(req)})
		return
	}
	issues := validateCreatePluginPlan(plan)
	s.jsonResponse(w, map[string]interface{}{"ok": len(issues) == 0, "errors": issues, "warnings": []createValidationIssue{}, "normalized": createPlanNormalized(plan)})
}

func buildCreatePluginPlan(req createPluginRequest) (*createPluginPlan, error) {
	pluginID := sanitizePluginID(req.ID)
	if pluginID == "" {
		pluginID = sanitizePluginID(req.Name)
	}
	if pluginID == "" && strings.TrimSpace(req.Name) != "" {
		pluginID = uniquePluginID("plugin")
	}
	if pluginID == "" {
		return nil, fmt.Errorf("插件名称不能为空")
	}
	if req.Name = strings.TrimSpace(req.Name); req.Name == "" {
		req.Name = pluginID
	}
	if req.Version = strings.TrimSpace(req.Version); req.Version == "" {
		req.Version = "1.0.0"
	}
	templateName := strings.TrimSpace(req.Template)
	if templateName == "" {
		templateName = "basic"
	}
	var accountTemplate *accountQLTemplate
	commands := []string{}
	if templateName == "nodejs_account_ql" || templateName == "python_account_ql" {
		template, err := normalizeAccountQLTemplate(pluginID, templateName, &req)
		if err != nil {
			return nil, err
		}
		accountTemplate = template
		req.Runtime = template.Runtime
		req.Trigger = accountQLTrigger(*template)
		req.UserConfigSchema = accountQLUserConfigSchema(*template)
		req.UserConfig = accountQLUserConfig(*template)
		commands = accountQLTriggerCommands(*template)
	} else if templateName != "basic" {
		return nil, fmt.Errorf("不支持的插件模板: %s", templateName)
	}
	if req.Runtime != "nodejs" && req.Runtime != "python" {
		return nil, fmt.Errorf("运行时只支持 nodejs 或 python")
	}
	if len(req.Platforms) == 0 {
		req.Platforms = append([]string(nil), defaultPluginPlatforms...)
	}
	if accountTemplate == nil {
		req.Trigger = strings.TrimSpace(req.Trigger)
		req.UserConfigSchema = normalizeCreatePluginSchema(req.UserConfigSchema)
		req.UserConfig = normalizeCreatePluginUserConfig(req.UserConfigSchema, req.UserConfig)
	}
	entry := "main.py"
	if req.Runtime == "nodejs" {
		entry = "main.js"
	}
	plan := &createPluginPlan{PluginID: pluginID, Template: templateName, TemplateVersion: accountQLTemplateVersion, Runtime: req.Runtime, Entry: entry, Trigger: req.Trigger, Commands: commands, AccountQL: accountTemplate}
	plan.Metadata = buildAccountQLTemplateMetadata(plan)
	accessControl := types.AccessControlConfig{InheritSystem: true}
	plan.Config = types.PluginConfig{
		Name:              req.Name,
		Version:           req.Version,
		Runtime:           req.Runtime,
		Entry:             entry,
		Platforms:         req.Platforms,
		AllowedAdapterIDs: []string{},
		Priority:          req.Priority,
		Trigger:           req.Trigger,
		Enabled:           req.Enabled,
		Dependencies:      map[string]string{},
		UserConfigSchema:  req.UserConfigSchema,
		UserConfig:        req.UserConfig,
		AccessControl:     &accessControl,
		OpenAPI:           types.OpenAPIConfig{Enabled: false, Path: pluginID, Method: "POST", Runtime: req.Runtime},
		Template:          plan.Template,
		TemplateVersion:   plan.TemplateVersion,
		TemplateMetadata:  plan.Metadata,
	}
	return plan, nil
}

func renderCreatePluginConfig(plan *createPluginPlan) types.PluginConfig {
	return plan.Config
}

func renderCreatePluginFiles(plan *createPluginPlan) []createGeneratedFile {
	configBytes, _ := json.MarshalIndent(renderCreatePluginConfig(plan), "", "  ")
	files := []createGeneratedFile{{Path: "plugin.json", Role: "config", Content: string(configBytes), Bytes: len(configBytes)}}
	entryCode := pluginTemplate(plan.Runtime, plan.Config.UserConfigSchema)
	if plan.AccountQL != nil {
		entryCode = accountQLPluginTemplate(*plan.AccountQL)
	}
	files = append(files, createGeneratedFile{Path: plan.Entry, Role: "entry", Content: entryCode, Bytes: len([]byte(entryCode))})
	if plan.AccountQL != nil {
		scriptCode := accountQLTaskScriptTemplate(plan.AccountQL.ScriptRuntime, plan.AccountQL.EnvName)
		files = append(files, createGeneratedFile{Path: plan.AccountQL.TaskScript, Role: "task_script", Content: scriptCode, Bytes: len([]byte(scriptCode))})
	}
	return files
}

func buildAccountQLTemplateMetadata(plan *createPluginPlan) map[string]interface{} {
	metadata := map[string]interface{}{"source": "builtin", "structure": createPlanStructure(plan)}
	if plan.AccountQL == nil {
		return metadata
	}
	metadata["prefix"] = plan.AccountQL.Prefix
	metadata["table_name"] = plan.AccountQL.TableName
	metadata["env_name"] = plan.AccountQL.EnvName
	metadata["task_script"] = plan.AccountQL.TaskScript
	metadata["script_runtime"] = plan.AccountQL.ScriptRuntime
	metadata["commands"] = plan.Commands
	metadata["enable_ck_check"] = plan.AccountQL.EnableCKCheck
	metadata["enable_expire_check"] = plan.AccountQL.EnableExpireCheck
	routes := make([]map[string]string, 0, len(plan.AccountQL.Routes))
	for _, route := range plan.AccountQL.Routes {
		routes = append(routes, map[string]string{"command": route.Command, "function_name": route.FunctionName, "description": route.Description})
	}
	metadata["routes"] = routes
	return metadata
}

func validateCreatePluginPlan(plan *createPluginPlan) []createValidationIssue {
	issues := make([]createValidationIssue, 0)
	if strings.TrimSpace(plan.Trigger) == "" {
		issues = append(issues, createValidationIssue{Field: "trigger", Message: "触发正则不能为空", Tab: "base"})
	} else if _, err := regexp.Compile(plan.Trigger); err != nil {
		issues = append(issues, createValidationIssue{Field: "trigger", Message: "触发正则无效: " + err.Error(), Tab: "base"})
	}
	if plan.AccountQL != nil {
		issues = append(issues, validateCronIssue("account_ql.cron", "默认定时表达式", plan.AccountQL.Cron)...)
		if plan.AccountQL.EnableCKCheck {
			issues = append(issues, validateCronIssue("account_ql.ck_check_cron", "CK 检测定时表达式", plan.AccountQL.CKCheckCron)...)
		}
		if plan.AccountQL.EnableExpireCheck {
			issues = append(issues, validateCronIssue("account_ql.expire_check_cron", "过期检测定时表达式", plan.AccountQL.ExpireCheckCron)...)
		}
	}
	return issues
}

func validateCronIssue(field string, label string, expression string) []createValidationIssue {
	if _, err := router.NextCronTime(expression, time.Now()); err != nil {
		return []createValidationIssue{{Field: field, Message: label + "无效: " + err.Error(), Tab: "ql"}}
	}
	return nil
}

func writeGeneratedPluginFile(pluginPath string, file createGeneratedFile) error {
	fullPath := filepath.Join(pluginPath, filepath.FromSlash(file.Path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(file.Content), 0644)
}

func createPluginPlanResponse(plan *createPluginPlan, files []createGeneratedFile, errors []createValidationIssue, warnings []createValidationIssue) map[string]interface{} {
	if errors == nil {
		errors = []createValidationIssue{}
	}
	if warnings == nil {
		warnings = []createValidationIssue{}
	}
	return map[string]interface{}{
		"plugin_id":          plan.PluginID,
		"template":           plan.Template,
		"template_version":   plan.TemplateVersion,
		"runtime":            plan.Runtime,
		"entry":              plan.Entry,
		"trigger":            plan.Trigger,
		"commands":           plan.Commands,
		"user_config_schema": plan.Config.UserConfigSchema,
		"user_config":        plan.Config.UserConfig,
		"files":              files,
		"metadata":           plan.Metadata,
		"errors":             errors,
		"warnings":           warnings,
		"normalized":         createPlanNormalized(plan),
	}
}

func createPlanNormalized(plan *createPluginPlan) map[string]interface{} {
	normalized := map[string]interface{}{
		"plugin_id":        plan.PluginID,
		"template":         plan.Template,
		"template_version": plan.TemplateVersion,
		"runtime":          plan.Runtime,
		"entry":            plan.Entry,
		"trigger":          plan.Trigger,
		"commands":         plan.Commands,
	}
	if plan.AccountQL != nil {
		normalized["script_runtime"] = plan.AccountQL.ScriptRuntime
		normalized["task_script"] = plan.AccountQL.TaskScript
	}
	return normalized
}

func createRequestNormalized(req createPluginRequest) map[string]interface{} {
	normalized := map[string]interface{}{"plugin_id": sanitizePluginID(firstNonEmpty(req.ID, req.Name)), "template": firstNonEmpty(req.Template, "basic"), "runtime": req.Runtime}
	if req.AccountQL != nil {
		normalized["script_runtime"] = normalizeAccountQLScriptRuntime(req.AccountQL.ScriptRuntime, req.AccountQL.TaskScript, req.Runtime)
		normalized["task_script"] = strings.TrimSpace(strings.ReplaceAll(req.AccountQL.TaskScript, "\\", "/"))
	}
	return normalized
}

func buildCreateValidationIssue(err error) createValidationIssue {
	message := err.Error()
	field, tab := "template", "base"
	switch {
	case strings.Contains(message, "指令前缀"):
		field, tab = "account_ql.prefix", "ql"
	case strings.Contains(message, "青龙变量名"):
		field, tab = "account_ql.env_name", "ql"
	case strings.Contains(message, "账号表名"):
		field, tab = "account_ql.table_name", "ql"
	case strings.Contains(message, "青龙脚本语言"):
		field, tab = "account_ql.script_runtime", "ql"
	case strings.Contains(message, "青龙脚本路径") || strings.Contains(message, "青龙脚本只支持"):
		field, tab = "account_ql.task_script", "ql"
	case strings.Contains(message, "登录解析代码"):
		field, tab = "account_ql.parse_input_code", "code"
	case strings.Contains(message, "查询代码"):
		field, tab = "account_ql.query_code", "code"
	case strings.Contains(message, "CK 检测代码"):
		field, tab = "account_ql.check_ck_code", "code"
	case strings.Contains(message, "自定义指令"):
		field, tab = "account_ql.routes", "routes"
	case strings.Contains(message, "运行时"):
		field, tab = "runtime", "base"
	case strings.Contains(message, "插件名称"):
		field, tab = "name", "base"
	}
	return createValidationIssue{Field: field, Message: message, Tab: tab}
}

func pluginCreateTemplates() []pluginTemplateInfo {
	return []pluginTemplateInfo{
		{ID: "basic", Name: "普通插件", Runtime: "nodejs", Version: accountQLTemplateVersion, Description: "生成基础 Node.js 或 Python 插件骨架", Features: []string{"基础触发正则", "用户配置", "空依赖"}, Defaults: map[string]interface{}{"runtime": "nodejs", "version": "1.0.0", "platforms": append([]string(nil), defaultPluginPlatforms...)}},
		{ID: "nodejs_account_ql", Name: "Node.js 青龙账号插件", Runtime: "nodejs", Version: accountQLTemplateVersion, Description: "生成 Node.js 青龙账号插件、任务脚本和账号授权配置", Features: []string{"账号登录", "账号查询", "青龙脚本运行", "CK 检测", "自定义指令"}, Defaults: accountQLTemplateDefaults("nodejs")},
		{ID: "python_account_ql", Name: "Python 青龙账号插件", Runtime: "python", Version: accountQLTemplateVersion, Description: "生成 Python 青龙账号插件、任务脚本和账号授权配置", Features: []string{"账号登录", "账号查询", "青龙脚本运行", "CK 检测", "自定义指令"}, Defaults: accountQLTemplateDefaults("python")},
	}
}

func accountQLTemplateDefaults(runtime string) map[string]interface{} {
	ext := "js"
	if runtime == "python" {
		ext = "py"
	}
	return map[string]interface{}{
		"runtime":                  runtime,
		"script_runtime":           runtime,
		"version":                  "1.0.0",
		"task_script":              "scripts/task." + ext,
		"cron":                     "0 8 * * *",
		"ck_check_cron":            "25 9 * * *",
		"expire_check_cron":        "15 9 * * *",
		"expire_notify_days":       "7,3,1,0",
		"expire_delete_after_days": -1,
		"run_wait_timeout":         7200,
	}
}

func createPlanStructure(plan *createPluginPlan) string {
	if plan.AccountQL != nil {
		return "account_ql"
	}
	return "basic"
}

func (s *Server) savePlanTemplateMetadata(plan *createPluginPlan) error {
	database := s.pluginTemplateMetadataDatabase()
	if database == nil {
		return fmt.Errorf("数据库不可用，无法保存模板元数据")
	}
	return database.SavePluginTemplateMetadata(&config.PluginTemplateMetadata{PluginID: plan.PluginID, Template: plan.Template, TemplateVersion: plan.TemplateVersion, Runtime: plan.Runtime, Structure: createPlanStructure(plan), Metadata: plan.Metadata})
}

func (s *Server) deletePlanTemplateMetadata(pluginID string) error {
	database := s.pluginTemplateMetadataDatabase()
	if database == nil {
		return nil
	}
	return database.DeletePluginTemplateMetadata(pluginID)
}

func (s *Server) pluginTemplateMetadataDatabase() *config.Database {
	if s.adapterManager == nil {
		return nil
	}
	return s.adapterManager.GetDatabase()
}

func normalizeNodeAccountQLTemplate(pluginID string, req *createPluginRequest) (*accountQLTemplate, error) {
	return normalizeAccountQLTemplate(pluginID, "nodejs_account_ql", req)
}

func normalizeAccountQLTemplate(pluginID string, templateName string, req *createPluginRequest) (*accountQLTemplate, error) {
	if req.AccountQL == nil {
		return nil, fmt.Errorf("青龙账号模板配置不能为空")
	}
	runtime := "nodejs"
	if templateName == "python_account_ql" {
		runtime = "python"
	}
	if currentRuntime := strings.TrimSpace(req.Runtime); currentRuntime != "" && currentRuntime != runtime {
		return nil, fmt.Errorf("%s 青龙账号插件模板只支持 %s 运行时", accountQLRuntimeLabel(runtime), runtime)
	}
	accountQL := req.AccountQL
	options := &accountQLTemplate{
		PluginID:          pluginID,
		PluginName:        strings.TrimSpace(req.Name),
		Runtime:           runtime,
		ScriptRuntime:     normalizeAccountQLScriptRuntime(accountQL.ScriptRuntime, accountQL.TaskScript, runtime),
		Prefix:            strings.TrimSpace(accountQL.Prefix),
		TableName:         strings.TrimSpace(accountQL.TableName),
		EnvName:           strings.TrimSpace(accountQL.EnvName),
		AuthPricePerMonth: accountQL.AuthPricePerMonth,
		Cron:              strings.TrimSpace(accountQL.Cron),
		CKCheckCron:       strings.TrimSpace(accountQL.CKCheckCron),
		RunWaitTimeout:    accountQL.RunWaitTimeout,
		ParseInputCode:    strings.TrimSpace(accountQL.ParseInputCode),
		QueryCode:         strings.TrimSpace(accountQL.QueryCode),
		CheckCKCode:       strings.TrimSpace(accountQL.CheckCKCode),
		ExpireCheckCron:   strings.TrimSpace(accountQL.ExpireCheckCron),
		ExpireNotifyDays:  strings.TrimSpace(accountQL.ExpireNotifyDays),
	}
	options.EnableCKCheck = boolPointerDefault(accountQL.EnableCKCheck, true)
	options.EnableExpireCheck = boolPointerDefault(accountQL.EnableExpireCheck, false)
	options.ExpireDeleteAfterDays = intPointerDefault(accountQL.ExpireDeleteAfterDays, -1)
	if options.PluginName == "" {
		options.PluginName = pluginID
	}
	if options.Prefix == "" {
		return nil, fmt.Errorf("指令前缀不能为空")
	}
	if err := validateAccountQLEnvName(options.EnvName); err != nil {
		return nil, err
	}
	if err := validateAccountQLTableName(options.TableName); err != nil {
		return nil, err
	}
	taskScript, err := validateAccountQLTaskScript(accountQL.TaskScript, runtime, options.ScriptRuntime)
	if err != nil {
		return nil, err
	}
	options.TaskScript = taskScript
	if options.AuthPricePerMonth < 0 {
		options.AuthPricePerMonth = 0
	}
	if options.Cron == "" {
		options.Cron = "0 8 * * *"
	}
	if options.CKCheckCron == "" {
		options.CKCheckCron = "25 9 * * *"
	}
	if options.RunWaitTimeout <= 0 {
		options.RunWaitTimeout = 7200
	}
	if options.ExpireCheckCron == "" {
		options.ExpireCheckCron = "15 9 * * *"
	}
	if options.ExpireNotifyDays == "" {
		options.ExpireNotifyDays = "7,3,1,0"
	}
	parseFunction, queryFunction, ckFunction := accountQLFunctionNames(runtime)
	if !containsAccountQLFunctionDefinition(runtime, options.ParseInputCode, parseFunction) {
		return nil, fmt.Errorf("登录解析代码必须定义 %s 函数", parseFunction)
	}
	if !containsAccountQLFunctionDefinition(runtime, options.QueryCode, queryFunction) {
		return nil, fmt.Errorf("查询代码必须定义 %s 函数", queryFunction)
	}
	if options.EnableCKCheck && !containsAccountQLFunctionDefinition(runtime, options.CheckCKCode, ckFunction) {
		return nil, fmt.Errorf("CK 检测代码必须定义 %s 函数", ckFunction)
	}
	routes, err := normalizeAccountQLRoutes(runtime, *options, accountQL.Routes)
	if err != nil {
		return nil, err
	}
	options.Routes = routes
	return options, nil
}

func accountQLRuntimeLabel(runtime string) string {
	if runtime == "python" {
		return "Python"
	}
	return "Node.js"
}

func normalizeAccountQLScriptRuntime(value string, taskScript string, pluginRuntime string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value != "" {
		switch value {
		case "node", "js", "javascript":
			return "nodejs"
		case "nodejs", "python":
			return value
		default:
			return ""
		}
	}
	switch strings.ToLower(path.Ext(strings.TrimSpace(strings.ReplaceAll(taskScript, "\\", "/")))) {
	case ".py":
		return "python"
	case ".js", ".mjs", ".cjs":
		return "nodejs"
	}
	return pluginRuntime
}

func boolPointerDefault(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func intPointerDefault(value *int, defaultValue int) int {
	if value == nil {
		return defaultValue
	}
	return *value
}

func accountQLFunctionNames(runtime string) (string, string, string) {
	if runtime == "python" {
		return "parse_input", "query", "check_ck"
	}
	return "parseInput", "query", "checkCk"
}

func normalizeAccountQLRoutes(runtime string, options accountQLTemplate, routes []createAccountQLRouteRequest) ([]accountQLRouteTemplate, error) {
	result := make([]accountQLRouteTemplate, 0, len(routes))
	seenCommands := map[string]bool{}
	for _, command := range accountQLTriggerCommands(options) {
		seenCommands[command] = true
	}
	parseFunction, queryFunction, ckFunction := accountQLFunctionNames(runtime)
	seenFunctions := map[string]bool{parseFunction: true, queryFunction: true, ckFunction: true}
	for index, route := range routes {
		command := strings.TrimSpace(route.Command)
		if command == "" {
			return nil, fmt.Errorf("自定义指令不能为空")
		}
		if seenCommands[command] {
			return nil, fmt.Errorf("自定义指令不能与内置指令或其他指令重复: %s", command)
		}
		functionName := strings.TrimSpace(route.FunctionName)
		if functionName == "" {
			functionName = defaultAccountQLRouteFunctionName(runtime, index+1)
		}
		if !isValidAccountQLFunctionName(runtime, functionName) {
			return nil, fmt.Errorf("自定义指令 %s 的函数名格式无效", command)
		}
		if seenFunctions[functionName] {
			return nil, fmt.Errorf("自定义指令函数名重复: %s", functionName)
		}
		code := strings.TrimSpace(route.Code)
		if !containsAccountQLFunctionDefinition(runtime, code, functionName) {
			return nil, fmt.Errorf("自定义指令 %s 的代码必须定义 %s 函数", command, functionName)
		}
		seenCommands[command] = true
		seenFunctions[functionName] = true
		result = append(result, accountQLRouteTemplate{Command: command, FunctionName: functionName, Description: strings.TrimSpace(route.Description), Code: code})
	}
	return result, nil
}

func defaultAccountQLRouteFunctionName(runtime string, index int) string {
	if runtime == "python" {
		return fmt.Sprintf("custom_route_%d", index)
	}
	return fmt.Sprintf("customRoute%d", index)
}

func isValidAccountQLFunctionName(runtime string, value string) bool {
	if strings.TrimSpace(value) == "" || reservedConfigVariableNames[strings.ToLower(value)] {
		return false
	}
	if runtime == "python" {
		return accountQLIdentifierPattern.MatchString(value)
	}
	return accountQLJSFunctionNamePattern.MatchString(value)
}

func containsAccountQLFunctionDefinition(runtime string, code string, functionName string) bool {
	if !isValidAccountQLFunctionName(runtime, functionName) {
		return false
	}
	if runtime == "python" {
		return regexp.MustCompile(`(?m)^\s*(async\s+def|def)\s+` + regexp.QuoteMeta(functionName) + `\s*\(`).MatchString(code)
	}
	return regexp.MustCompile(`(?m)^\s*(async\s+function|function)\s+` + regexp.QuoteMeta(functionName) + `\s*\(`).MatchString(code)
}

func accountQLTrigger(options accountQLTemplate) string {
	commands := accountQLTriggerCommands(options)
	parts := make([]string, 0, len(commands))
	for _, command := range commands {
		parts = append(parts, regexp.QuoteMeta(command))
	}
	return "^(" + regexp.QuoteMeta(options.Prefix) + ")(" + strings.Join(parts, "|") + ")$"
}

func accountQLTriggerCommands(options accountQLTemplate) []string {
	commands := []string{"登录", "账号", "管理", "查询", "运行", "一键运行", "签到", "删除", "授权", "帮助"}
	if options.EnableCKCheck {
		commands = append(commands, "CK检测")
	}
	if options.EnableExpireCheck {
		commands = append(commands, "过期检测")
	}
	for _, route := range options.Routes {
		commands = append(commands, route.Command)
	}
	return commands
}

func nodeAccountQLTrigger(prefix string) string {
	return accountQLTrigger(accountQLTemplate{Prefix: prefix, EnableCKCheck: true})
}

func validateAccountQLEnvName(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("青龙变量名不能为空")
	}
	if !accountQLIdentifierPattern.MatchString(value) {
		return fmt.Errorf("青龙变量名格式无效")
	}
	return nil
}

func validateAccountQLTableName(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("账号表名不能为空")
	}
	if !accountQLIdentifierPattern.MatchString(value) {
		return fmt.Errorf("账号表名格式无效")
	}
	return nil
}

func validateAccountQLTaskScript(value string, pluginRuntime string, scriptRuntime string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "", fmt.Errorf("青龙脚本路径不能为空")
	}
	if scriptRuntime != "nodejs" && scriptRuntime != "python" {
		return "", fmt.Errorf("青龙脚本语言只支持 nodejs 或 python")
	}
	if strings.HasPrefix(value, "/") || filepath.IsAbs(value) || accountQLWindowsDrivePattern.MatchString(value) {
		return "", fmt.Errorf("青龙脚本路径必须是插件目录内相对路径")
	}
	cleanPath := path.Clean(value)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("青龙脚本路径不能越界")
	}
	entry := "main.js"
	if pluginRuntime == "python" {
		entry = "main.py"
	}
	if cleanPath == entry {
		return "", fmt.Errorf("青龙脚本路径不能覆盖插件入口文件")
	}
	if scriptRuntime == "python" {
		if strings.ToLower(path.Ext(cleanPath)) != ".py" {
			return "", fmt.Errorf("Python 青龙脚本只支持 .py")
		}
		return cleanPath, nil
	}
	switch strings.ToLower(path.Ext(cleanPath)) {
	case ".js", ".mjs", ".cjs":
		return cleanPath, nil
	default:
		return "", fmt.Errorf("Node.js 青龙脚本只支持 .js、.mjs 或 .cjs")
	}
}

func accountQLUserConfigSchema(options accountQLTemplate) []types.PluginUserConfigField {
	fields := []types.PluginUserConfigField{
		{Key: "task_script", Label: "青龙脚本路径", Type: "text", Required: true, Default: options.TaskScript, Description: "相对插件目录的脚本路径，原青龙脚本不需要修改"},
		{Key: "script_runtime", Label: "青龙脚本语言", Type: "select", Required: true, Default: options.ScriptRuntime, Description: "青龙任务脚本运行语言，可与插件入口语言不同"},
		{Key: "auth_price_per_month", Label: "账号授权价格", Type: "number", Default: options.AuthPricePerMonth, Description: "用户自助授权单个账号每月扣除的积分，0 表示免费授权"},
		{Key: "cron", Label: "默认定时表达式", Type: "text", Required: true, Default: options.Cron, Description: "插件触发一次后自动声明默认运行任务"},
		{Key: "run_wait_timeout", Label: "等待脚本完成秒数", Type: "number", Default: options.RunWaitTimeout, Description: "用户运行时等待脚本结束并回传日志，超时后可到后台脚本任务查看"},
	}
	if options.EnableCKCheck {
		fields = append(fields, types.PluginUserConfigField{Key: "ck_check_cron", Label: "CK 检测定时表达式", Type: "text", Required: true, Default: options.CKCheckCron, Description: "每日检测账号 CK 是否失效，仅通知用户，不自动删除账号"})
	}
	if options.EnableExpireCheck {
		fields = append(fields,
			types.PluginUserConfigField{Key: "expire_check_cron", Label: "过期检测定时表达式", Type: "text", Required: true, Default: options.ExpireCheckCron, Description: "每日检测账号授权到期并推送续费提醒"},
			types.PluginUserConfigField{Key: "expire_notify_days", Label: "过期提醒天数", Type: "text", Default: options.ExpireNotifyDays, Description: "逗号分隔，0 表示到期当天"},
			types.PluginUserConfigField{Key: "expire_delete_after_days", Label: "过期后自动删除天数", Type: "number", Default: options.ExpireDeleteAfterDays, Description: "-1 表示不自动删除；0 表示到期后检测到就删除；正数表示过期 N 天后删除"},
		)
	}
	return fields
}

func accountQLUserConfig(options accountQLTemplate) map[string]interface{} {
	config := map[string]interface{}{
		"task_script":          options.TaskScript,
		"script_runtime":       options.ScriptRuntime,
		"auth_price_per_month": options.AuthPricePerMonth,
		"cron":                 options.Cron,
		"run_wait_timeout":     options.RunWaitTimeout,
	}
	if options.EnableCKCheck {
		config["ck_check_cron"] = options.CKCheckCron
	}
	if options.EnableExpireCheck {
		config["expire_check_cron"] = options.ExpireCheckCron
		config["expire_notify_days"] = options.ExpireNotifyDays
		config["expire_delete_after_days"] = options.ExpireDeleteAfterDays
	}
	return config
}

func normalizeCreatePluginSchema(fields []types.PluginUserConfigField) []types.PluginUserConfigField {
	result := make([]types.PluginUserConfigField, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		field.Key = sanitizeConfigKey(field.Key)
		field.Label = strings.TrimSpace(field.Label)
		field.Description = strings.TrimSpace(field.Description)
		if field.Key == "" || seen[field.Key] {
			continue
		}
		seen[field.Key] = true
		if field.Label == "" {
			field.Label = field.Key
		}
		if field.Type == "" {
			field.Type = "text"
		}
		result = append(result, field)
	}
	return result
}

func normalizeCreatePluginUserConfig(fields []types.PluginUserConfigField, values map[string]interface{}) map[string]interface{} {
	if values == nil {
		values = map[string]interface{}{}
	}
	for _, field := range fields {
		if _, ok := values[field.Key]; ok {
			continue
		}
		values[field.Key] = field.Default
		if values[field.Key] == nil {
			values[field.Key] = ""
		}
	}
	return values
}

func pluginTemplate(runtime string, fields []types.PluginUserConfigField) string {
	if runtime == "nodejs" {
		return nodePluginTemplate(fields)
	}
	return pythonPluginTemplate(fields)
}

func nodePluginTemplate(fields []types.PluginUserConfigField) string {
	lines := []string{
		"const path = require('path');",
		"",
		"const sdkPath = path.join(__dirname, '../../sdk/nodejs');",
		"const { runDirect } = require(path.join(sdkPath, 'allbot_direct'));",
		"",
		"async function handle(ctx) {",
	}
	if len(fields) == 0 {
		lines = append(lines, "    await ctx.reply('插件已运行');")
	} else {
		for _, field := range fields {
			lines = append(lines, fmt.Sprintf("    const %s = String(ctx.config('%s', %s)).trim();", configVariableName(field.Key), field.Key, jsLiteral(field.Default)))
		}
		lines = append(lines, "    await ctx.reply('插件已运行');")
	}
	lines = append(lines, "}", "", "runDirect(handle);", "")
	return strings.Join(lines, "\n")
}

func accountQLPluginTemplate(options accountQLTemplate) string {
	if options.Runtime == "python" {
		return pythonAccountQLPluginTemplate(options)
	}
	return nodeAccountQLPluginTemplate(options)
}

func nodeAccountQLPluginTemplate(options accountQLTemplate) string {
	lines := []string{
		"const { createAccountQLPlugin, builtinPointsAuth } = require('../../sdk/nodejs/account_ql_plugin');",
		"",
		fmt.Sprintf("const ENV_NAME = %s;", jsLiteral(options.EnvName)),
		"",
		options.ParseInputCode,
		"",
		options.QueryCode,
	}
	if options.EnableCKCheck {
		lines = append(lines, "", options.CheckCKCode)
	}
	for _, route := range options.Routes {
		lines = append(lines, "", route.Code)
	}
	lines = append(lines,
		"",
		"createAccountQLPlugin({",
		fmt.Sprintf("  prefix: %s,", jsLiteral(options.Prefix)),
		fmt.Sprintf("  tableName: %s,", jsLiteral(options.TableName)),
		"  envName: ENV_NAME,",
		fmt.Sprintf("  loginPrompt: %s,", jsLiteral(fmt.Sprintf("请发送%s账号 CK，回复 q 退出：", options.Prefix))),
	)
	if len(options.Routes) > 0 {
		lines = append(lines, "  routes: {")
		for index, route := range options.Routes {
			comma := ","
			if index == len(options.Routes)-1 {
				comma = ""
			}
			lines = append(lines, fmt.Sprintf("    %s: %s%s", jsLiteral(route.Command), route.FunctionName, comma))
		}
		lines = append(lines, "  },")
	}
	lines = append(lines,
		"  account: {",
		"    parseInput,",
	)
	queryLine := "    query"
	if options.EnableCKCheck {
		queryLine += ","
	}
	lines = append(lines, queryLine)
	if options.EnableCKCheck {
		lines = append(lines, "    checkCk")
	}
	lines = append(lines,
		"  },",
		"  auth: { provider: builtinPointsAuth({ priceConfig: 'auth_price_per_month' }) },",
		"  ql: {",
		fmt.Sprintf("    runtime: %s,", jsLiteral(options.ScriptRuntime)),
		"    runtimeConfig: 'script_runtime',",
		fmt.Sprintf("    script: %s,", jsLiteral(options.TaskScript)),
		"    scriptConfig: 'task_script',",
		"    timeoutConfig: 'run_wait_timeout',",
		"    env: (ctx, accounts) => ({ [ENV_NAME]: accounts.map((item) => item.env_value).join('\\n') })",
		"  },",
		"  schedules: {",
		fmt.Sprintf("    run: { taskKey: %s, name: %s, cronConfig: 'cron', cron: %s, content: %s }%s", jsLiteral(options.PluginID+"-default-run"), jsLiteral(options.PluginName+"自动运行"), jsLiteral(options.Cron), jsLiteral(options.Prefix+"一键运行"), nodeScheduleComma(options, "run")),
	)
	if options.EnableExpireCheck {
		lines = append(lines, fmt.Sprintf("    expireCheck: { taskKey: %s, name: %s, cronConfig: 'expire_check_cron', cron: %s, content: %s }%s", jsLiteral(options.PluginID+"-expiration-check"), jsLiteral(options.PluginName+"过期检测"), jsLiteral(options.ExpireCheckCron), jsLiteral(options.Prefix+"过期检测"), nodeScheduleComma(options, "expireCheck")))
	}
	if options.EnableCKCheck {
		lines = append(lines, fmt.Sprintf("    ckCheck: { taskKey: %s, name: %s, cronConfig: 'ck_check_cron', cron: %s, content: %s }", jsLiteral(options.PluginID+"-ck-check"), jsLiteral(options.PluginName+" CK 检测"), jsLiteral(options.CKCheckCron), jsLiteral(options.Prefix+"CK检测")))
	}
	lines = append(lines, "  }", "});", "")
	return strings.Join(lines, "\n")
}

func nodeScheduleComma(options accountQLTemplate, key string) string {
	if key == "run" && (options.EnableExpireCheck || options.EnableCKCheck) {
		return ","
	}
	if key == "expireCheck" && options.EnableCKCheck {
		return ","
	}
	return ""
}

func pythonAccountQLPluginTemplate(options accountQLTemplate) string {
	lines := []string{
		"import os",
		"import sys",
		"",
		"sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), \"../../sdk/python\")))",
		"",
		"from account_ql_plugin import builtin_points_auth, create_account_ql_plugin",
		"",
		"",
		fmt.Sprintf("ENV_NAME = %s", pythonLiteral(options.EnvName)),
		"",
		"",
		options.ParseInputCode,
		"",
		"",
		options.QueryCode,
	}
	if options.EnableCKCheck {
		lines = append(lines, "", "", options.CheckCKCode)
	}
	for _, route := range options.Routes {
		lines = append(lines, "", "", route.Code)
	}
	lines = append(lines,
		"",
		"",
		"create_account_ql_plugin({",
		fmt.Sprintf("    \"prefix\": %s,", pythonLiteral(options.Prefix)),
		fmt.Sprintf("    \"table_name\": %s,", pythonLiteral(options.TableName)),
		"    \"env_name\": ENV_NAME,",
		fmt.Sprintf("    \"login_prompt\": %s,", pythonLiteral(fmt.Sprintf("请发送%s账号 CK，回复 q 退出：", options.Prefix))),
	)
	if len(options.Routes) > 0 {
		lines = append(lines, "    \"routes\": {")
		for _, route := range options.Routes {
			lines = append(lines, fmt.Sprintf("        %s: %s,", pythonLiteral(route.Command), route.FunctionName))
		}
		lines = append(lines, "    },")
	}
	lines = append(lines,
		"    \"account\": {",
		"        \"parse_input\": parse_input,",
		"        \"query\": query,",
	)
	if options.EnableCKCheck {
		lines = append(lines, "        \"check_ck\": check_ck,")
	}
	lines = append(lines,
		"    },",
		"    \"auth\": {\"provider\": builtin_points_auth(\"auth_price_per_month\")},",
		"    \"ql\": {",
		fmt.Sprintf("        \"runtime\": %s,", pythonLiteral(options.ScriptRuntime)),
		"        \"runtime_config\": \"script_runtime\",",
		fmt.Sprintf("        \"script\": %s,", pythonLiteral(options.TaskScript)),
		"        \"script_config\": \"task_script\",",
		"        \"timeout_config\": \"run_wait_timeout\",",
		"        \"env\": {},",
		"    },",
		"    \"schedules\": {",
		fmt.Sprintf("        \"run\": {\"task_key\": %s, \"name\": %s, \"cron_config\": \"cron\", \"cron\": %s, \"content\": %s},", pythonLiteral(options.PluginID+"-default-run"), pythonLiteral(options.PluginName+"自动运行"), pythonLiteral(options.Cron), pythonLiteral(options.Prefix+"一键运行")),
	)
	if options.EnableExpireCheck {
		lines = append(lines, fmt.Sprintf("        \"expire_check\": {\"task_key\": %s, \"name\": %s, \"cron_config\": \"expire_check_cron\", \"cron\": %s, \"content\": %s},", pythonLiteral(options.PluginID+"-expiration-check"), pythonLiteral(options.PluginName+"过期检测"), pythonLiteral(options.ExpireCheckCron), pythonLiteral(options.Prefix+"过期检测")))
	}
	if options.EnableCKCheck {
		lines = append(lines, fmt.Sprintf("        \"ck_check\": {\"task_key\": %s, \"name\": %s, \"cron_config\": \"ck_check_cron\", \"cron\": %s, \"content\": %s},", pythonLiteral(options.PluginID+"-ck-check"), pythonLiteral(options.PluginName+" CK 检测"), pythonLiteral(options.CKCheckCron), pythonLiteral(options.Prefix+"CK检测")))
	}
	lines = append(lines, "    },", "})", "")
	return strings.Join(lines, "\n")
}

func accountQLTaskScriptTemplate(runtime string, envName string) string {
	if runtime == "python" {
		lines := []string{
			"import os",
			"",
			fmt.Sprintf("env_name = %s", pythonLiteral(envName)),
			"accounts = [item for item in str(os.environ.get(env_name, '')).split('\\n') if item]",
			"print(f'{env_name} 账号数量：{len(accounts)}')",
			"",
		}
		return strings.Join(lines, "\n")
	}
	lines := []string{
		fmt.Sprintf("const envName = %s;", jsLiteral(envName)),
		"const accounts = String(process.env[envName] || '').split('\\n').filter(Boolean);",
		"console.log(`${envName} 账号数量：${accounts.length}`);",
		"",
	}
	return strings.Join(lines, "\n")
}

func nodeAccountQLTaskScriptTemplate(envName string) string {
	return accountQLTaskScriptTemplate("nodejs", envName)
}

func pythonPluginTemplate(fields []types.PluginUserConfigField) string {
	lines := []string{
		"import os",
		"import sys",
		"",
		"sdk_path = os.path.join(os.path.dirname(__file__), '../../sdk/python')",
		"sys.path.insert(0, sdk_path)",
		"",
		"from allbot_direct import run_direct",
		"",
		"",
		"async def handle(ctx):",
	}
	if len(fields) == 0 {
		lines = append(lines, "    await ctx.reply('插件已运行')")
	} else {
		for _, field := range fields {
			lines = append(lines, fmt.Sprintf("    %s = str(ctx.config('%s', %s)).strip()", configVariableName(field.Key), field.Key, pythonLiteral(field.Default)))
		}
		lines = append(lines, "    await ctx.reply('插件已运行')")
	}
	lines = append(lines, "", "", "if __name__ == '__main__':", "    run_direct(handle)", "")
	return strings.Join(lines, "\n")
}

func sanitizePluginID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = pluginIDSanitizePattern.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_-")
	return value
}

func sanitizeConfigKey(value string) string {
	value = strings.TrimSpace(value)
	value = configKeySanitizePattern.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return ""
	}
	if value[0] >= '0' && value[0] <= '9' {
		value = "config_" + value
	}
	return value
}

func configVariableName(key string) string {
	key = sanitizeConfigKey(key)
	if key == "" {
		return "config_value"
	}
	if reservedConfigVariableNames[key] {
		return "config_" + key
	}
	return key
}

var reservedConfigVariableNames = map[string]bool{
	"and": true, "as": true, "assert": true, "async": true, "await": true, "break": true,
	"case": true, "catch": true, "class": true, "const": true, "continue": true, "debugger": true,
	"def": true, "default": true, "del": true, "delete": true, "do": true, "elif": true,
	"else": true, "enum": true, "except": true, "export": true, "extends": true, "false": true,
	"finally": true, "for": true, "from": true, "function": true, "global": true, "if": true,
	"implements": true, "import": true, "in": true, "instanceof": true, "interface": true, "is": true,
	"lambda": true, "let": true, "new": true, "none": true, "nonlocal": true, "not": true,
	"or": true, "package": true, "pass": true, "private": true, "protected": true, "public": true,
	"raise": true, "return": true, "static": true, "super": true, "switch": true, "this": true,
	"throw": true, "true": true, "try": true, "typeof": true, "var": true, "void": true,
	"while": true, "with": true, "yield": true,
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func jsLiteral(value interface{}) string {
	data, _ := json.Marshal(value)
	if len(data) == 0 || string(data) == "null" {
		return "''"
	}
	return string(data)
}

func pythonLiteral(value interface{}) string {
	data, _ := json.Marshal(value)
	if len(data) == 0 || string(data) == "null" {
		return "''"
	}
	switch string(data) {
	case "true":
		return "True"
	case "false":
		return "False"
	}
	return string(data)
}

func uniquePluginID(prefix string) string {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 1000; i++ {
		candidate := fmt.Sprintf("%s_%d_%04d", prefix, time.Now().Unix(), rand.Intn(10000))
		if _, err := os.Stat(filepath.Join("plugins", candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

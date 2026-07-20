package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

const (
	accountQLTemplateSourceVersion = 1
	hybridTemplateSourceVersion    = 2
	hybridTemplateSourceMode       = "hybrid"
)

type accountQLTemplateSource struct {
	Version           int                     `json:"version"`
	Mode              string                  `json:"mode,omitempty"`
	Template          string                  `json:"template,omitempty"`
	Plugin            accountQLPluginSource   `json:"plugin"`
	Account           createAccountQLRequest  `json:"account_ql,omitempty"`
	Files             []templateSourceFile    `json:"files"`
	Sections          []templateSourceSection `json:"sections,omitempty"`
	TaskScripts       interface{}             `json:"task_scripts,omitempty"`
	ReferenceExisting interface{}             `json:"reference_existing,omitempty"`
	Compatibility     interface{}             `json:"compatibility,omitempty"`
	Migration         interface{}             `json:"migration,omitempty"`
}

type accountQLPluginSource struct {
	ID             string                `json:"id,omitempty"`
	Template       string                `json:"template,omitempty"`
	Name           string                `json:"name"`
	Version        string                `json:"version"`
	Runtime        string                `json:"runtime"`
	RuntimeProfile string                `json:"runtime_profile"`
	Priority       int                   `json:"priority"`
	Platforms      []string              `json:"platforms"`
	Enabled        bool                  `json:"enabled"`
	ScriptEnv      types.ScriptEnvConfig `json:"script_env"`
}

type templateSourceFile struct {
	Path           string `json:"path"`
	Role           string `json:"role"`
	Ownership      string `json:"ownership,omitempty"`
	SHA256         string `json:"sha256"`
	ReadOnlyReason string `json:"read_only_reason,omitempty"`
}

type templateSourceSection struct {
	ID             string `json:"id"`
	Category       string `json:"category"`
	Label          string `json:"label"`
	Path           string `json:"path"`
	Content        string `json:"content"`
	Ownership      string `json:"ownership,omitempty"`
	ReadOnlyReason string `json:"read_only_reason,omitempty"`
}

type templateEditorRequest struct {
	TemplateSource          *accountQLTemplateSource `json:"template_source"`
	ConvertLegacy           bool                     `json:"convert_legacy"`
	OverwriteGeneratedFiles bool                     `json:"overwrite_generated_files"`
	Force                   bool                     `json:"force"`
}

type templateEditorState struct {
	PluginID         string                   `json:"plugin_id"`
	Version          int                      `json:"version,omitempty"`
	Mode             string                   `json:"mode,omitempty"`
	Editable         bool                     `json:"editable"`
	RequiresConvert  bool                     `json:"requires_conversion"`
	Reason           string                   `json:"reason,omitempty"`
	Template         string                   `json:"template,omitempty"`
	TemplateVersion  string                   `json:"template_version,omitempty"`
	TemplateSource   *accountQLTemplateSource `json:"template_source,omitempty"`
	ConversionSource *accountQLTemplateSource `json:"conversion_source,omitempty"`
	Files            []templateSourceFile     `json:"files,omitempty"`
	ModifiedFiles    []string                 `json:"modified_files"`
	SourceChanged    bool                     `json:"source_changed"`
}

type templateFileSnapshot struct {
	Path    string
	Existed bool
	Mode    os.FileMode
	Content []byte
}

func (s *Server) pluginMutationLock(pluginID string) *sync.Mutex {
	value, _ := s.pluginMutationLocks.LoadOrStore(pluginID, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func (s *Server) handlePluginTemplateEditor(w http.ResponseWriter, r *http.Request) {
	pluginID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/plugins/template-editor/"), "/")
	if pluginID == "" || strings.ContainsAny(pluginID, `/\\`) || pluginID == "." || strings.HasPrefix(pluginID, "..") {
		s.jsonError(w, "插件 ID 无效", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		state, err := s.loadTemplateEditorState(pluginID)
		if err != nil {
			s.writeTemplateEditorError(w, err)
			return
		}
		s.jsonResponse(w, state)
	case http.MethodPut:
		var req templateEditorRequest
		r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			s.jsonError(w, "模板编辑请求无效: "+err.Error(), http.StatusBadRequest)
			return
		}
		state, err := s.updateTemplateEditor(pluginID, req)
		if err != nil {
			s.writeTemplateEditorError(w, err)
			return
		}
		s.jsonResponse(w, state)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) writeTemplateEditorError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case strings.Contains(err.Error(), "已被修改"), strings.Contains(err.Error(), "拒绝覆盖"), strings.Contains(err.Error(), "基线漂移"), strings.Contains(err.Error(), "无法唯一定位"), strings.Contains(err.Error(), "section update"), strings.Contains(err.Error(), "原 content"), strings.Contains(err.Error(), "新 content"), strings.Contains(err.Error(), "在原文件中重叠"), strings.Contains(err.Error(), "在新文件中重叠"):
		status = http.StatusConflict
	case os.IsNotExist(err):
		status = http.StatusNotFound
	case strings.Contains(err.Error(), "不存在"):
		status = http.StatusNotFound
	case strings.Contains(err.Error(), "不可编辑"), strings.Contains(err.Error(), "转换"), strings.Contains(err.Error(), "无效"), strings.Contains(err.Error(), "不能为空"), strings.Contains(err.Error(), "必须"), strings.Contains(err.Error(), "不允许"), strings.Contains(err.Error(), "未登记"), strings.Contains(err.Error(), "冲突"), strings.Contains(err.Error(), "重复"):
		status = http.StatusBadRequest
	}
	s.jsonError(w, err.Error(), status)
}

func (s *Server) loadTemplateEditorState(pluginID string) (*templateEditorState, error) {
	pluginRoot := filepath.Join("plugins", pluginID)
	configValue, err := readPluginConfig(pluginRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("插件不存在: %s", pluginID)
		}
		return nil, err
	}
	state := &templateEditorState{
		PluginID:        pluginID,
		Template:        strings.TrimSpace(configValue.Template),
		TemplateVersion: strings.TrimSpace(configValue.TemplateVersion),
		ModifiedFiles:   []string{},
	}
	stored, err := s.loadStoredTemplateSource(pluginID, configValue)
	if err != nil {
		return nil, err
	}
	if stored != nil && stored.Version == hybridTemplateSourceVersion {
		if err := validateHybridTemplateSourceIdentity(pluginID, configValue, stored); err != nil {
			return nil, err
		}
		metadata, err := s.currentTemplateMetadata(pluginID)
		if err != nil {
			return nil, err
		}
		registration, err := resolveHybridTemplateRegistration(configValue, stored, metadata)
		if err != nil {
			return nil, err
		}
		state.Template = registration.Template
		state.TemplateVersion = registration.TemplateVersion
		state.Version = hybridTemplateSourceVersion
		state.Mode = hybridTemplateSourceMode
		state.Editable = true
		state.TemplateSource = stored
		state.Files = append([]templateSourceFile(nil), stored.Files...)
		state.ModifiedFiles, err = modifiedTemplateFiles(pluginRoot, stored.Files)
		if err != nil {
			return nil, err
		}
		state.SourceChanged = len(state.ModifiedFiles) > 0
		if state.SourceChanged {
			state.Reason = "模板登记文件已在编辑器之外修改"
		}
		return state, nil
	}
	if !isAccountQLTemplate(configValue.Template) {
		state.Reason = "仅账号模板插件支持分类编辑"
		return state, nil
	}
	if stored == nil {
		state.RequiresConvert = true
		state.Reason = "旧账号插件需要显式转换后才能分类编辑"
		state.ConversionSource, err = s.legacyAccountQLConversionSource(pluginID, configValue)
		if err != nil {
			return nil, err
		}
		return state, nil
	}
	state.Editable = true
	state.TemplateSource = stored
	state.Files = append([]templateSourceFile(nil), stored.Files...)
	state.ModifiedFiles, err = modifiedTemplateFiles(pluginRoot, stored.Files)
	if err != nil {
		return nil, err
	}
	state.SourceChanged = len(state.ModifiedFiles) > 0
	if state.SourceChanged {
		state.Reason = "生成文件已在模板编辑器之外修改，保存前需要确认覆盖"
	}
	return state, nil
}

func (s *Server) updateTemplateEditor(pluginID string, req templateEditorRequest) (*templateEditorState, error) {
	mutationLock := s.pluginMutationLock(pluginID)
	mutationLock.Lock()
	defer mutationLock.Unlock()

	pluginRoot := filepath.Join("plugins", pluginID)
	currentConfig, err := readPluginConfig(pluginRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("插件不存在: %s", pluginID)
		}
		return nil, err
	}
	stored, err := s.loadStoredTemplateSource(pluginID, currentConfig)
	if err != nil {
		return nil, err
	}
	if stored != nil && stored.Version == hybridTemplateSourceVersion {
		if req.ConvertLegacy {
			return nil, fmt.Errorf("v2 hybrid 插件不允许转换")
		}
		if req.TemplateSource == nil {
			return nil, fmt.Errorf("template_source 不能为空")
		}
		return s.updateHybridTemplateEditor(pluginID, currentConfig, req.TemplateSource, stored)
	}
	if !isAccountQLTemplate(currentConfig.Template) {
		return nil, fmt.Errorf("插件不是可转换的账号模板插件")
	}
	if stored == nil {
		if !req.ConvertLegacy {
			return nil, fmt.Errorf("旧账号插件必须通过 convert_legacy 显式转换")
		}
		if req.TemplateSource == nil {
			return nil, fmt.Errorf("旧账号插件转换必须提交完整 template_source")
		}
	} else if req.ConvertLegacy {
		return nil, fmt.Errorf("插件已完成转换，无需再次 convert_legacy")
	}
	if stored != nil {
		modified, err := modifiedTemplateFiles(pluginRoot, stored.Files)
		if err != nil {
			return nil, err
		}
		if len(modified) > 0 && !req.OverwriteGeneratedFiles && !req.Force {
			return nil, fmt.Errorf("生成文件已被修改，拒绝覆盖: %s", strings.Join(modified, ", "))
		}
	}

	source := req.TemplateSource
	if source == nil {
		return nil, fmt.Errorf("template_source 不能为空")
	}
	plan, normalizedSource, err := buildPlanFromTemplateSource(pluginID, *source)
	if err != nil {
		return nil, err
	}
	preserveUnmanagedPluginConfig(&plan.Config, currentConfig)
	files := renderCreatePluginFiles(plan)
	normalizedSource, err = decodeTemplateSource(plan.Config.TemplateSource)
	if err != nil {
		return nil, err
	}
	if stored == nil && !req.OverwriteGeneratedFiles && !req.Force {
		return nil, fmt.Errorf("旧账号插件转换必须明确允许覆盖生成文件")
	}
	unexpectedTargets, err := unexpectedGeneratedTargets(pluginRoot, files, stored)
	if err != nil {
		return nil, err
	}
	if len(unexpectedTargets) > 0 && !req.OverwriteGeneratedFiles && !req.Force {
		return nil, fmt.Errorf("生成目标已存在，拒绝覆盖: %s", strings.Join(unexpectedTargets, ", "))
	}

	paths := generatedFilePaths(files)
	if stored != nil {
		paths = append(paths, templateFilePaths(stored.Files)...)
	}
	snapshots, err := snapshotTemplateFiles(pluginRoot, uniqueStrings(paths))
	if err != nil {
		return nil, err
	}
	oldMetadata, err := s.currentTemplateMetadata(pluginID)
	if err != nil {
		return nil, err
	}
	if err := writeTemplateFilesAtomically(pluginRoot, files); err != nil {
		return nil, errors.Join(err, wrapTemplateRestoreError("文件", restoreTemplateFiles(pluginRoot, snapshots)))
	}
	if err := s.savePlanTemplateMetadata(plan); err != nil {
		return nil, errors.Join(
			err,
			wrapTemplateRestoreError("文件", restoreTemplateFiles(pluginRoot, snapshots)),
			wrapTemplateRestoreError("模板元数据", s.restoreTemplateMetadata(pluginID, oldMetadata)),
		)
	}
	if err := s.reloadAndRegisterPlugin(pluginID); err != nil {
		restoreErr := errors.Join(
			wrapTemplateRestoreError("文件", restoreTemplateFiles(pluginRoot, snapshots)),
			wrapTemplateRestoreError("模板元数据", s.restoreTemplateMetadata(pluginID, oldMetadata)),
			wrapTemplateRestoreError("插件运行状态", s.reloadAndRegisterPlugin(pluginID)),
		)
		if restoreErr != nil {
			return nil, errors.Join(fmt.Errorf("模板更新后重载失败: %w", err), restoreErr)
		}
		return nil, fmt.Errorf("模板更新后重载失败，已恢复原状态: %w", err)
	}
	removeStaleTemplateFiles(pluginRoot, stored, normalizedSource.Files)
	return s.loadTemplateEditorState(pluginID)
}

func buildPlanFromTemplateSource(pluginID string, source accountQLTemplateSource) (*createPluginPlan, *accountQLTemplateSource, error) {
	if source.Version != accountQLTemplateSourceVersion {
		return nil, nil, fmt.Errorf("template_source 版本无效: %d", source.Version)
	}
	if !isAccountQLTemplate(source.Template) {
		return nil, nil, fmt.Errorf("template_source 模板无效")
	}
	req := createPluginRequest{
		ID:             pluginID,
		Name:           source.Plugin.Name,
		Version:        source.Plugin.Version,
		Runtime:        source.Plugin.Runtime,
		RuntimeProfile: source.Plugin.RuntimeProfile,
		Priority:       source.Plugin.Priority,
		Platforms:      append([]string(nil), source.Plugin.Platforms...),
		Enabled:        source.Plugin.Enabled,
		ScriptEnv:      source.Plugin.ScriptEnv,
		Template:       source.Template,
		AccountQL:      &source.Account,
	}
	plan, err := buildCreatePluginPlan(req)
	if err != nil {
		return nil, nil, err
	}
	if issues := validateCreatePluginPlan(plan); len(issues) > 0 {
		return nil, nil, fmt.Errorf("模板配置无效: %s", issues[0].Message)
	}
	normalized := buildTemplateSource(plan, nil)
	return plan, normalized, nil
}

func preserveUnmanagedPluginConfig(target *types.PluginConfig, current types.PluginConfig) {
	if target == nil {
		return
	}
	target.AllowedAdapterIDs = append([]string(nil), current.AllowedAdapterIDs...)
	target.Pinned = current.Pinned
	target.Dependencies = cloneStringMap(current.Dependencies)
	target.AccessControl = cloneAccessControl(current.AccessControl)
	target.OpenAPI = current.OpenAPI
	target.WebUI = current.WebUI
	target.WebChat = current.WebChat

	managedFields := accountQLUserConfigSchema(accountQLTemplate{EnableCKCheck: true, EnableExpireCheck: true})
	managedKeys := make(map[string]bool, len(managedFields))
	for _, field := range managedFields {
		managedKeys[field.Key] = true
	}
	for _, field := range current.UserConfigSchema {
		if !managedKeys[field.Key] {
			target.UserConfigSchema = append(target.UserConfigSchema, field)
		}
	}
	if target.UserConfig == nil {
		target.UserConfig = map[string]interface{}{}
	}
	for key, value := range current.UserConfig {
		if !managedKeys[key] {
			target.UserConfig[key] = value
		}
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneAccessControl(source *types.AccessControlConfig) *types.AccessControlConfig {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.WhitelistGroups = append([]string(nil), source.WhitelistGroups...)
	copyValue.BlockedGroups = append([]string(nil), source.BlockedGroups...)
	copyValue.WhitelistUserIDs = append([]string(nil), source.WhitelistUserIDs...)
	copyValue.BlockedUserIDs = append([]string(nil), source.BlockedUserIDs...)
	copyValue.WhitelistUnionIDs = append([]string(nil), source.WhitelistUnionIDs...)
	copyValue.BlockedUnionIDs = append([]string(nil), source.BlockedUnionIDs...)
	return &copyValue
}

func buildTemplateSource(plan *createPluginPlan, files []createGeneratedFile) *accountQLTemplateSource {
	if plan == nil || plan.AccountQL == nil {
		return nil
	}
	afterRun := plan.AccountQL.EnableAfterRun
	waitScheduled := plan.AccountQL.WaitScheduled
	enableCK := plan.AccountQL.EnableCKCheck
	enableExpire := plan.AccountQL.EnableExpireCheck
	expireDelete := plan.AccountQL.ExpireDeleteAfterDays
	routes := make([]createAccountQLRouteRequest, 0, len(plan.AccountQL.Routes))
	for _, route := range plan.AccountQL.Routes {
		routes = append(routes, createAccountQLRouteRequest{Command: route.Command, FunctionName: route.FunctionName, Description: route.Description, Code: route.Code})
	}
	source := &accountQLTemplateSource{
		Version:  accountQLTemplateSourceVersion,
		Template: plan.Template,
		Plugin: accountQLPluginSource{
			Name: plan.Config.Name, Version: plan.Config.Version, Runtime: plan.Runtime,
			RuntimeProfile: plan.Config.RuntimeProfile, Priority: plan.Config.Priority,
			Platforms: append([]string(nil), plan.Config.Platforms...), Enabled: plan.Config.Enabled,
			ScriptEnv: plan.Config.ScriptEnv,
		},
		Account: createAccountQLRequest{
			Prefix: plan.AccountQL.Prefix, TableName: plan.AccountQL.TableName, EnvName: plan.AccountQL.EnvName,
			TaskScript: plan.AccountQL.TaskScript, ScriptRuntime: plan.AccountQL.ScriptRuntime,
			AuthPricePerMonth: plan.AccountQL.AuthPricePerMonth, Cron: plan.AccountQL.Cron,
			CKCheckCron: plan.AccountQL.CKCheckCron, RunWaitTimeout: plan.AccountQL.RunWaitTimeout,
			WaitScheduled: &waitScheduled, EnableAfterRun: &afterRun, AfterRunCode: plan.AccountQL.AfterRunCode,
			ParseInputCode: plan.AccountQL.ParseInputCode, QueryCode: plan.AccountQL.QueryCode,
			EnableCKCheck: &enableCK, CheckCKCode: plan.AccountQL.CheckCKCode,
			EnableExpireCheck: &enableExpire, ExpireCheckCron: plan.AccountQL.ExpireCheckCron,
			ExpireNotifyDays: plan.AccountQL.ExpireNotifyDays, ExpireDeleteAfterDays: &expireDelete, Routes: routes,
		},
		Files: hashGeneratedTemplateFiles(files),
	}
	return source
}

func (s *Server) legacyAccountQLConversionSource(pluginID string, configValue types.PluginConfig) (*accountQLTemplateSource, error) {
	metadata := configValue.TemplateMetadata
	stored, err := s.currentTemplateMetadata(pluginID)
	if err != nil {
		return nil, err
	}
	if len(metadata) == 0 && stored != nil {
		metadata = stored.Metadata
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	account := createAccountQLRequest{
		Prefix: stringMapValue(metadata, "prefix"), TableName: stringMapValue(metadata, "table_name"),
		EnvName: stringMapValue(metadata, "env_name"), TaskScript: stringMapValue(metadata, "task_script"),
		ScriptRuntime: stringMapValue(metadata, "script_runtime"), Cron: stringInterfaceValue(configValue.UserConfig, "cron"),
		CKCheckCron: stringInterfaceValue(configValue.UserConfig, "ck_check_cron"), ExpireCheckCron: stringInterfaceValue(configValue.UserConfig, "expire_check_cron"),
		ExpireNotifyDays: stringInterfaceValue(configValue.UserConfig, "expire_notify_days"), AuthPricePerMonth: intInterfaceValue(configValue.UserConfig, "auth_price_per_month"),
		RunWaitTimeout: intInterfaceValue(configValue.UserConfig, "run_wait_timeout"), Routes: legacyRouteSummaries(metadata),
	}
	account.WaitScheduled = boolMapPointer(metadata, "wait_scheduled", true)
	account.EnableAfterRun = boolMapPointer(metadata, "enable_after_run", false)
	account.EnableCKCheck = boolMapPointer(metadata, "enable_ck_check", true)
	account.EnableExpireCheck = boolMapPointer(metadata, "enable_expire_check", false)
	expireDelete := intInterfaceValue(configValue.UserConfig, "expire_delete_after_days")
	if _, ok := configValue.UserConfig["expire_delete_after_days"]; !ok {
		expireDelete = -1
	}
	account.ExpireDeleteAfterDays = &expireDelete
	if account.ScriptRuntime == "" {
		account.ScriptRuntime = configValue.Runtime
	}
	if account.TaskScript == "" {
		account.TaskScript = stringInterfaceValue(configValue.UserConfig, "task_script")
	}
	return &accountQLTemplateSource{
		Version: accountQLTemplateSourceVersion, Template: configValue.Template,
		Plugin:  accountQLPluginSource{Name: configValue.Name, Version: configValue.Version, Runtime: configValue.Runtime, RuntimeProfile: configValue.RuntimeProfile, Priority: configValue.Priority, Platforms: append([]string(nil), configValue.Platforms...), Enabled: configValue.Enabled, ScriptEnv: configValue.ScriptEnv},
		Account: account, Files: []templateSourceFile{},
	}, nil
}

func legacyRouteSummaries(metadata map[string]interface{}) []createAccountQLRouteRequest {
	data, err := json.Marshal(metadata["routes"])
	if err != nil {
		return []createAccountQLRouteRequest{}
	}
	var routes []createAccountQLRouteRequest
	if json.Unmarshal(data, &routes) != nil {
		return []createAccountQLRouteRequest{}
	}
	for index := range routes {
		routes[index].Code = ""
	}
	return routes
}

func (s *Server) loadStoredTemplateSource(pluginID string, configValue types.PluginConfig) (*accountQLTemplateSource, error) {
	if len(configValue.TemplateSource) > 0 {
		return decodeTemplateSource(configValue.TemplateSource)
	}
	metadata, err := s.currentTemplateMetadata(pluginID)
	if err != nil {
		return nil, err
	}
	if metadata == nil || len(metadata.TemplateSource) == 0 {
		return nil, nil
	}
	return decodeTemplateSource(metadata.TemplateSource)
}

func decodeTemplateSource(value map[string]interface{}) (*accountQLTemplateSource, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var source accountQLTemplateSource
	if err := json.Unmarshal(data, &source); err != nil {
		return nil, fmt.Errorf("template_source 无效: %w", err)
	}
	if source.Version == hybridTemplateSourceVersion {
		if strings.ToLower(strings.TrimSpace(source.Mode)) != hybridTemplateSourceMode {
			return nil, fmt.Errorf("template_source v2 必须使用 mode=hybrid")
		}
		return &source, nil
	}
	if source.Version != accountQLTemplateSourceVersion || !isAccountQLTemplate(source.Template) {
		return nil, fmt.Errorf("template_source 无效")
	}
	return &source, nil
}

func templateSourceMap(source *accountQLTemplateSource) map[string]interface{} {
	if source == nil {
		return nil
	}
	data, _ := json.Marshal(source)
	var value map[string]interface{}
	_ = json.Unmarshal(data, &value)
	return value
}

func hashGeneratedTemplateFiles(files []createGeneratedFile) []templateSourceFile {
	result := make([]templateSourceFile, 0, len(files))
	for _, file := range files {
		// plugin.json 内嵌 template_source，无法稳定保存自身摘要；配置文件由重载校验保护。
		if file.Role == "config" {
			continue
		}
		result = append(result, templateSourceFile{Path: filepath.ToSlash(file.Path), Role: file.Role, SHA256: sha256Text(file.Content)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

func modifiedTemplateFiles(pluginRoot string, files []templateSourceFile) ([]string, error) {
	modified := make([]string, 0)
	for _, file := range files {
		fullPath, err := safeTemplateFilePath(pluginRoot, file.Path)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				modified = append(modified, file.Path)
				continue
			}
			return nil, err
		}
		actualSHA256 := sha256Bytes(data)
		if file.Role == "config" {
			var configValue types.PluginConfig
			if json.Unmarshal(data, &configValue) != nil {
				modified = append(modified, file.Path)
				continue
			}
			actualSHA256 = pluginConfigSHA256(configValue)
		}
		if actualSHA256 != file.SHA256 {
			modified = append(modified, file.Path)
		}
	}
	sort.Strings(modified)
	return modified, nil
}

func unexpectedGeneratedTargets(pluginRoot string, files []createGeneratedFile, previous *accountQLTemplateSource) ([]string, error) {
	managed := map[string]bool{"plugin.json": true}
	if previous != nil {
		for _, file := range previous.Files {
			managed[filepath.ToSlash(file.Path)] = true
		}
	}
	unexpected := make([]string, 0)
	for _, file := range files {
		pathValue := filepath.ToSlash(file.Path)
		if managed[pathValue] {
			continue
		}
		fullPath, err := safeTemplateFilePath(pluginRoot, pathValue)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(fullPath); err == nil {
			unexpected = append(unexpected, pathValue)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	sort.Strings(unexpected)
	return unexpected, nil
}

func writeTemplateFilesAtomically(pluginRoot string, files []createGeneratedFile) error {
	type stagedFile struct{ target, temp string }
	staged := make([]stagedFile, 0, len(files))
	cleanup := func() {
		for _, item := range staged {
			_ = os.Remove(item.temp)
		}
	}
	for _, file := range files {
		target, err := safeTemplateFilePath(pluginRoot, file.Path)
		if err != nil {
			cleanup()
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			cleanup()
			return err
		}
		tempFile, err := os.CreateTemp(filepath.Dir(target), ".allbot-template-*")
		if err != nil {
			cleanup()
			return err
		}
		tempPath := tempFile.Name()
		if _, err = tempFile.Write([]byte(file.Content)); err == nil {
			err = tempFile.Sync()
		}
		closeErr := tempFile.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(tempPath)
			cleanup()
			return err
		}
		staged = append(staged, stagedFile{target: target, temp: tempPath})
	}
	for _, item := range staged {
		if err := replaceFile(item.temp, item.target); err != nil {
			cleanup()
			return err
		}
	}
	return nil
}

func replaceFile(tempPath, targetPath string) error {
	backupPath := targetPath + ".allbot-template-old"
	_ = os.Remove(backupPath)
	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, backupPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Rename(backupPath, targetPath)
		return err
	}
	_ = os.Remove(backupPath)
	return nil
}

func snapshotTemplateFiles(pluginRoot string, paths []string) ([]templateFileSnapshot, error) {
	result := make([]templateFileSnapshot, 0, len(paths))
	for _, name := range paths {
		fullPath, err := safeTemplateFilePath(pluginRoot, name)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			result = append(result, templateFileSnapshot{Path: name})
			continue
		}
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		result = append(result, templateFileSnapshot{Path: name, Existed: true, Mode: info.Mode(), Content: data})
	}
	return result, nil
}

func wrapTemplateRestoreError(target string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("恢复%s失败: %w", target, err)
}

func restoreTemplateFiles(pluginRoot string, snapshots []templateFileSnapshot) error {
	var restoreErr error
	for _, snapshot := range snapshots {
		fullPath, err := safeTemplateFilePath(pluginRoot, snapshot.Path)
		if err != nil {
			restoreErr = err
			continue
		}
		if !snapshot.Existed {
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				restoreErr = err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			restoreErr = err
			continue
		}
		if err := os.WriteFile(fullPath, snapshot.Content, snapshot.Mode.Perm()); err != nil {
			restoreErr = err
		}
	}
	return restoreErr
}

func removeStaleTemplateFiles(pluginRoot string, previous *accountQLTemplateSource, current []templateSourceFile) {
	if previous == nil {
		return
	}
	keep := map[string]bool{}
	for _, file := range current {
		keep[file.Path] = true
	}
	for _, file := range previous.Files {
		if file.Role == "config" || keep[file.Path] {
			continue
		}
		fullPath, err := safeTemplateFilePath(pluginRoot, file.Path)
		if err == nil {
			_ = os.Remove(fullPath)
		}
	}
}

func safeTemplateFilePath(root, relative string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relative)))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("模板文件路径无效: %s", relative)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(filepath.Join(rootAbs, clean))
	if err != nil {
		return "", err
	}
	if fullPath != rootAbs && !strings.HasPrefix(fullPath, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("模板文件路径无效: %s", relative)
	}
	return fullPath, nil
}

func readPluginConfig(pluginRoot string) (types.PluginConfig, error) {
	var value types.PluginConfig
	data, err := os.ReadFile(filepath.Join(pluginRoot, "plugin.json"))
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("plugin.json 无效: %w", err)
	}
	return value, nil
}

func (s *Server) reloadAndRegisterPlugin(pluginID string) error {
	if s.pluginManager == nil || s.router == nil {
		return fmt.Errorf("插件管理器不可用")
	}
	if err := s.pluginManager.ReloadPlugin(pluginID); err != nil {
		return err
	}
	process := s.pluginManager.GetPlugin(pluginID)
	if process == nil || process.Plugin == nil {
		return fmt.Errorf("插件重载后不存在")
	}
	return s.router.RegisterPlugin(process.Plugin)
}

func (s *Server) currentTemplateMetadata(pluginID string) (*config.PluginTemplateMetadata, error) {
	database := s.pluginTemplateMetadataDatabase()
	if database == nil {
		return nil, fmt.Errorf("数据库不可用，无法保存模板元数据")
	}
	return database.GetPluginTemplateMetadata(pluginID)
}

func (s *Server) restoreTemplateMetadata(pluginID string, previous *config.PluginTemplateMetadata) error {
	if previous == nil {
		return s.deletePlanTemplateMetadata(pluginID)
	}
	return s.pluginTemplateMetadataDatabase().SavePluginTemplateMetadata(previous)
}

func generatedFilePaths(files []createGeneratedFile) []string {
	result := make([]string, 0, len(files))
	for _, file := range files {
		result = append(result, file.Path)
	}
	return result
}

func templateFilePaths(files []templateSourceFile) []string {
	result := make([]string, 0, len(files))
	for _, file := range files {
		result = append(result, file.Path)
	}
	return result
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = filepath.ToSlash(strings.TrimSpace(value))
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func isAccountQLTemplate(value string) bool {
	return value == "nodejs_account_ql" || value == "python_account_ql"
}

func sha256Text(value string) string { return sha256Bytes([]byte(value)) }

func sha256Bytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func stringMapValue(value map[string]interface{}, key string) string {
	result, _ := value[key].(string)
	return strings.TrimSpace(result)
}

func stringInterfaceValue(value map[string]interface{}, key string) string {
	if value == nil {
		return ""
	}
	result, _ := value[key].(string)
	return strings.TrimSpace(result)
}

func intInterfaceValue(value map[string]interface{}, key string) int {
	if value == nil {
		return 0
	}
	switch item := value[key].(type) {
	case float64:
		return int(item)
	case int:
		return item
	case json.Number:
		parsed, _ := item.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func boolMapPointer(value map[string]interface{}, key string, fallback bool) *bool {
	result, ok := value[key].(bool)
	if !ok {
		result = fallback
	}
	return &result
}

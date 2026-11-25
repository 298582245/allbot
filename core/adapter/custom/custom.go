package custom

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_contract "github.com/allbot/allbot/core/adapter/_contract"
	_registry "github.com/allbot/allbot/core/adapter/_registry"
	"github.com/allbot/allbot/core/types"
)

const manifestName = "adapter.json"
const CallbackTokenField = "_allbot_callback_token"

var safeID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)
var runtimeMu sync.RWMutex
var nodeExecutable = "node"
var pythonExecutable = "python"
var runtimeResolver func(runtimeName, profileID string) (string, string, error)

func SetRuntimeExecutables(node, python string) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if strings.TrimSpace(node) != "" {
		nodeExecutable = node
	}
	if strings.TrimSpace(python) != "" {
		pythonExecutable = python
	}
}

func SetRuntimeResolver(resolve func(runtimeName, profileID string) (string, string, error)) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	runtimeResolver = resolve
}

func NewCallbackToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

type manifest struct {
	ID              string                  `json:"id"`
	Name            string                  `json:"name"`
	Version         string                  `json:"version"`
	Author          string                  `json:"author"`
	Description     string                  `json:"description"`
	Runtime         string                  `json:"runtime"`
	Entry           string                  `json:"entry"`
	ProtocolVersion string                  `json:"protocol_version"`
	ConfigSchema    []_registry.ConfigField `json:"config_schema"`
	Capabilities    []string                `json:"capabilities"`
}

type launchConfig struct {
	Manifest        manifest
	Dir             string
	Values          json.RawMessage
	InstanceID      int64
	CallbackBaseURL string
}

// Load registers adapter.json manifests in root. Existing built-in IDs are left untouched.
func Load(root string) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var failures []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, manifestName)); os.IsNotExist(err) {
			continue
		}
		if err := LoadOne(root, entry.Name(), false); err != nil {
			failures = append(failures, entry.Name()+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("自定义适配器加载异常: %s", strings.Join(failures, "; "))
	}
	return nil
}

func LoadOne(root, id string, replace bool) error {
	if !safeID.MatchString(id) {
		return fmt.Errorf("适配器 ID 无效")
	}
	dir := filepath.Join(root, id)
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return err
	}
	var manifestFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &manifestFields); err != nil {
		return fmt.Errorf("adapter.json 无效: %w", err)
	}
	if _, hasLocalProfile := manifestFields["runtime_profile"]; hasLocalProfile {
		delete(manifestFields, "runtime_profile")
		cleanData, err := json.MarshalIndent(manifestFields, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, manifestName), cleanData, 0644); err != nil {
			return err
		}
		data = cleanData
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("adapter.json 无效: %w", err)
	}
	if err := validateManifest(&m, dir); err != nil {
		return err
	}
	if m.ID != id {
		return fmt.Errorf("目录名必须与 adapter.json 的 id 一致")
	}
	desc := descriptorFor(m, dir)
	if replace {
		return _registry.ReplaceCustom(desc)
	}
	return _registry.RegisterDynamic(desc)
}

func descriptorFor(m manifest, dir string) _registry.Descriptor {
	cfg := launchConfig{Manifest: m, Dir: dir}
	return _registry.Descriptor{
		Platform: m.ID, DisplayName: m.Name, Version: m.Version, Author: m.Author,
		Runtime: m.Runtime, Custom: true, Description: m.Description, Entry: m.Entry,
		ConfigSchema: m.ConfigSchema, Capabilities: capabilitiesFor(m.Capabilities),
		DeclaredCapabilities: append([]string{}, m.Capabilities...),
		ParseConfig: func(raw string) (interface{}, error) {
			if !json.Valid([]byte(raw)) {
				return nil, fmt.Errorf("配置必须为有效 JSON")
			}
			copy := cfg
			copy.Values = json.RawMessage(raw)
			return copy, nil
		},
		NewAdapter: func(value interface{}) (_contract.Adapter, error) {
			parsed, ok := value.(launchConfig)
			if !ok {
				return nil, fmt.Errorf("自定义适配器配置类型无效")
			}
			return newProcessAdapter(parsed), nil
		},
	}
}

func CreateDefinition(root string, raw json.RawMessage, code string) error {
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("adapter.json 无效: %w", err)
	}
	if !safeID.MatchString(strings.TrimSpace(m.ID)) {
		return fmt.Errorf("适配器 ID 无效")
	}
	if filepath.IsAbs(m.Entry) || filepath.Clean(m.Entry) != m.Entry || strings.HasPrefix(m.Entry, "..") {
		return fmt.Errorf("entry 必须是适配器目录内的相对路径")
	}
	raw, err := manifestWithoutLocalRuntimeProfile(raw)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, strings.TrimSpace(m.ID))
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("适配器 ID 已存在")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	entryPath := filepath.Join(dir, m.Entry)
	if err := os.MkdirAll(filepath.Dir(entryPath), 0755); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	if err := os.WriteFile(entryPath, []byte(code), 0644); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), raw, 0644); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	if err := LoadOne(root, m.ID, false); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	return nil
}

func SaveDefinition(root, id string, raw json.RawMessage) error {
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("adapter.json 无效: %w", err)
	}
	if strings.TrimSpace(m.ID) != id {
		return fmt.Errorf("适配器 ID 不允许修改")
	}
	dir := filepath.Join(root, id)
	if err := validateManifest(&m, dir); err != nil {
		return err
	}
	raw, err := manifestWithoutLocalRuntimeProfile(raw)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), raw, 0644); err != nil {
		return err
	}
	return LoadOne(root, id, true)
}

func DeleteDefinition(root, id string) error {
	if !safeID.MatchString(id) {
		return fmt.Errorf("适配器 ID 无效")
	}
	if err := os.RemoveAll(filepath.Join(root, id)); err != nil {
		return err
	}
	return _registry.UnregisterCustom(id)
}

func validateManifest(m *manifest, dir string) error {
	m.ID, m.Name, m.Runtime, m.Entry = strings.TrimSpace(m.ID), strings.TrimSpace(m.Name), strings.ToLower(strings.TrimSpace(m.Runtime)), strings.TrimSpace(m.Entry)
	if !safeID.MatchString(m.ID) {
		return fmt.Errorf("id 仅支持字母、数字、点、下划线和连字符")
	}
	if m.Name == "" || m.Version == "" || m.Entry == "" {
		return fmt.Errorf("name、version 和 entry 均为必填项")
	}
	if m.ProtocolVersion != "1" {
		return fmt.Errorf("不支持协议版本 %q", m.ProtocolVersion)
	}
	if m.Runtime != "nodejs" && m.Runtime != "python" {
		return fmt.Errorf("runtime 只支持 nodejs 或 python")
	}
	if filepath.IsAbs(m.Entry) || filepath.Clean(m.Entry) != m.Entry || strings.HasPrefix(m.Entry, "..") {
		return fmt.Errorf("entry 必须是适配器目录内的相对路径")
	}
	entryPath := filepath.Join(dir, m.Entry)
	resolvedDir, dirErr := filepath.EvalSymlinks(dir)
	resolvedEntry, entryErr := filepath.EvalSymlinks(entryPath)
	if dirErr != nil || entryErr != nil || (resolvedEntry != resolvedDir && !strings.HasPrefix(resolvedEntry, resolvedDir+string(os.PathSeparator))) {
		return fmt.Errorf("entry 必须解析到适配器目录内部")
	}
	info, err := os.Stat(resolvedEntry)
	if err != nil || info.IsDir() {
		return fmt.Errorf("入口文件不存在: %s", m.Entry)
	}
	for _, field := range m.ConfigSchema {
		if strings.TrimSpace(field.Key) == "" {
			return fmt.Errorf("config_schema 字段 key 不能为空")
		}
		if field.Key == CallbackTokenField {
			return fmt.Errorf("config_schema 不能使用保留字段 %s", CallbackTokenField)
		}
		switch field.Type {
		case "text", "password", "number", "boolean", "textarea", "select":
		default:
			return fmt.Errorf("配置字段 %s 的 type 无效", field.Key)
		}
	}
	return nil
}

func manifestWithoutLocalRuntimeProfile(raw json.RawMessage) (json.RawMessage, error) {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("adapter.json 无效: %w", err)
	}
	delete(values, "runtime_profile")
	encoded, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func capabilitiesFor(items []string) _registry.Capabilities {
	result := _registry.Capabilities{SendText: true, SendImage: true, SendFile: true}
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item)) {
		case "image", "send_image":
			result.SendImage = true
		case "audio", "send_audio":
			result.SendAudio = true
		case "video", "send_video":
			result.SendVideo = true
		case "file", "send_file":
			result.SendFile = true
		case "markdown", "send_markdown":
			result.SendMarkdown = true
		case "rich", "send_rich":
			result.SendRich = true
		case "buttons", "send_buttons", "send_mixed_content":
			result.SendMixedContent = true
		case "private", "private_message":
			result.PrivateMessage = true
		case "group", "group_message":
			result.GroupMessage = true
		case "mention", "at_user":
			result.Mention = true
		}
	}
	return result
}

type wire struct {
	ID      string          `json:"id,omitempty"`
	Action  string          `json:"action,omitempty"`
	Event   string          `json:"event,omitempty"`
	OK      bool            `json:"ok,omitempty"`
	Error   string          `json:"error,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message *messagePayload `json:"message,omitempty"`
}

type messagePayload struct {
	ID        string               `json:"id"`
	Platform  string               `json:"platform"`
	UserID    string               `json:"user_id"`
	Nickname  string               `json:"nickname"`
	GroupID   string               `json:"group_id"`
	Content   string               `json:"content"`
	Medias    []types.MessageMedia `json:"medias"`
	MediaOnly bool                 `json:"media_only"`
	Metadata  map[string]string    `json:"metadata"`
}

func (m *messagePayload) toMessage() *types.Message {
	return &types.Message{ID: m.ID, Platform: m.Platform, UserID: m.UserID, Nickname: m.Nickname, GroupID: m.GroupID, Content: m.Content, Medias: m.Medias, MediaOnly: m.MediaOnly, Metadata: m.Metadata}
}

func fromMessage(msg *types.Message) *messagePayload {
	if msg == nil {
		return nil
	}
	return &messagePayload{ID: msg.ID, Platform: msg.Platform, UserID: msg.UserID, Nickname: msg.Nickname, GroupID: msg.GroupID, Content: msg.Content, Medias: msg.Medias, MediaOnly: msg.MediaOnly, Metadata: msg.Metadata}
}

type processAdapter struct {
	cfg            launchConfig
	capabilities   map[string]bool
	remark         string
	runtimeProfile string
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	mu             sync.Mutex
	writeMu        sync.Mutex
	waiters        map[string]chan wire
	nextID         atomic.Uint64
	handler        func(*types.Message)
	stopped        atomic.Bool
}

func newProcessAdapter(cfg launchConfig) *processAdapter {
	capabilities := make(map[string]bool, len(cfg.Manifest.Capabilities))
	for _, capability := range cfg.Manifest.Capabilities {
		capabilities[strings.ToLower(capability)] = true
	}
	return &processAdapter{cfg: cfg, capabilities: capabilities, waiters: make(map[string]chan wire)}
}
func (a *processAdapter) supports(name string) bool { return a.capabilities[name] }
func (a *processAdapter) GetPlatform() string       { return a.cfg.Manifest.ID }
func (a *processAdapter) SetRemark(remark string)   { a.remark = strings.TrimSpace(remark) }
func (a *processAdapter) SetInstanceID(id int64)    { a.cfg.InstanceID = id }
func (a *processAdapter) SetCallbackBaseURL(value string) {
	a.cfg.CallbackBaseURL = strings.TrimRight(strings.TrimSpace(value), "/")
}
func (a *processAdapter) SetMessageHandler(handler func(*types.Message)) {
	a.mu.Lock()
	a.handler = handler
	a.mu.Unlock()
}
func (a *processAdapter) SetRuntimeProfile(profileID string) {
	a.runtimeProfile = strings.TrimSpace(profileID)
}
func (a *processAdapter) Start() error {
	dir, err := filepath.Abs(a.cfg.Dir)
	if err != nil {
		return fmt.Errorf("解析适配器目录失败: %w", err)
	}
	entry := filepath.Join(dir, a.cfg.Manifest.Entry)
	var cmd *exec.Cmd
	var runtimeProfileID, nodePath string
	runtimeMu.RLock()
	node, python := nodeExecutable, pythonExecutable
	resolver := runtimeResolver
	runtimeMu.RUnlock()
	if profileID := a.runtimeProfile; profileID != "" {
		if resolver == nil {
			return fmt.Errorf("无法解析运行环境 Profile: %s", profileID)
		}
		executable, resolvedNodePath, err := resolver(a.cfg.Manifest.Runtime, profileID)
		if err != nil {
			return fmt.Errorf("解析运行环境 Profile %s 失败: %w", profileID, err)
		}
		runtimeProfileID, nodePath = profileID, resolvedNodePath
		if a.cfg.Manifest.Runtime == "nodejs" {
			node = executable
		} else {
			python = executable
		}
	}
	if a.cfg.Manifest.Runtime == "nodejs" {
		cmd = exec.Command(node, entry)
	} else {
		cmd = exec.Command(python, entry)
	}
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ALLBOT_ADAPTER_ID="+a.cfg.Manifest.ID, "ALLBOT_ADAPTER_PROTOCOL=1")
	if runtimeProfileID != "" {
		cmd.Env = append(cmd.Env, "ALLBOT_RUNTIME_PROFILE="+runtimeProfileID)
	}
	if nodePath != "" && a.cfg.Manifest.Runtime == "nodejs" {
		cmd.Env = append(cmd.Env, "NODE_PATH="+nodePath)
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
		return fmt.Errorf("启动适配器子进程失败: %w", err)
	}
	a.cmd, a.stdin = cmd, stdin
	go a.readLoop(stdout)
	go a.logLoop(stderr)
	go func() {
		if err := cmd.Wait(); err != nil && !a.stopped.Load() {
			log.Printf("[ADAPTER:%s] 子进程退出: %v", a.GetPlatform(), err)
		}
	}()
	var configured map[string]interface{}
	if err := json.Unmarshal(a.cfg.Values, &configured); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("解析实例配置失败: %w", err)
	}
	token, _ := configured[CallbackTokenField].(string)
	delete(configured, CallbackTokenField)
	callbackPath := fmt.Sprintf("/api/open/adapters/custom/%d/%s", a.cfg.InstanceID, token)
	callbackURL := callbackPath
	if a.cfg.CallbackBaseURL != "" {
		callbackURL = a.cfg.CallbackBaseURL + callbackPath
	}
	_, err = a.request("start", map[string]interface{}{"config": configured, "protocol_version": "1", "callback_url": callbackURL}, 20*time.Second)
	if err != nil {
		a.stopped.Store(true)
		_ = cmd.Process.Kill()
		return err
	}
	return nil
}

func (a *processAdapter) Stop() error {
	if !a.stopped.CompareAndSwap(false, true) {
		return nil
	}
	_, _ = a.request("stop", nil, 3*time.Second)
	if a.stdin != nil {
		_ = a.stdin.Close()
	}
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
	}
	return nil
}
func (a *processAdapter) IsHealthy() bool {
	return !a.stopped.Load() && a.cmd != nil && a.cmd.ProcessState == nil
}
func (a *processAdapter) readLoop(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for scanner.Scan() {
		var msg wire
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			log.Printf("[ADAPTER:%s] 忽略无效协议行: %v", a.GetPlatform(), err)
			continue
		}
		if msg.Event == "message" && msg.Message != nil {
			incoming := msg.Message.toMessage()
			a.logReceivedMessage(incoming)
			a.mu.Lock()
			h := a.handler
			a.mu.Unlock()
			if h != nil {
				h(incoming)
			}
			continue
		}
		if msg.ID != "" {
			a.mu.Lock()
			ch := a.waiters[msg.ID]
			delete(a.waiters, msg.ID)
			a.mu.Unlock()
			if ch != nil {
				ch <- msg
			}
		}
	}
	if err := scanner.Err(); err != nil && !a.stopped.Load() {
		log.Printf("[ADAPTER:%s] 读取协议失败: %v", a.GetPlatform(), err)
	}
}

func (a *processAdapter) logReceivedMessage(msg *types.Message) {
	if msg == nil {
		return
	}
	name := strings.TrimSpace(a.cfg.Manifest.Name)
	if name == "" {
		name = a.cfg.Manifest.ID
	}
	user := strings.TrimSpace(msg.UserID)
	if user == "" {
		user = "-"
	}
	nickname := strings.TrimSpace(msg.Nickname)
	if nickname != "" {
		nickname = "<" + nickname + ">"
	}
	chat := "私聊"
	if groupID := strings.TrimSpace(msg.GroupID); groupID != "" {
		chat = "群 " + groupID
	}
	content := msg.Content
	if content == "" && len(msg.Medias) > 0 {
		content = "[媒体消息]"
	}
	log.Printf("[接收][%s]%s[%s%s(%s)]：%s", name, _contract.RemarkTag(a.remark), user, nickname, chat, content)
}

func (a *processAdapter) logLoop(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		log.Printf("[ADAPTER:%s] %s", a.GetPlatform(), scanner.Text())
	}
}
func (a *processAdapter) request(action string, data interface{}, timeout time.Duration) (json.RawMessage, error) {
	if a.stdin == nil {
		return nil, fmt.Errorf("适配器未启动")
	}
	id := fmt.Sprintf("%d", a.nextID.Add(1))
	ch := make(chan wire, 1)
	a.mu.Lock()
	a.waiters[id] = ch
	a.mu.Unlock()
	encoded, err := json.Marshal(map[string]interface{}{"id": id, "action": action, "data": data})
	if err != nil {
		return nil, err
	}
	a.writeMu.Lock()
	_, err = a.stdin.Write(append(encoded, '\n'))
	a.writeMu.Unlock()
	if err != nil {
		a.mu.Lock()
		delete(a.waiters, id)
		a.mu.Unlock()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	select {
	case response := <-ch:
		if !response.OK {
			return nil, fmt.Errorf("%s", response.Error)
		}
		return response.Data, nil
	case <-ctx.Done():
		a.mu.Lock()
		delete(a.waiters, id)
		a.mu.Unlock()
		return nil, fmt.Errorf("适配器请求超时: %s", action)
	}
}
func (a *processAdapter) call(action string, data interface{}) error {
	_, err := a.request(action, data, 20*time.Second)
	return err
}
func (a *processAdapter) SendMessage(target, text string) error {
	return a.call("send_text", map[string]string{"target": target, "text": text})
}
func (a *processAdapter) SendImage(target, url string) error {
	return a.call("send_image", map[string]string{"target": target, "url": url})
}
func (a *processAdapter) SendFile(target, path string) error {
	return a.call("send_file", map[string]string{"target": target, "path": path})
}
func (a *processAdapter) SendAudio(target, url string) error {
	if !a.supports("audio") {
		return _contract.ErrUnsupported
	}
	return a.call("send_audio", map[string]string{"target": target, "url": url})
}
func (a *processAdapter) SendVideo(target, url string) error {
	if !a.supports("video") {
		return _contract.ErrUnsupported
	}
	return a.call("send_video", map[string]string{"target": target, "url": url})
}
func (a *processAdapter) SendMarkdown(target, markdown string) error {
	if !a.supports("markdown") {
		return _contract.ErrUnsupported
	}
	return a.call("send_markdown", map[string]string{"target": target, "markdown": markdown})
}
func (a *processAdapter) SendRichMessage(target string, message types.RichMessage) error {
	if !a.supports("rich") {
		return _contract.ErrUnsupported
	}
	return a.call("send_rich", map[string]interface{}{"target": target, "message": message})
}
func (a *processAdapter) SendButtons(target, text string, buttons [][]types.ButtonOption) error {
	if !a.supports("buttons") {
		return _contract.ErrUnsupported
	}
	return a.call("send_buttons", map[string]interface{}{"target": target, "text": text, "buttons": buttons})
}
func (a *processAdapter) SendMessageWithSequence(target, text string, sequence int) error {
	if !a.supports("sequence") {
		return _contract.ErrUnsupported
	}
	return a.call("send_text_sequence", map[string]interface{}{"target": target, "text": text, "sequence": sequence})
}
func (a *processAdapter) GetUserInfo(userID string) (*_contract.UserInfo, error) {
	raw, err := a.request("get_user_info", map[string]string{"user_id": userID}, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var value struct {
		UserID   string            `json:"user_id"`
		Nickname string            `json:"nickname"`
		Avatar   string            `json:"avatar"`
		Extra    map[string]string `json:"extra"`
	}
	err = json.Unmarshal(raw, &value)
	return &_contract.UserInfo{UserID: value.UserID, Nickname: value.Nickname, Avatar: value.Avatar, Extra: value.Extra}, err
}
func (a *processAdapter) GetGroupInfo(groupID string) (*_contract.GroupInfo, error) {
	raw, err := a.request("get_group_info", map[string]string{"group_id": groupID}, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var value struct {
		GroupID     string            `json:"group_id"`
		Name        string            `json:"name"`
		MemberCount int               `json:"member_count"`
		Extra       map[string]string `json:"extra"`
	}
	err = json.Unmarshal(raw, &value)
	return &_contract.GroupInfo{GroupID: value.GroupID, Name: value.Name, MemberCount: value.MemberCount, Extra: value.Extra}, err
}
func (a *processAdapter) AtUser(groupID, userID string) error {
	return a.call("at_user", map[string]string{"group_id": groupID, "user_id": userID})
}
func (a *processAdapter) DeleteMessage(msg *types.Message) error {
	if !a.supports("delete_message") {
		return _contract.ErrUnsupported
	}
	return a.call("delete_message", map[string]interface{}{"message": fromMessage(msg)})
}
func (a *processAdapter) Mute(groupID, userID string, seconds int) error {
	if !a.supports("mute") {
		return _contract.ErrUnsupported
	}
	return a.call("mute", map[string]interface{}{"group_id": groupID, "user_id": userID, "duration_seconds": seconds})
}
func (a *processAdapter) ReplyTarget(msg *types.Message) string {
	if msg != nil && msg.Metadata != nil && msg.Metadata["reply_target"] != "" {
		return msg.Metadata["reply_target"]
	}
	if msg != nil && msg.GroupID != "" {
		return "group_" + msg.GroupID
	}
	if msg != nil {
		return "user_" + msg.UserID
	}
	return ""
}
func (a *processAdapter) FormatReplyText(_ *types.Message, text string) string { return text }
func (a *processAdapter) SendTarget(userID, groupID string) string {
	if groupID != "" {
		return "group_" + groupID
	}
	return "user_" + userID
}

func (a *processAdapter) HandleHTTPCallback(relativePath string, w http.ResponseWriter, r *http.Request) {
	if !a.supports("webhook") {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2*1024*1024))
	if err != nil {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	headers := make(map[string][]string, len(r.Header))
	for key, values := range r.Header {
		headers[key] = append([]string(nil), values...)
	}
	response, err := a.request("http_callback", map[string]interface{}{
		"method": r.Method, "path": relativePath, "query": r.URL.Query(), "headers": headers, "body": string(body),
	}, 25*time.Second)
	if err != nil {
		http.Error(w, "Adapter callback failed", http.StatusBadGateway)
		return
	}
	var result struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		http.Error(w, "Invalid adapter callback response", http.StatusBadGateway)
		return
	}
	if result.Status < 100 || result.Status > 599 {
		result.Status = http.StatusOK
	}
	for key, value := range result.Headers {
		if strings.EqualFold(key, "Content-Length") || strings.EqualFold(key, "Transfer-Encoding") || strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			continue
		}
		w.Header().Set(key, value)
	}
	w.WriteHeader(result.Status)
	_, _ = io.WriteString(w, result.Body)
}

var _ _contract.Adapter = (*processAdapter)(nil)
var _ _contract.AudioSender = (*processAdapter)(nil)
var _ _contract.VideoSender = (*processAdapter)(nil)
var _ _contract.HealthChecker = (*processAdapter)(nil)
var _ _contract.MessageDeleter = (*processAdapter)(nil)
var _ _contract.GroupMuter = (*processAdapter)(nil)
var _ _contract.MarkdownSender = (*processAdapter)(nil)
var _ _contract.RichMessageSender = (*processAdapter)(nil)
var _ _contract.ButtonSender = (*processAdapter)(nil)
var _ _contract.MessageSequenceSender = (*processAdapter)(nil)
var _ _contract.ReplyTargetResolver = (*processAdapter)(nil)
var _ _contract.ReplyTextFormatter = (*processAdapter)(nil)
var _ _contract.SendTargetResolver = (*processAdapter)(nil)
var _ _contract.HTTPCallbackHandler = (*processAdapter)(nil)

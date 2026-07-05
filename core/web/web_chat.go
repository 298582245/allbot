package web

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/adapter"
	webadapter "github.com/allbot/allbot/core/adapter/web"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/imagehost"
	"github.com/allbot/allbot/core/types"
)

const webChatCookieName = "allbot_web_chat_session"

type webChatEmailSender interface {
	SendWebChatCode(to, code string) error
}

type smtpWebChatEmailSender struct{ adapter *webadapter.Adapter }

const webChatAdapterUnavailableMessage = "web 适配器未启动，请管理员先在机器人管理中添加并启用 Web 聊天室"

type webChatRateLimiter struct {
	mu     sync.Mutex
	events map[string][]time.Time
}

var defaultWebChatLimiter = &webChatRateLimiter{events: map[string][]time.Time{}}

func (s *Server) handleWebChatAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/open/web-chat"), "/")
	switch path {
	case "email-code":
		s.handleWebChatEmailCode(w, r)
	case "register":
		s.handleWebChatRegister(w, r)
	case "reset-password":
		s.handleWebChatResetPassword(w, r)
	case "login":
		s.handleWebChatLogin(w, r)
	case "email-login":
		s.handleWebChatEmailLogin(w, r)
	case "logout":
		s.handleWebChatLogout(w, r)
	case "me":
		s.handleWebChatMe(w, r)
	case "bind-code":
		s.handleWebChatBindCode(w, r)
	case "plugins":
		s.handleWebChatPlugins(w, r)
	case "messages":
		if r.Method == http.MethodGet {
			s.handleWebChatMessages(w, r)
		} else {
			s.handleWebChatSendMessage(w, r)
		}
	case "message-counts":
		s.handleWebChatMessageCounts(w, r)
	case "read-state":
		s.handleWebChatReadState(w, r)
	case "events":
		s.handleWebChatEvents(w, r)
	case "images":
		s.handleWebChatImages(w, r)
	default:
		s.jsonError(w, "接口不存在", http.StatusNotFound)
	}
}

func (s *Server) handleWebChatEmailCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validWebChatOrigin(r) {
		s.jsonError(w, "请求来源无效", http.StatusForbidden)
		return
	}
	adp, ok := s.requireRunningWebChatAdapter(w)
	if !ok {
		return
	}
	if !defaultWebChatLimiter.Allow("email:"+webChatClientIP(r), time.Minute, 10) {
		s.jsonError(w, "请求过于频繁", http.StatusTooManyRequests)
		return
	}
	var req struct {
		Email   string `json:"email"`
		Purpose string `json:"purpose"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	database, ok := s.requireWebChatDatabase(w)
	if !ok {
		return
	}
	purpose := strings.TrimSpace(req.Purpose)
	if purpose == "" {
		purpose = config.WebChatEmailPurposeRegister
	}
	switch purpose {
	case config.WebChatEmailPurposeRegister:
	case config.WebChatEmailPurposeResetPassword, config.WebChatEmailPurposeLogin:
		keyPrefix := "email-reset:"
		if purpose == config.WebChatEmailPurposeLogin {
			keyPrefix = "email-login:"
		}
		purposeKey := keyPrefix + webChatClientIP(r) + ":" + strings.ToLower(strings.TrimSpace(req.Email))
		if !defaultWebChatLimiter.Allow(purposeKey, time.Minute, 1) {
			s.jsonError(w, "请求过于频繁", http.StatusTooManyRequests)
			return
		}
		exists, err := database.WebChatUserExistsByEmail(req.Email)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !exists {
			s.jsonResponse(w, map[string]bool{"success": true})
			return
		}
	default:
		s.jsonError(w, "验证码用途无效", http.StatusBadRequest)
		return
	}
	code, err := config.RandomWebChatEmailCode()
	if err != nil {
		s.jsonError(w, "生成验证码失败", http.StatusInternalServerError)
		return
	}
	if err := database.CreateWebChatEmailCode(req.Email, code, purpose, webChatClientIP(r)); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.webChatEmailSender(adp).SendWebChatCode(req.Email, code); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]bool{"success": true})
}

func (s *Server) handleWebChatRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validWebChatOrigin(r) {
		s.jsonError(w, "请求来源无效", http.StatusForbidden)
		return
	}
	if _, ok := s.requireRunningWebChatAdapter(w); !ok {
		return
	}
	if !defaultWebChatLimiter.Allow("register:"+webChatClientIP(r), 10*time.Minute, 20) {
		s.jsonError(w, "注册请求过于频繁", http.StatusTooManyRequests)
		return
	}
	var req struct {
		Email       string `json:"email"`
		Code        string `json:"code"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		BindCode    string `json:"bind_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	database, ok := s.requireWebChatDatabase(w)
	if !ok {
		return
	}
	user, err := database.RegisterWebChatUser(config.WebChatRegisterInput{Email: req.Email, Code: req.Code, Username: req.Username, Password: req.Password, DisplayName: req.DisplayName, BindCode: req.BindCode})
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	session, err := database.CreateWebChatSession(user.UserID, r.UserAgent(), webChatClientIP(r))
	if err != nil {
		s.jsonError(w, "创建会话失败", http.StatusInternalServerError)
		return
	}
	s.setWebChatCookie(w, r, session.Token)
	s.jsonResponse(w, session)
}

func (s *Server) handleWebChatResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validWebChatOrigin(r) {
		s.jsonError(w, "请求来源无效", http.StatusForbidden)
		return
	}
	if _, ok := s.requireRunningWebChatAdapter(w); !ok {
		return
	}
	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	key := "reset-password:" + webChatClientIP(r) + ":" + strings.ToLower(strings.TrimSpace(req.Email))
	if !defaultWebChatLimiter.Allow(key, 10*time.Minute, 20) {
		s.jsonError(w, "重置密码请求过于频繁", http.StatusTooManyRequests)
		return
	}
	database, ok := s.requireWebChatDatabase(w)
	if !ok {
		return
	}
	if err := database.ResetWebChatUserPassword(req.Email, req.Code, req.Password); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]bool{"success": true})
}

func (s *Server) handleWebChatLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validWebChatOrigin(r) {
		s.jsonError(w, "请求来源无效", http.StatusForbidden)
		return
	}
	if _, ok := s.requireRunningWebChatAdapter(w); !ok {
		return
	}
	var req struct{ Login, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	key := "login:" + webChatClientIP(r) + ":" + strings.ToLower(strings.TrimSpace(req.Login))
	if !defaultWebChatLimiter.Allow(key, 10*time.Minute, 20) {
		s.jsonError(w, "登录请求过于频繁", http.StatusTooManyRequests)
		return
	}
	database, ok := s.requireWebChatDatabase(w)
	if !ok {
		return
	}
	user, err := database.VerifyWebChatLogin(req.Login, req.Password)
	if err != nil {
		s.jsonError(w, "账号或密码错误", http.StatusUnauthorized)
		return
	}
	session, err := database.CreateWebChatSession(user.UserID, r.UserAgent(), webChatClientIP(r))
	if err != nil {
		s.jsonError(w, "创建会话失败", http.StatusInternalServerError)
		return
	}
	s.setWebChatCookie(w, r, session.Token)
	s.jsonResponse(w, session)
}

func (s *Server) handleWebChatEmailLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validWebChatOrigin(r) {
		s.jsonError(w, "请求来源无效", http.StatusForbidden)
		return
	}
	if _, ok := s.requireRunningWebChatAdapter(w); !ok {
		return
	}
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	key := "email-login-submit:" + webChatClientIP(r) + ":" + strings.ToLower(strings.TrimSpace(req.Email))
	if !defaultWebChatLimiter.Allow(key, 10*time.Minute, 20) {
		s.jsonError(w, "登录请求过于频繁", http.StatusTooManyRequests)
		return
	}
	database, ok := s.requireWebChatDatabase(w)
	if !ok {
		return
	}
	user, err := database.VerifyWebChatEmailLogin(req.Email, req.Code)
	if err != nil {
		s.jsonError(w, "邮箱验证码错误", http.StatusUnauthorized)
		return
	}
	session, err := database.CreateWebChatSession(user.UserID, r.UserAgent(), webChatClientIP(r))
	if err != nil {
		s.jsonError(w, "创建会话失败", http.StatusInternalServerError)
		return
	}
	s.setWebChatCookie(w, r, session.Token)
	s.jsonResponse(w, session)
}

func (s *Server) handleWebChatLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if database := s.runtimeDatabase(); database != nil {
		_ = database.DeleteWebChatSession(s.webChatToken(r))
	}
	s.clearWebChatCookie(w)
	s.jsonResponse(w, map[string]bool{"success": true})
}

func (s *Server) handleWebChatMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.requireRunningWebChatAdapter(w); !ok {
		return
	}
	session, ok := s.requireWebChatSession(w, r, false)
	if !ok {
		return
	}
	s.jsonResponse(w, session)
}

func (s *Server) handleWebChatBindCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.requireRunningWebChatAdapter(w); !ok {
		return
	}
	session, ok := s.requireWebChatSession(w, r, true)
	if !ok {
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	database := s.runtimeDatabase()
	webAccount, sourceAccount, err := database.BindWebChatUserByCode(session.User.UserID, req.Code)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := database.GetWebChatUser(session.User.UserID)
	s.jsonResponse(w, map[string]interface{}{"web_account": webAccount, "source_account": sourceAccount, "user": user})
}

func (s *Server) handleWebChatPlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	adapterID, ok := s.requireRunningWebChatAdapterID(w)
	if !ok {
		return
	}
	session, ok := s.requireWebChatSession(w, r, false)
	if !ok {
		return
	}
	msg := &types.Message{Platform: config.WebChatPlatform, AdapterID: adapterID, UserID: session.User.UserID, Content: ""}
	allowed := map[string]bool{}
	if s.router != nil {
		for _, plugin := range s.router.FilterPluginsForMessage(msg, false) {
			allowed[plugin.ID] = true
		}
	}
	items := make([]map[string]interface{}, 0)
	if s.pluginManager != nil {
		for _, process := range s.pluginManager.GetAllPlugins() {
			if process == nil || process.Plugin == nil || !allowed[process.Plugin.ID] || !webChatPluginRunning(process.Status) || !webChatPluginVisible(process.Plugin) {
				continue
			}
			items = append(items, webChatPluginItem(process.Plugin, process.Status))
		}
	}
	sort.Slice(items, func(i, j int) bool { return fmt.Sprint(items[i]["title"]) < fmt.Sprint(items[j]["title"]) })
	s.jsonResponse(w, items)
}

func (s *Server) handleWebChatMessages(w http.ResponseWriter, r *http.Request) {
	adapterID, ok := s.requireRunningWebChatAdapterID(w)
	if !ok {
		return
	}
	session, ok := s.requireWebChatSession(w, r, false)
	if !ok {
		return
	}
	pluginID := strings.TrimSpace(r.URL.Query().Get("plugin_id"))
	if pluginID != "" && !s.webChatPluginAvailable(session.User.UserID, adapterID, pluginID) {
		s.jsonError(w, "插件不可用", http.StatusBadRequest)
		return
	}
	afterID, _ := strconv.ParseInt(r.URL.Query().Get("after_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.runtimeDatabase().ListWebChatMessagesByPlugin(session.User.UserID, pluginID, afterID, limit)
	if err != nil {
		s.jsonError(w, "读取消息失败", http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, items)
}

func (s *Server) handleWebChatSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	adp, ok := s.requireRunningWebChatAdapter(w)
	if !ok {
		return
	}
	session, ok := s.requireWebChatSession(w, r, true)
	if !ok {
		return
	}
	if !defaultWebChatLimiter.Allow("msg:"+session.User.UserID, time.Minute, 60) {
		s.jsonError(w, "发送过于频繁", http.StatusTooManyRequests)
		return
	}
	var req struct {
		PluginID string        `json:"plugin_id"`
		Type     string        `json:"type"`
		Content  string        `json:"content"`
		ImageURL string        `json:"image_url"`
		Parts    []interface{} `json:"parts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	pluginID := strings.TrimSpace(req.PluginID)
	if pluginID != "" && !s.webChatPluginAvailable(session.User.UserID, s.runningWebAdapterID(), pluginID) {
		s.jsonError(w, "插件不可用", http.StatusBadRequest)
		return
	}
	requestType := strings.TrimSpace(req.Type)
	if (requestType != "" && requestType != "text") || strings.TrimSpace(req.ImageURL) != "" || len(req.Parts) > 0 {
		s.jsonError(w, "Web 聊天室用户消息暂只支持文本", http.StatusBadRequest)
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		s.jsonError(w, "消息内容不能为空", http.StatusBadRequest)
		return
	}
	saved, err := s.runtimeDatabase().SaveWebChatMessage(&config.WebChatMessage{UserID: session.User.UserID, Direction: "in", MessageType: "text", Content: content, Target: "user_" + session.User.UserID, PluginID: pluginID})
	if err != nil {
		s.jsonError(w, "保存消息失败", http.StatusInternalServerError)
		return
	}
	metadata := map[string]string{"message_type": "text"}
	sessionGroupID := ""
	if pluginID != "" {
		metadata["web_chat_plugin_id"] = pluginID
		sessionGroupID = webChatPluginGroupID(pluginID)
		metadata["web_chat_session_group_id"] = sessionGroupID
	}
	msg := &types.Message{ID: fmt.Sprintf("web_%d", time.Now().UnixNano()), Platform: config.WebChatPlatform, AdapterID: s.runningWebAdapterID(), UserID: session.User.UserID, Content: content, Metadata: metadata}
	log.Printf("[接收][web][%s][session_group=%s]：%s", session.User.UserID, sessionGroupID, content)
	if s.router != nil {
		if pluginID == "" {
			s.router.HandleMessage(msg)
		} else if s.router.HasWaitingSessionForPlugin(msg.UserID, sessionGroupID, pluginID) {
			if err := s.router.HandleMessageForPlugin(msg, pluginID); err != nil {
				log.Printf("[SYSTEM] Web chat plugin dispatch failed: %v", err)
				s.sendWebChatPluginDispatchError(adp, msg, pluginID, err)
			}
		} else if targetPlugin, status := s.webChatMatchedOtherPlugin(msg, pluginID); targetPlugin != nil {
			if err := s.sendWebChatPluginSuggestion(adp, msg, pluginID, targetPlugin, status); err != nil {
				log.Printf("[SYSTEM] Web chat plugin suggestion failed: %v", err)
			}
		} else if err := s.router.HandleMessageForPlugin(msg, pluginID); err != nil {
			log.Printf("[SYSTEM] Web chat plugin dispatch failed: %v", err)
			s.sendWebChatPluginDispatchError(adp, msg, pluginID, err)
		}
	} else {
		adp.ReceiveMessage(session.User.UserID, content, "text", "", "")
	}
	s.jsonResponse(w, saved)
}

func (s *Server) handleWebChatMessageCounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.requireRunningWebChatAdapterID(w); !ok {
		return
	}
	session, ok := s.requireWebChatSession(w, r, false)
	if !ok {
		return
	}
	items, err := s.runtimeDatabase().CountWebChatMessagesByPlugin(session.User.UserID)
	if err != nil {
		s.jsonError(w, "读取消息数量失败", http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, items)
}

func (s *Server) handleWebChatReadState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	adapterID, ok := s.requireRunningWebChatAdapterID(w)
	if !ok {
		return
	}
	session, ok := s.requireWebChatSession(w, r, true)
	if !ok {
		return
	}
	var req struct {
		PluginID string `json:"plugin_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	pluginID := strings.TrimSpace(req.PluginID)
	if pluginID != "" && !s.webChatPluginAvailable(session.User.UserID, adapterID, pluginID) {
		s.jsonError(w, "插件不可用", http.StatusBadRequest)
		return
	}
	item, err := s.runtimeDatabase().MarkWebChatMessagesRead(session.User.UserID, pluginID)
	if err != nil {
		s.jsonError(w, "更新已读状态失败", http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, item)
}

func (s *Server) handleWebChatEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	adp, ok := s.requireRunningWebChatAdapter(w)
	if !ok {
		return
	}
	session, ok := s.requireWebChatSession(w, r, false)
	if !ok {
		return
	}
	ch, cancel := adp.Subscribe(session.User.UserID, 16)
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)
	_, _ = w.Write([]byte(": connected\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(msg)
			_, _ = w.Write([]byte("event: message\n"))
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleWebChatImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.requireRunningWebChatAdapter(w); !ok {
		return
	}
	if _, ok := s.requireWebChatSession(w, r, true); !ok {
		return
	}
	service, ok := s.requireImageHostService(w)
	if !ok {
		return
	}
	settings, err := service.Settings()
	if err != nil {
		s.jsonError(w, "读取图床配置失败", http.StatusInternalServerError)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, settings.MaxSize+1024*1024)
	if err := r.ParseMultipartForm(settings.MaxSize + 1024*1024); err != nil {
		s.jsonError(w, "上传表单无效", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		s.jsonError(w, "图片文件不能为空", http.StatusBadRequest)
		return
	}
	defer file.Close()
	originalName := ""
	if header != nil {
		originalName = header.Filename
	}
	asset, err := service.Upload(imagehost.UploadInput{Reader: file, OriginalName: originalName, RequestHost: r.Host, RequestScheme: requestScheme(r)})
	if err != nil {
		if errors.Is(err, imagehost.ErrInvalidInput) {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonError(w, "上传图片失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, asset)
}

func (s *Server) requireWebChatDatabase(w http.ResponseWriter) (*config.Database, bool) {
	database := s.runtimeDatabase()
	if database == nil {
		s.jsonError(w, "数据库不可用", http.StatusInternalServerError)
		return nil, false
	}
	return database, true
}

func (s *Server) requireWebChatSession(w http.ResponseWriter, r *http.Request, csrf bool) (*config.WebChatSession, bool) {
	database, ok := s.requireWebChatDatabase(w)
	if !ok {
		return nil, false
	}
	session, err := database.GetWebChatSession(s.webChatToken(r))
	if err != nil {
		s.jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if csrf {
		if !s.validWebChatOrigin(r) || strings.TrimSpace(r.Header.Get("X-AllBot-WebChat-CSRF")) != session.CSRFToken {
			s.jsonError(w, "CSRF 校验失败", http.StatusForbidden)
			return nil, false
		}
	}
	return session, true
}

func (s *Server) webChatToken(r *http.Request) string {
	cookie, err := r.Cookie(webChatCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func (s *Server) setWebChatCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{Name: webChatCookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil, MaxAge: int((7 * 24 * time.Hour).Seconds())})
}

func (s *Server) clearWebChatCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: webChatCookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func (s *Server) requireRunningWebChatAdapter(w http.ResponseWriter) (*webadapter.Adapter, bool) {
	adp := s.webChatAdapter()
	if adp == nil {
		s.jsonError(w, webChatAdapterUnavailableMessage, http.StatusServiceUnavailable)
		return nil, false
	}
	return adp, true
}

func (s *Server) requireRunningWebChatAdapterID(w http.ResponseWriter) (string, bool) {
	adapterID := s.runningWebAdapterID()
	if adapterID == "" {
		s.jsonError(w, webChatAdapterUnavailableMessage, http.StatusServiceUnavailable)
		return "", false
	}
	return adapterID, true
}

func (s *Server) webChatAdapter() *webadapter.Adapter {
	if s.adapterManager == nil {
		return nil
	}
	if adp, ok := s.adapterManager.GetAdapter(config.WebChatPlatform).(*webadapter.Adapter); ok {
		return adp
	}
	return nil
}

func (s *Server) runningWebAdapterID() string {
	if s.adapterManager == nil || s.adapterManager.GetDatabase() == nil {
		return ""
	}
	items, err := s.adapterManager.GetDatabase().GetAllAdapters()
	if err != nil {
		return ""
	}
	for _, item := range items {
		if item != nil && item.Enabled && item.Platform == config.WebChatPlatform && s.adapterManager.GetAdapterByID(item.ID) != nil {
			return strconv.FormatInt(item.ID, 10)
		}
	}
	return ""
}

func (s *Server) webChatEmailSender(adp *webadapter.Adapter) webChatEmailSender {
	if s.webChatMailer != nil {
		return s.webChatMailer
	}
	return smtpWebChatEmailSender{adapter: adp}
}

func (sender smtpWebChatEmailSender) SendWebChatCode(to, code string) error {
	if sender.adapter == nil {
		return fmt.Errorf("Web 聊天室 SMTP 未配置，请先在机器人管理中编辑 Web 聊天室实例")
	}
	cfg := sender.adapter.SMTPConfig()
	host := strings.TrimSpace(cfg.SMTPHost)
	port := strings.TrimSpace(cfg.SMTPPort)
	username := strings.TrimSpace(cfg.SMTPUsername)
	password := strings.TrimSpace(cfg.SMTPPassword)
	from := strings.TrimSpace(cfg.SMTPFrom)
	if host == "" || port == "" || username == "" || password == "" || from == "" {
		return fmt.Errorf("Web 聊天室 SMTP 未配置，请先在机器人管理中编辑 Web 聊天室实例")
	}
	addr := host + ":" + port
	auth := smtp.PlainAuth("", username, password, host)
	body := buildWebChatCodeEmail(from, to, cfg.SMTPSubject, code)
	if port == "465" {
		return sendSMTPWithImplicitTLS(addr, host, auth, from, []string{to}, []byte(body))
	}
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(body))
}

func buildWebChatCodeEmail(from, to, subject, code string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = webadapter.DefaultSMTPSubject
	}
	subject = strings.ReplaceAll(strings.ReplaceAll(subject, "\r", " "), "\n", " ")
	return "From: " + from + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n您的验证码是：" + code + "，10 分钟内有效。\r\n"
}

func sendSMTPWithImplicitTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, address := range to {
		if err := client.Rcpt(address); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (l *webChatRateLimiter) Allow(key string, window time.Duration, max int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-window)
	items := l.events[key]
	kept := items[:0]
	for _, item := range items {
		if item.After(cutoff) {
			kept = append(kept, item)
		}
	}
	if len(kept) >= max {
		l.events[key] = kept
		return false
	}
	l.events[key] = append(kept, now)
	return true
}

func (s *Server) validWebChatOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		origin = strings.TrimSpace(r.Header.Get("Referer"))
	}
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func webChatClientIP(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); value != "" {
		return strings.TrimSpace(strings.Split(value, ",")[0])
	}
	if value := strings.TrimSpace(r.Header.Get("X-Real-IP")); value != "" {
		return value
	}
	return strings.TrimSpace(strings.Split(r.RemoteAddr, ":")[0])
}

func webChatPluginGroupID(pluginID string) string {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return ""
	}
	return "web_plugin_" + pluginID
}

func (s *Server) webChatMatchedOtherPlugin(msg *types.Message, currentPluginID string) (*types.Plugin, string) {
	if s.router == nil || msg == nil {
		return nil, ""
	}
	statusByPlugin := map[string]string{}
	if s.pluginManager != nil {
		for _, process := range s.pluginManager.GetAllPlugins() {
			if process != nil && process.Plugin != nil {
				statusByPlugin[process.Plugin.ID] = process.Status
			}
		}
	}
	currentPluginID = strings.TrimSpace(currentPluginID)
	matched := s.router.FilterPluginsForMessage(msg, true)
	for _, plugin := range matched {
		if plugin != nil && plugin.ID == currentPluginID {
			return nil, ""
		}
	}
	for _, plugin := range matched {
		if plugin == nil || !webChatPluginVisible(plugin) {
			continue
		}
		status, hasStatus := statusByPlugin[plugin.ID]
		if hasStatus && !webChatPluginRunning(status) {
			continue
		}
		return plugin, status
	}
	return nil, ""
}

func (s *Server) sendWebChatPluginDispatchError(adp *webadapter.Adapter, msg *types.Message, pluginID string, dispatchErr error) {
	if adp == nil || msg == nil || strings.TrimSpace(pluginID) == "" || dispatchErr == nil {
		return
	}
	text := "你没有权限使用该插件"
	if !strings.Contains(dispatchErr.Error(), "无权访问") && !strings.Contains(dispatchErr.Error(), "未启用") && !strings.Contains(dispatchErr.Error(), "不存在") {
		text = "插件处理失败：" + dispatchErr.Error()
	}
	target := adp.SendTarget(msg.UserID, "") + "#plugin_" + strings.TrimSpace(pluginID)
	if err := adp.SendWebChatMessage(target, &config.WebChatMessage{MessageType: "text", Content: text, PluginID: pluginID}); err != nil {
		log.Printf("[SYSTEM] Web chat plugin error reply failed: %v", err)
	}
}

func (s *Server) sendWebChatPluginSuggestion(adp *webadapter.Adapter, msg *types.Message, currentPluginID string, targetPlugin *types.Plugin, status string) error {
	if adp == nil || msg == nil || targetPlugin == nil {
		return fmt.Errorf("插件切换建议参数无效")
	}
	pluginItem := webChatPluginItem(targetPlugin, status)
	fallback := fmt.Sprintf("检测到你可能想打开「%s」，点击卡片切换到该插件。", pluginItem["title"])
	payload := map[string]interface{}{
		"parts": []map[string]interface{}{
			{
				"type": "text",
				"text": "检测到这条消息可能属于其他插件，当前插件不会自动执行。",
			},
			{
				"type":         "plugin_card",
				"plugin":       pluginItem,
				"matched_text": msg.Content,
				"action": map[string]string{
					"type":      "switch_plugin",
					"plugin_id": targetPlugin.ID,
					"label":     "切换到此插件",
				},
			},
		},
		"fallback_text": fallback,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	target := adp.SendTarget(msg.UserID, msg.GroupID) + "#plugin_" + strings.TrimSpace(currentPluginID)
	return adp.SendWebChatMessage(target, &config.WebChatMessage{MessageType: "rich", Content: fallback, RichJSON: string(data), PluginID: currentPluginID})
}

func (s *Server) webChatPluginAvailable(userID, adapterID, pluginID string) bool {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" || s.router == nil {
		return false
	}
	plugin := s.router.GetPlugin(pluginID)
	if plugin == nil || !plugin.Enabled || !webChatPluginVisible(plugin) {
		return false
	}
	msg := &types.Message{Platform: config.WebChatPlatform, AdapterID: strings.TrimSpace(adapterID), UserID: strings.TrimSpace(userID), Content: ""}
	return s.webChatPluginSupportsMessage(plugin, msg)
}

func (s *Server) webChatPluginSupportsMessage(plugin *types.Plugin, msg *types.Message) bool {
	if plugin == nil || msg == nil {
		return false
	}
	if len(plugin.Platforms) > 0 && !containsWebChatString(plugin.Platforms, msg.Platform) {
		return false
	}
	if len(plugin.AllowedAdapterIDs) > 0 && !containsWebChatString(plugin.AllowedAdapterIDs, msg.AdapterID) {
		return false
	}
	return true
}

func containsWebChatString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func webChatPluginVisible(plugin *types.Plugin) bool {
	if plugin == nil {
		return false
	}
	if plugin.WebChat.Enabled != nil {
		return *plugin.WebChat.Enabled
	}
	return plugin.Enabled
}

func webChatPluginItem(plugin *types.Plugin, status string) map[string]interface{} {
	webChat := plugin.WebChat
	title := strings.TrimSpace(webChat.Title)
	if title == "" {
		title = plugin.Name
	}
	description := strings.TrimSpace(webChat.Description)
	if description == "" {
		description = "点击进入插件对话"
	}
	placeholder := strings.TrimSpace(webChat.Placeholder)
	if placeholder == "" {
		placeholder = "输入文本消息，按 Ctrl+Enter 发送"
	}
	entryText := strings.TrimSpace(webChat.EntryText)
	quickActions := append([]types.PluginWebChatQuickAction(nil), webChat.QuickActions...)
	if entryText == "" {
		entryText = simpleWebChatTriggerText(plugin.Trigger)
	}
	if len(quickActions) == 0 && entryText != "" {
		quickActions = []types.PluginWebChatQuickAction{{Label: "打开插件", Text: entryText}}
	}
	return map[string]interface{}{
		"id":            plugin.ID,
		"name":          plugin.Name,
		"title":         title,
		"description":   description,
		"trigger":       plugin.Trigger,
		"entry_text":    entryText,
		"placeholder":   placeholder,
		"quick_actions": quickActions,
		"keywords":      webChat.Keywords,
		"enabled":       plugin.Enabled,
		"status":        status,
	}
}

func simpleWebChatTriggerText(trigger string) string {
	trigger = strings.TrimSpace(trigger)
	trigger = strings.TrimPrefix(trigger, "^")
	trigger = strings.TrimSuffix(trigger, "$")
	if trigger == "" || strings.ContainsAny(trigger, `[](){}+*?|\\`) {
		return ""
	}
	return trigger
}

func webChatPluginRunning(status string) bool {
	status = strings.TrimSpace(status)
	return status == "running" || status == "ready" || status == ""
}

var _ adapter.Adapter = (*webadapter.Adapter)(nil)

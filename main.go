package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/deps"
	"github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/router"
	"github.com/allbot/allbot/core/session"
	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/web"
)

//go:embed web/* web/assets/*
var embeddedWeb embed.FS

func main() {
	pluginDir := flag.String("plugins", "./plugins", "插件目录")
	flag.Parse()

	if delay := restartDelayFromEnv(); delay > 0 {
		time.Sleep(delay)
	}

	log.Println("AllBot 启动中...")
	restartChan := make(chan router.RestartRequest, 1)
	var restartRequested atomic.Bool
	requestRestart := func(request router.RestartRequest) error {
		if !restartRequested.CompareAndSwap(false, true) {
			return fmt.Errorf("重启已在执行")
		}
		select {
		case restartChan <- request:
			return nil
		default:
			restartRequested.Store(false)
			return fmt.Errorf("重启已在执行")
		}
	}

	webPort, err := resolveWebPort()
	if err != nil {
		log.Fatalf("Web UI 端口配置错误: %v", err)
	}
	if err := os.MkdirAll(*pluginDir, 0755); err != nil {
		log.Fatalf("创建插件目录失败: %v", err)
	}

	configDB, err := config.NewDatabase("./config.db")
	if err != nil {
		log.Fatalf("初始化配置数据库失败: %v", err)
	}

	adminPasswordInit, err := configDB.EnsureAdminPassword()
	if err != nil {
		log.Fatalf("初始化管理员密码失败: %v", err)
	}
	if err := initDefaultConfig(configDB); err != nil {
		log.Printf("警告：初始化默认配置失败: %v", err)
	}
	startBindCodeCleaner(configDB)

	depsManager := deps.NewManager("./runtime")
	if err := depsManager.InitPythonEnv(); err != nil {
		log.Printf("警告：初始化 Python 环境失败: %v", err)
	}
	if err := depsManager.InitNodeEnv(); err != nil {
		log.Printf("警告：初始化 Node.js 环境失败: %v", err)
	}

	sessionManager := session.NewManager()
	messageRouter := router.NewRouter(sessionManager)
	pluginManager := plugin.NewManager(*pluginDir, depsManager)
	pluginManager.SetDatabase(configDB)
	messageRouter.SetPluginManager(pluginManager)

	plugins, err := pluginManager.LoadAllPlugins()
	if err != nil {
		log.Printf("警告：加载插件失败: %v", err)
	}
	for _, item := range plugins {
		if err := messageRouter.RegisterPlugin(item); err != nil {
			log.Printf("警告：注册插件失败 %s: %v", item.Name, err)
		}
	}
	log.Printf("已注册 %d 个插件", len(plugins))

	adapterManager := config.NewAdapterManager(configDB)
	adapterManager.SetMessageHandler(func(msg *types.Message) {
		messageRouter.HandleMessage(msg)
	})
	if err := adapterManager.LoadAndStartAdapters(); err != nil {
		log.Printf("警告：加载适配器失败: %v", err)
	}
	messageRouter.SetAdapterGetter(adapterManager.GetAdapter)
	messageRouter.SetMessageAdapterGetter(adapterManager.GetAdapterForMessage)
	messageRouter.SetDataViewSaver(configDB.SaveDataView)
	messageRouter.SetDatabase(configDB)
	messageRouter.SetAdminChecker(configDB.IsPlatformAdmin)
	keywordReplyManager := router.NewKeywordReplyManager(configDB, messageRouter.GetAdapterForMessage, configDB.IsPlatformAdmin, time.Now())
	keywordReplyManager.SetRestartHandler(requestRestart)
	messageRouter.SetKeywordReplyManager(keywordReplyManager)
	scheduledTaskRunner := router.NewScheduledTaskRunner(configDB, messageRouter)
	scheduledTaskRunner.Start()
	notifyRestartCompleted(adapterManager)

	webFiles, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		log.Fatalf("初始化内嵌 Web UI 失败: %v", err)
	}
	webServer := web.NewServer(webPort, pluginManager, messageRouter, adapterManager, webFiles)
	if adminPasswordInit.Generated {
		log.Println("首次启动已生成管理员登录密码，请立即登录后修改：")
	} else if adminPasswordInit.Migrated {
		log.Println("已将旧管理员明文密码迁移为哈希存储")
	}
	if adminPasswordInit.GeneratedPassword != "" {
		log.Println("管理员首次自动生成的默认密码如下，修改密码后这里仍显示该默认密码：")
		log.Printf("- 管理员账号: %s", adminPasswordInit.Username)
		log.Printf("- 默认密码: %s", adminPasswordInit.GeneratedPassword)
	}
	log.SetOutput(web.NewCustomLogger(webServer.GetLogManager()))

	go func() {
		if err := webServer.Start(); err != nil {
			log.Printf("Web UI 启动失败: %v", err)
		}
	}()

	log.Println("AllBot 启动成功！")
	log.Printf("- 插件目录: %s", *pluginDir)
	log.Printf("- 已加载插件: %d 个", len(plugins))
	log.Printf("- Web UI: http://localhost:%s", webPort)
	log.Printf("- 管理员账号: %s", adminPasswordInit.Username)

	shutdown := func() {
		log.Println("AllBot 关闭中...")
		scheduledTaskRunner.Stop()
		adapterManager.StopAll()
		for _, item := range pluginManager.GetAllPlugins() {
			pluginManager.StopPlugin(item.Plugin.ID)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := webServer.Shutdown(ctx); err != nil {
			log.Printf("Web UI 关闭失败: %v", err)
		}
		if err := configDB.Close(); err != nil {
			log.Printf("配置数据库关闭失败: %v", err)
		}
		log.Println("AllBot 已关闭")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-sigChan:
			shutdown()
			return
		case request := <-restartChan:
			log.Println("AllBot 准备重启...")
			if err := saveRestartContext(request); err != nil {
				log.Printf("保存重启上下文失败: %v", err)
				restartRequested.Store(false)
				keywordReplyManager.SetRestartHandler(requestRestart)
				continue
			}
			if err := spawnRestartProcess(); err != nil {
				log.Printf("启动重启进程失败: %v", err)
				restartRequested.Store(false)
				keywordReplyManager.SetRestartHandler(requestRestart)
				continue
			}
			shutdown()
			return
		}
	}
}

func saveRestartContext(request router.RestartRequest) error {
	if strings.TrimSpace(request.MessageKey) == "" {
		return errors.New("重启消息唯一标识为空")
	}
	values := map[string]string{
		"ALLBOT_IGNORE_RESTART_MESSAGE_KEY": request.MessageKey,
		"ALLBOT_RESTART_NOTIFY_PLATFORM":    request.Platform,
		"ALLBOT_RESTART_NOTIFY_ADAPTER_ID":  request.AdapterID,
		"ALLBOT_RESTART_NOTIFY_USER_ID":     request.UserID,
		"ALLBOT_RESTART_NOTIFY_GROUP_ID":    request.GroupID,
		"ALLBOT_RESTART_NOTIFY_TARGET":      request.Target,
		"ALLBOT_RESTART_STARTED_AT_NS":      strconv.FormatInt(request.StartedAt.UnixNano(), 10),
	}
	for key, value := range values {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return nil
}

func notifyRestartCompleted(adapterManager *config.AdapterManager) {
	if strings.TrimSpace(os.Getenv("ALLBOT_RESTARTED")) != "1" || adapterManager == nil {
		return
	}
	platform := strings.TrimSpace(os.Getenv("ALLBOT_RESTART_NOTIFY_PLATFORM"))
	userID := strings.TrimSpace(os.Getenv("ALLBOT_RESTART_NOTIFY_USER_ID"))
	groupID := strings.TrimSpace(os.Getenv("ALLBOT_RESTART_NOTIFY_GROUP_ID"))
	target := strings.TrimSpace(os.Getenv("ALLBOT_RESTART_NOTIFY_TARGET"))
	adapterID := strings.TrimSpace(os.Getenv("ALLBOT_RESTART_NOTIFY_ADAPTER_ID"))
	if platform == "" || target == "" {
		return
	}
	msg := &types.Message{Platform: platform, AdapterID: adapterID, UserID: userID, GroupID: groupID, Metadata: map[string]string{}}
	if adapterID != "" {
		msg.Metadata["adapter_id"] = adapterID
	}
	adp := adapterManager.GetAdapterForMessage(msg)
	if adp == nil {
		log.Printf("重启完成通知发送失败：适配器不存在 platform=%s adapter_id=%s", platform, adapterID)
		return
	}
	text := "AllBot 重启完成"
	if startedAt, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("ALLBOT_RESTART_STARTED_AT_NS")), 10, 64); err == nil && startedAt > 0 {
		text = fmt.Sprintf("AllBot 重启完成，耗时：%s", formatRestartDuration(time.Since(time.Unix(0, startedAt))))
	}
	if err := adp.SendMessage(target, text); err != nil {
		log.Printf("重启完成通知发送失败: %v", err)
	}
}

func formatRestartDuration(duration time.Duration) string {
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", duration.Seconds())
}

func restartDelayFromEnv() time.Duration {
	value := strings.TrimSpace(os.Getenv("ALLBOT_RESTART_DELAY_MS"))
	if value == "" {
		return 0
	}
	milliseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || milliseconds <= 0 {
		return 0
	}
	return time.Duration(milliseconds) * time.Millisecond
}

func spawnRestartProcess() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := buildRestartCommand(exe, os.Args[1:], wd)
	return cmd.Start()
}

func buildRestartCommand(exe string, args []string, wd string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	cmd.Dir = wd
	env := append(os.Environ(), "ALLBOT_RESTARTED=1", "ALLBOT_RESTART_DELAY_MS=2000", fmt.Sprintf("ALLBOT_PARENT_PID=%d", os.Getpid()))
	for _, key := range []string{
		"ALLBOT_IGNORE_RESTART_MESSAGE_KEY",
		"ALLBOT_RESTART_NOTIFY_PLATFORM",
		"ALLBOT_RESTART_NOTIFY_ADAPTER_ID",
		"ALLBOT_RESTART_NOTIFY_USER_ID",
		"ALLBOT_RESTART_NOTIFY_GROUP_ID",
		"ALLBOT_RESTART_NOTIFY_TARGET",
		"ALLBOT_RESTART_STARTED_AT_NS",
	} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			env = append(env, key+"="+value)
		}
	}
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func resolveWebPort() (string, error) {
	value := strings.TrimSpace(os.Getenv("ALLBOT_WEB_PORT"))
	if value == "" {
		return "3000", nil
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return "", fmt.Errorf("ALLBOT_WEB_PORT 必须是 1-65535 的有效端口")
		}
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("ALLBOT_WEB_PORT 必须是 1-65535 的有效端口")
	}
	return strconv.Itoa(port), nil
}

func startBindCodeCleaner(db *config.Database) {
	go func() {
		for {
			if err := db.DeleteExpiredUserBindCodes(); err != nil {
				log.Printf("清理过期绑定码失败: %v", err)
			}
			time.Sleep(time.Minute)
		}
	}()
}

func initDefaultConfig(db *config.Database) error {
	existing, err := db.GetAdapter("qq")
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	qqConfig := config.QQConfig{ServerURL: "ws://127.0.0.1:3001"}
	configJSON, err := json.Marshal(qqConfig)
	if err != nil {
		return err
	}

	adapter := &config.AdapterConfig{
		Platform:    "qq",
		Remark:      "默认 QQ",
		Description: "默认创建的 NapCat QQ 适配器配置",
		Enabled:     false,
		Config:      string(configJSON),
	}
	if err := db.SaveAdapter(adapter); err != nil {
		return err
	}
	log.Println("已创建默认 QQ 适配器配置（禁用状态）")
	return nil
}

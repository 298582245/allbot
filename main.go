package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/deps"
	"github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/router"
	"github.com/allbot/allbot/core/session"
	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/web"
)

func main() {
	// 命令行参数
	pluginDir := flag.String("plugins", "./plugins", "插件目录")
	flag.Parse()

	log.Println("AllBot 启动中...")

	// 1. 初始化配置数据库
	configDB, err := config.NewDatabase("./config.db")
	if err != nil {
		log.Fatalf("初始化配置数据库失败: %v", err)
	}
	defer configDB.Close()

	// 初始化默认配置（如果不存在）
	if err := initDefaultConfig(configDB); err != nil {
		log.Printf("警告：初始化默认配置失败: %v", err)
	}

	// 2. 初始化依赖管理器
	depsManager := deps.NewManager("./runtime")

	log.Println("初始化 Python 环境...")
	if err := depsManager.InitPythonEnv(); err != nil {
		log.Printf("警告：初始化 Python 环境失败: %v", err)
		log.Println("Python 插件将无法运行，但其他功能不受影响")
	}

	log.Println("初始化 Node.js 环境...")
	if err := depsManager.InitNodeEnv(); err != nil {
		log.Printf("警告：初始化 Node.js 环境失败: %v", err)
		log.Println("Node.js 插件将无法运行，但其他功能不受影响")
	}

	// 3. 创建会话管理器
	sessionManager := session.NewManager()

	// 4. 创建消息路由器
	messageRouter := router.NewRouter(sessionManager)

	// 5. 创建插件管理器
	pluginManager := plugin.NewManager(*pluginDir, depsManager)

	// 6. 连接路由器和插件管理器
	messageRouter.SetPluginManager(pluginManager)

	// 7. 加载所有插件
	plugins, err := pluginManager.LoadAllPlugins()
	if err != nil {
		log.Printf("警告：加载插件失败: %v", err)
	}

	// 8. 注册插件到路由器（但不启动进程，按需启动）
	for _, p := range plugins {
		if err := messageRouter.RegisterPlugin(p); err != nil {
			log.Printf("警告：注册插件失败 %s: %v", p.Name, err)
		}
	}

	log.Printf("已注册 %d 个插件（按需启动模式）", len(plugins))

	// 9. 创建适配器管理器
	adapterManager := config.NewAdapterManager(configDB)

	// 设置消息处理器
	adapterManager.SetMessageHandler(func(msg *types.Message) {
		messageRouter.HandleMessage(msg)
	})

	// 加载并启动所有启用的适配器
	log.Println("加载平台适配器...")
	if err := adapterManager.LoadAndStartAdapters(); err != nil {
		log.Printf("警告：加载适配器失败: %v", err)
	}

	// 将适配器传递给Router，以便插件可以发送消息
	messageRouter.SetAdapters(adapterManager.GetAllAdapters())

	// 10. 启动 Web UI 服务器
	webServer := web.NewServer("3000", pluginManager, messageRouter, adapterManager, "admin", "admin123")
	go func() {
		if err := webServer.Start(); err != nil {
			log.Printf("Web UI 启动失败: %v", err)
		}
	}()

	log.Println("AllBot 启动成功！")
	log.Printf("- 插件目录: %s", *pluginDir)
	log.Printf("- 已加载插件: %d 个", len(plugins))
	log.Printf("- Web UI: http://localhost:3000")
	log.Printf("- 默认账号: admin / admin123")

	// 11. 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("AllBot 关闭中...")

	// 12. 清理资源
	adapterManager.StopAll()

	// 停止所有插件
	for _, p := range pluginManager.GetAllPlugins() {
		pluginManager.StopPlugin(p.Plugin.ID)
	}

	log.Println("AllBot 已关闭")
}

// initDefaultConfig 初始化默认配置
func initDefaultConfig(db *config.Database) error {
	// 检查是否已有 QQ 配置
	existing, err := db.GetAdapter("qq")
	if err != nil {
		return err
	}

	// 如果不存在，创建默认配置
	if existing == nil {
		qqConfig := config.QQConfig{
			APIURL:     "http://localhost:5700",
			ListenAddr: ":8080",
		}

		configJSON, err := json.Marshal(qqConfig)
		if err != nil {
			return err
		}

		adapter := &config.AdapterConfig{
			Platform: "qq",
			Enabled:  false, // 默认禁用，需要用户在 Web UI 中配置后启用
			Config:   string(configJSON),
		}

		if err := db.SaveAdapter(adapter); err != nil {
			return err
		}

		log.Println("已创建默认 QQ 适配器配置（禁用状态）")
	}

	return nil
}

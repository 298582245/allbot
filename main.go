package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/allbot/allbot/core/adapter"
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
	qqAPIURL := flag.String("qq-api", "http://localhost:5700", "go-cqhttp API 地址")
	flag.Parse()

	log.Println("AllBot 启动中...")

	// 1. 初始化依赖管理器
	depsManager := deps.NewManager("./runtime")

	log.Println("初始化 Python 环境...")
	if err := depsManager.InitPythonEnv(); err != nil {
		log.Fatalf("初始化 Python 环境失败: %v", err)
	}

	log.Println("初始化 Node.js 环境...")
	if err := depsManager.InitNodeEnv(); err != nil {
		log.Fatalf("初始化 Node.js 环境失败: %v", err)
	}

	// 2. 创建会话管理器
	sessionManager := session.NewManager()

	// 3. 创建消息路由器
	messageRouter := router.NewRouter(sessionManager)

	// 4. 创建插件管理器
	pluginManager := plugin.NewManager(*pluginDir, depsManager)

	// 5. 连接路由器和插件管理器
	messageRouter.SetPluginManager(pluginManager)

	// 6. 加载所有插件
	plugins, err := pluginManager.LoadAllPlugins()
	if err != nil {
		log.Printf("警告：加载插件失败: %v", err)
	}

	// 7. 注册插件到路由器并启动插件进程
	for _, p := range plugins {
		if err := messageRouter.RegisterPlugin(p); err != nil {
			log.Printf("警告：注册插件失败 %s: %v", p.Name, err)
			continue
		}

		pluginPath := filepath.Join(*pluginDir, p.ID)
		if err := pluginManager.StartPlugin(p, pluginPath); err != nil {
			log.Printf("警告：启动插件失败 %s: %v", p.Name, err)
			continue
		}
	}

	// 8. 创建平台适配器
		}

		pluginPath := filepath.Join(*pluginDir, p.ID)
		if err := pluginManager.StartPlugin(p, pluginPath); err != nil {
			log.Printf("警告：启动插件失败 %s: %v", p.Name, err)
			continue
		}
	}

	// 6. 创建平台适配器
	qqAdapter := adapter.NewQQAdapter(*qqAPIURL, ":8080")

	// 设置消息处理器
	qqAdapter.SetMessageHandler(func(msg *types.Message) {
		messageRouter.HandleMessage(msg)
	})

	// 启动适配器
	if err := qqAdapter.Start(); err != nil {
		log.Fatalf("启动 QQ 适配器失败: %v", err)
	}

	// 7. 启动 Web UI 服务器
	webServer := web.NewServer("3000", pluginManager, messageRouter, "admin", "admin123")
	go func() {
		if err := webServer.Start(); err != nil {
			log.Printf("Web UI 启动失败: %v", err)
		}
	}()

	log.Println("AllBot 启动成功！")
	log.Printf("- 插件目录: %s", *pluginDir)
	log.Printf("- 已加载插件: %d 个", len(plugins))
	log.Printf("- QQ 适配器: %s", *qqAPIURL)
	log.Printf("- Web UI: http://localhost:3000")
	log.Printf("- 默认账号: admin / admin123")

	// 8. 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("AllBot 关闭中...")

	// 8. 清理资源
	qqAdapter.Stop()

	// 停止所有插件
	for _, p := range pluginManager.GetAllPlugins() {
		pluginManager.StopPlugin(p.Plugin.ID)
	}

	log.Println("AllBot 已关闭")
}

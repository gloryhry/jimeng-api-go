package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gloryhry/jimeng-api-go/internal/api/routes"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/config"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/proxy"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/server"
)

const version = "1.6.3"

func main() {
	startTime := time.Now()

	// 初始化配置
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "初始化配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志系统
	logger.Init(
		config.System.LogDirPath(),
		config.System.LogLevel,
		config.System.Debug,
		config.System.LogWriteInterval,
	)

	// 输出日志头部
	logger.Header()

	// 初始化代理配置
	proxy.Setup()

	logger.Info("<<<< jimeng free server >>>>")
	logger.Info(fmt.Sprintf("Version: %s", version))
	logger.Info(fmt.Sprintf("Process id: %d", os.Getpid()))
	logger.Info(fmt.Sprintf("Environment: %s", config.Environment))
	logger.Info(fmt.Sprintf("Service name: %s", config.Service.Name))

	// 创建服务器
	srv := server.NewServer()

	// 注册路由
	routes.RegisterRoutes(srv.Engine)

	// 启动服务器
	if err := srv.Listen(config.Service.BindAddress()); err != nil {
		logger.Error(fmt.Sprintf("启动服务器失败: %v", err))
		os.Exit(1)
	}

	elapsed := time.Since(startTime).Milliseconds()
	logger.Success(fmt.Sprintf("Service startup completed (%dms)", elapsed))

	// 等待中断信号
	srv.Wait()

	// 输出日志尾部
	logger.Footer()
	logger.Destroy()
}

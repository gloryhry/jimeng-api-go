package routes

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/config"
)

// RegisterRoutes 注册所有路由
func RegisterRoutes(engine *gin.Engine) {
	// 根路径 - 欢迎页面
	engine.GET("/", func(c *gin.Context) {
		welcomeFile := filepath.Join(config.System.PublicDirPath(), "welcome.html")
		content, err := os.ReadFile(welcomeFile)
		if err != nil {
			c.String(500, "Error reading welcome page")
			return
		}
		c.Data(200, "text/html; charset=utf-8", content)
	})

	// Ping - 健康检查
	engine.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	// V1 API 组
	v1 := engine.Group("/v1")
	RegisterImageRoutes(v1)
	RegisterChatRoutes(v1)
	RegisterVideoRoutes(v1)
	RegisterModelRoutes(v1)

	// 非 V1 路由
	RegisterTokenRoutes(engine.Group(""))
}

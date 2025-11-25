package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/config"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
)

// Server HTTP 服务器
type Server struct {
	Engine     *gin.Engine
	httpServer *http.Server
}

// NewServer 创建服务器
func NewServer() *Server {
	// 设置 Gin 模式
	if !config.System.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// 添加中间件
	engine.Use(RecoveryMiddleware())
	engine.Use(CORSMiddleware())
	
	if config.System.RequestLog {
		engine.Use(RequestLogMiddleware())
	}

	logger.Success("Server initialized")

	return &Server{
		Engine: engine,
	}
}

// Listen 启动服务器
func (s *Server) Listen(addr string) error {
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.Engine,
	}

	logger.Info(fmt.Sprintf("Server listening on %s", addr))

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(fmt.Sprintf("Server error: %v", err))
		}
	}()

	return nil
}

// Wait 等待中断信号
func (s *Server) Wait() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		logger.Error(fmt.Sprintf("Server forced to shutdown: %v", err))
	}

	logger.Info("Server exited")
}

// CORSMiddleware CORS 中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// RequestLogMiddleware 请求日志中间件
func RequestLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		logger.Info(fmt.Sprintf("%s %s - %d (%v)", method, path, statusCode, latency))
	}
}

// RecoveryMiddleware 恢复中间件
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error(fmt.Sprintf("Panic recovered: %v", err))
				c.JSON(500, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gloryhry/jimeng-api-go/internal/api/controllers"
)

// RegisterChatRoutes 注册聊天接口
func RegisterChatRoutes(v1 *gin.RouterGroup) {
	v1.POST("/chat/completions", handleChatCompletion)
}

func handleChatCompletion(c *gin.Context) {
	var req struct {
		Model    string                    `json:"model"`
		Messages []controllers.ChatMessage `json:"messages" binding:"required"`
		Stream   bool                      `json:"stream"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := pickToken(c)
	if err != nil {
		return
	}
	if req.Stream {
		stream, err := controllers.CreateCompletionStream(req.Messages, token, req.Model)
		if err != nil {
			respondError(c, err)
			return
		}
		writeSSE(c, stream)
		return
	}
	resp, err := controllers.CreateCompletion(req.Messages, token, req.Model)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func writeSSE(c *gin.Context, stream <-chan string) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器不支持SSE"})
		return
	}
	c.Status(http.StatusOK)
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case chunk, ok := <-stream:
			if !ok {
				return
			}
			if _, err := c.Writer.Write([]byte(chunk)); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

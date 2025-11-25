package routes

import "github.com/gin-gonic/gin"

// RegisterModelRoutes 模型列表
func RegisterModelRoutes(v1 *gin.RouterGroup) {
	v1.GET("/models", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"data": []gin.H{
				{"id": "jimeng", "object": "model", "owned_by": "jimeng-free-api"},
				{"id": "jimeng-video-3.0", "object": "model", "owned_by": "jimeng-free-api", "description": "即梦AI视频生成模型 3.0 版本"},
				{"id": "jimeng-video-3.0-pro", "object": "model", "owned_by": "jimeng-free-api", "description": "即梦AI视频生成模型 3.0 专业版"},
				{"id": "jimeng-video-2.0", "object": "model", "owned_by": "jimeng-free-api", "description": "即梦AI视频生成模型 2.0 版本"},
				{"id": "jimeng-video-2.0-pro", "object": "model", "owned_by": "jimeng-free-api", "description": "即梦AI视频生成模型 2.0 专业版"},
			},
		})
	})
}

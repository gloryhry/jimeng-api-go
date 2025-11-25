package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gloryhry/jimeng-api-go/internal/api/controllers"
)

// RegisterTokenRoutes 注册 token 接口
func RegisterTokenRoutes(router *gin.RouterGroup) {
	group := router.Group("/token")
	group.POST("/check", handleTokenCheck)
	group.POST("/points", handleTokenPoints)
}

func handleTokenCheck(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	live, err := controllers.GetTokenLiveStatus(req.Token)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"live": live})
}

func handleTokenPoints(c *gin.Context) {
	tokens := controllers.TokenSplit(c.GetHeader("Authorization"))
	if len(tokens) == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 Authorization"})
		return
	}
	results := make([]gin.H, 0, len(tokens))
	for _, token := range tokens {
		credit, err := controllers.GetCredit(token)
		if err != nil {
			results = append(results, gin.H{"token": token, "error": err.Error()})
			continue
		}
		results = append(results, gin.H{
			"token":  token,
			"points": credit,
		})
	}
	c.JSON(http.StatusOK, results)
}

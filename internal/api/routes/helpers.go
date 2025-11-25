package routes

import (
	"math/rand"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gloryhry/jimeng-api-go/internal/api/controllers"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/errors"
)

func pickToken(c *gin.Context) (string, error) {
	tokens := controllers.TokenSplit(c.GetHeader("Authorization"))
	if len(tokens) == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 Authorization"})
		return "", errUnauthorized
	}
	idx := rand.Intn(len(tokens))
	return tokens[idx], nil
}

var errUnauthorized = errors.ErrAPIRequestParamsInvalid("missing token")

func respondError(c *gin.Context, err error) {
	if apiErr, ok := err.(*errors.APIException); ok {
		c.JSON(apiErr.HTTPStatusCode(), gin.H{"error": apiErr.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

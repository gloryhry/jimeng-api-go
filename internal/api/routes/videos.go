package routes

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gloryhry/jimeng-api-go/internal/api/controllers"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/utils"
)

// RegisterVideoRoutes 注册视频接口
func RegisterVideoRoutes(v1 *gin.RouterGroup) {
	v1.POST("/videos/generations", handleVideoGeneration)
}

func handleVideoGeneration(c *gin.Context) {
	isMultipart := strings.HasPrefix(c.ContentType(), "multipart/form-data")
	var req struct {
		Model          string   `json:"model"`
		Prompt         string   `json:"prompt" binding:"required"`
		Ratio          string   `json:"ratio"`
		Resolution     string   `json:"resolution"`
		Duration       int      `json:"duration"`
		FilePaths      []string `json:"file_paths"`
		FilePathsAlias []string `json:"filePaths"`
		ResponseFormat string   `json:"response_format"`
	}
	var fileBuffers [][]byte
	if isMultipart {
		if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "上传图片失败"})
			return
		}
		req.Model = c.PostForm("model")
		req.Prompt = c.PostForm("prompt")
		req.Ratio = c.PostForm("ratio")
		req.Resolution = c.PostForm("resolution")
		req.ResponseFormat = c.PostForm("response_format")
		req.Duration = int(parseFloat(c.PostForm("duration")))
		files := c.Request.MultipartForm.File["files"]
		if len(files) == 0 {
			files = c.Request.MultipartForm.File["images"]
		}
		for _, fh := range files {
			buf, err := readFileBytes(fh)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			fileBuffers = append(fileBuffers, buf)
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if len(strings.TrimSpace(req.Prompt)) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt不能为空"})
		return
	}
	token, err := pickToken(c)
	if err != nil {
		return
	}
	paths := req.FilePaths
	if len(paths) == 0 {
		paths = req.FilePathsAlias
	}
	options := &controllers.VideoOptions{
		Ratio:       defaultString(req.Ratio, "1:1"),
		Resolution:  defaultString(req.Resolution, "720p"),
		Duration:    req.Duration,
		FilePaths:   paths,
		FileBuffers: fileBuffers,
	}
	videoURL, err := controllers.GenerateVideo(req.Model, req.Prompt, options, token)
	if err != nil {
		respondError(c, err)
		return
	}
	var data []map[string]string
	if defaultResponseFormat(req.ResponseFormat) == "b64_json" {
		b64, err := utils.FetchFileBASE64(videoURL)
		if err != nil {
			respondError(c, err)
			return
		}
		data = []map[string]string{{"b64_json": b64, "revised_prompt": req.Prompt}}
	} else {
		data = []map[string]string{{"url": videoURL, "revised_prompt": req.Prompt}}
	}
	c.JSON(http.StatusOK, gin.H{"created": utils.UnixTimestamp(), "data": data})
}

func defaultString(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}

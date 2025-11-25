package routes

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gloryhry/jimeng-api-go/internal/api/controllers"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/utils"
)

// RegisterImageRoutes 图片相关接口
func RegisterImageRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/images")
	group.POST("/generations", handleImageGenerations)
	group.POST("/compositions", handleImageCompositions)
	group.POST("/edits", handleImageEdits)
}

func handleImageGenerations(c *gin.Context) {
	var req struct {
		Model            string  `json:"model"`
		Prompt           string  `json:"prompt" binding:"required"`
		Ratio            string  `json:"ratio"`
		Resolution       string  `json:"resolution"`
		IntelligentRatio bool    `json:"intelligent_ratio"`
		SampleStrength   float64 `json:"sample_strength"`
		NegativePrompt   string  `json:"negative_prompt"`
		ResponseFormat   string  `json:"response_format"`
		N                *int    `json:"n"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := pickToken(c)
	if err != nil {
		return
	}
	options := &controllers.ImageOptions{
		Ratio:            req.Ratio,
		Resolution:       req.Resolution,
		SampleStrength:   req.SampleStrength,
		NegativePrompt:   req.NegativePrompt,
		IntelligentRatio: req.IntelligentRatio,
	}
	urls, err := controllers.GenerateImages(req.Model, req.Prompt, options, token)
	if err != nil {
		respondError(c, err)
		return
	}
	data, err := formatImageResponse(urls, req.ResponseFormat, req.N)
	if err != nil {
		respondError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"created": utils.UnixTimestamp(), "data": data})
}

func handleImageCompositions(c *gin.Context) {
	token, err := pickToken(c)
	if err != nil {
		return
	}
	isMultipart := strings.HasPrefix(c.ContentType(), "multipart/form-data")
	var images []interface{}
	var reqBody struct {
		Model            string        `json:"model"`
		Prompt           string        `json:"prompt" binding:"required"`
		NegativePrompt   string        `json:"negative_prompt"`
		Ratio            string        `json:"ratio"`
		Resolution       string        `json:"resolution"`
		SampleStrength   float64       `json:"sample_strength"`
		IntelligentRatio bool          `json:"intelligent_ratio"`
		ResponseFormat   string        `json:"response_format"`
		Images           []interface{} `json:"images"`
	}
	if isMultipart {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "上传图片失败"})
			return
		}
		files := c.Request.MultipartForm.File["images"]
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "至少提供1张图片"})
			return
		}
		for _, fh := range files {
			buf, err := readFileBytes(fh)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			images = append(images, buf)
		}
		reqBody.Model = c.PostForm("model")
		reqBody.Prompt = c.PostForm("prompt")
		reqBody.NegativePrompt = c.PostForm("negative_prompt")
		reqBody.Ratio = c.PostForm("ratio")
		reqBody.Resolution = c.PostForm("resolution")
		reqBody.ResponseFormat = c.PostForm("response_format")
		reqBody.SampleStrength = parseFloat(c.PostForm("sample_strength"))
		reqBody.IntelligentRatio = parseBool(c.PostForm("intelligent_ratio"))
	} else {
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(reqBody.Images) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "至少提供1张图片"})
			return
		}
		images = reqBody.Images
	}
	options := &controllers.ImageOptions{
		Ratio:            reqBody.Ratio,
		Resolution:       reqBody.Resolution,
		SampleStrength:   reqBody.SampleStrength,
		NegativePrompt:   reqBody.NegativePrompt,
		IntelligentRatio: reqBody.IntelligentRatio,
	}
	urls, err := controllers.GenerateImageComposition(reqBody.Model, reqBody.Prompt, images, options, token)
	if err != nil {
		respondError(c, err)
		return
	}
	data, err := formatImageResponse(urls, reqBody.ResponseFormat, nil)
	if err != nil {
		respondError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"created": utils.UnixTimestamp(), "data": data, "input_images": len(images)})
}

func handleImageEdits(c *gin.Context) {
	token, err := pickToken(c)
	if err != nil {
		return
	}
	isMultipart := strings.HasPrefix(c.ContentType(), "multipart/form-data")
	var images []interface{}
	var reqBody struct {
		Model          string        `json:"model"`
		Prompt         interface{}   `json:"prompt"`
		Size           string        `json:"size"`
		Quality        string        `json:"quality"`
		NegativePrompt string        `json:"negative_prompt"`
		SampleStrength float64       `json:"sample_strength"`
		ResponseFormat string        `json:"response_format"`
		Images         []interface{} `json:"images"`
	}
	if isMultipart {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "上传图片失败"})
			return
		}
		files := c.Request.MultipartForm.File["image"]
		if len(files) == 0 {
			files = c.Request.MultipartForm.File["images"]
		}
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少图片"})
			return
		}
		for _, fh := range files {
			buf, err := readFileBytes(fh)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			images = append(images, buf)
		}
		reqBody.Model = c.PostForm("model")
		reqBody.Prompt = c.PostForm("prompt")
		reqBody.Size = c.PostForm("size")
		reqBody.Quality = c.PostForm("quality")
		reqBody.ResponseFormat = c.PostForm("response_format")
		reqBody.SampleStrength = parseFloat(c.PostForm("sample_strength"))
		reqBody.NegativePrompt = c.PostForm("negative_prompt")
	} else {
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		images = reqBody.Images
	}
	if len(images) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少提供1张图片"})
		return
	}
	mapped := mapOpenAIParams(struct {
		Model          string
		Prompt         interface{}
		Size           string
		Quality        string
		NegativePrompt string
		SampleStrength float64
		ResponseFormat string
		Images         []interface{}
	}{
		Model:          reqBody.Model,
		Prompt:         reqBody.Prompt,
		Size:           reqBody.Size,
		Quality:        reqBody.Quality,
		NegativePrompt: reqBody.NegativePrompt,
		SampleStrength: reqBody.SampleStrength,
		ResponseFormat: reqBody.ResponseFormat,
		Images:         reqBody.Images,
	})
	urls, err := controllers.GenerateImageEdits(mapped.Model, mapped.Prompt, images, &controllers.ImageOptions{
		Ratio:          mapped.Ratio,
		Resolution:     mapped.Resolution,
		SampleStrength: mapped.SampleStrength,
		NegativePrompt: reqBody.NegativePrompt,
	}, token)
	if err != nil {
		respondError(c, err)
		return
	}
	data, err := formatImageResponse(urls, mapped.ResponseFormat, mapped.Count)
	if err != nil {
		respondError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"created": utils.UnixTimestamp(), "data": data})
}

func mapOpenAIParams(body struct {
	Model          string
	Prompt         interface{}
	Size           string
	Quality        string
	NegativePrompt string
	SampleStrength float64
	ResponseFormat string
	Images         []interface{}
}) struct {
	Model          string
	Prompt         string
	Ratio          string
	Resolution     string
	SampleStrength float64
	ResponseFormat string
	Count          *int
} {
	prompt := normalizePrompt(body.Prompt)
	if body.NegativePrompt != "" {
		prompt = fmt.Sprintf("%s negative_prompt: %s", prompt, body.NegativePrompt)
	}
	ratio := mapSizeToRatio(body.Size)
	resolution := mapQualityToResolution(body.Quality)
	return struct {
		Model          string
		Prompt         string
		Ratio          string
		Resolution     string
		SampleStrength float64
		ResponseFormat string
		Count          *int
	}{
		Model:          body.Model,
		Prompt:         prompt,
		Ratio:          ratio,
		Resolution:     resolution,
		SampleStrength: body.SampleStrength,
		ResponseFormat: defaultResponseFormat(body.ResponseFormat),
	}
}

func formatImageResponse(urls []string, format string, limit *int) ([]map[string]string, error) {
	if limit != nil && *limit > 0 && *limit < len(urls) {
		urls = urls[:*limit]
	}
	format = defaultResponseFormat(format)
	data := make([]map[string]string, 0, len(urls))
	if format == "b64_json" {
		for _, url := range urls {
			b64, err := utils.FetchFileBASE64(url)
			if err != nil {
				return nil, err
			}
			data = append(data, map[string]string{"b64_json": b64})
		}
	} else {
		for _, url := range urls {
			data = append(data, map[string]string{"url": url})
		}
	}
	return data, nil
}

func defaultResponseFormat(v string) string {
	if v == "b64_json" {
		return v
	}
	return "url"
}

func readFileBytes(fh *multipart.FileHeader) ([]byte, error) {
	file, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func parseFloat(value string) float64 {
	if value == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(value, 64)
	return f
}

func parseBool(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "1"
}

func mapSizeToRatio(size string) string {
	switch size {
	case "1024x1024":
		return "1:1"
	case "1536x1024":
		return "3:2"
	case "1024x1536":
		return "2:3"
	case "auto":
		return "16:9"
	default:
		return "1:1"
	}
}

func mapQualityToResolution(quality string) string {
	switch strings.ToLower(quality) {
	case "high":
		return "1k"
	case "low":
		return "4k"
	default:
		return "2k"
	}
}

func normalizePrompt(prompt interface{}) string {
	switch value := prompt.(type) {
	case string:
		return value
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			parts = append(parts, normalizePrompt(item))
		}
		return strings.Join(parts, " ")
	default:
		return fmt.Sprintf("%v", prompt)
	}
}

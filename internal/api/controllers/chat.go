package controllers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gloryhry/jimeng-api-go/internal/api/consts"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/errors"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/utils"
)

const defaultChatModel = defaultImageModel

var chatModelSizePattern = regexp.MustCompile(`(\d+)[^\d]+(\d+)`)

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatModelPayload 记录模型和尺寸
type chatModelPayload struct {
	Original string
	Model    string
	Width    int
	Height   int
}

// DisplayModel 返回响应里展示的模型名称
func (p chatModelPayload) DisplayModel() string {
	if strings.TrimSpace(p.Original) != "" {
		return p.Original
	}
	return p.Model
}

// CreateCompletion 同步补全
func CreateCompletion(messages []ChatMessage, refreshToken string, model string) (map[string]interface{}, error) {
	if len(messages) == 0 {
		return nil, errors.ErrAPIRequestParamsInvalid("消息不能为空")
	}
	if model == "" {
		model = defaultChatModel
	}
	payload := parseChatModel(model)
	logger.Info(fmt.Sprintf("Chat completion messages: %+v", messages))
	prompt := strings.TrimSpace(messages[len(messages)-1].Content)
	return createCompletionWithRetry(payload, prompt, refreshToken, 0)
}

// CreateCompletionStream 流式补全
func CreateCompletionStream(messages []ChatMessage, refreshToken string, model string) (chan string, error) {
	if len(messages) == 0 {
		return nil, errors.ErrAPIRequestParamsInvalid("消息不能为空")
	}
	if model == "" {
		model = defaultChatModel
	}
	payload := parseChatModel(model)
	logger.Info(fmt.Sprintf("Chat completion(stream) messages: %+v", messages))
	prompt := strings.TrimSpace(messages[len(messages)-1].Content)

	stream := make(chan string, 8)
	go func() {
		defer close(stream)
		if isVideoModel(payload.Original) {
			streamVideoCompletion(stream, payload, prompt, refreshToken)
		} else {
			streamImageCompletion(stream, payload, prompt, refreshToken)
		}
	}()
	return stream, nil
}

func createCompletionWithRetry(payload chatModelPayload, prompt string, refreshToken string, attempt int) (map[string]interface{}, error) {
	response, err := createCompletionOnce(payload, prompt, refreshToken)
	if err == nil {
		return response, nil
	}
	if attempt < consts.MaxRetryCount {
		logger.Warn(fmt.Sprintf("聊天补全失败 (尝试 %d/%d): %v", attempt+1, consts.MaxRetryCount+1, err))
		time.Sleep(time.Duration(consts.RetryDelay) * time.Millisecond)
		return createCompletionWithRetry(payload, prompt, refreshToken, attempt+1)
	}
	return nil, err
}

func createCompletionOnce(payload chatModelPayload, prompt string, refreshToken string) (map[string]interface{}, error) {
	if isVideoModel(payload.Original) {
		modelName := payload.Original
		if strings.TrimSpace(modelName) == "" {
			modelName = payload.Model
		}
		videoURL, err := GenerateVideo(modelName, prompt, &VideoOptions{
			Ratio:      "1:1",
			Resolution: "720p",
			Duration:   5,
		}, refreshToken)
		if err != nil {
			if _, ok := err.(*errors.APIException); ok {
				return nil, err
			}
			message := fmt.Sprintf("生成视频失败: %v\n\n如果您在即梦官网看到已生成的视频，可能是获取结果时出现了问题，请前往即梦官网查看。", err)
			return chatResponse(payload.DisplayModel(), message), nil
		}
		return chatResponse(payload.DisplayModel(), fmt.Sprintf("![video](%s)\n", videoURL)), nil
	}

	images, err := GenerateImages(payload.Model, prompt, &ImageOptions{}, refreshToken)
	if err != nil {
		return nil, err
	}
	var message strings.Builder
	for idx, url := range images {
		message.WriteString(fmt.Sprintf("![image_%d](%s)\n", idx, url))
	}
	return chatResponse(payload.DisplayModel(), message.String()), nil
}

func streamImageCompletion(stream chan<- string, payload chatModelPayload, prompt string, refreshToken string) {
	done := make(chan struct{})
	defer close(done)

	sendStreamChunk(stream, done, buildChunk(payload.DisplayModel(), 0, "assistant", "🎨 图像生成中，请稍候...", nil))

	images, err := GenerateImages(payload.Model, prompt, &ImageOptions{}, refreshToken)
	if err != nil {
		logger.Error(fmt.Sprintf("图像生成失败: %v", err))
		sendStreamChunk(stream, done, buildChunk(payload.DisplayModel(), 1, "assistant", fmt.Sprintf("生成图片失败: %v", err), "stop"))
		sendStreamDone(stream, done)
		return
	}

	for idx, url := range images {
		finish := interface{}(nil)
		if idx == len(images)-1 {
			finish = "stop"
		}
		sendStreamChunk(stream, done, buildChunk(payload.DisplayModel(), idx+1, "assistant", fmt.Sprintf("![image_%d](%s)\n", idx, url), finish))
	}

	sendStreamChunk(stream, done, buildChunk(payload.DisplayModel(), len(images)+1, "assistant", "图像生成完成！", "stop"))
	sendStreamDone(stream, done)
}

func streamVideoCompletion(stream chan<- string, payload chatModelPayload, prompt string, refreshToken string) {
	done := make(chan struct{})
	defer close(done)

	displayModel := payload.DisplayModel()
	sendStreamChunk(stream, done, buildChunk(displayModel, 0, "assistant", "🎬 视频生成中，请稍候...\n这可能需要1-2分钟，请耐心等待", nil))

	progressTicker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-progressTicker.C:
				sendStreamChunk(stream, done, buildChunk(displayModel, 0, "assistant", ".", nil))
			}
		}
	}()

	timeoutTimer := time.AfterFunc(2*time.Minute, func() {
		message := "\n\n视频生成时间较长（已等待2分钟），但视频可能仍在生成中。\n\n请前往即梦官网查看您的视频：\n1. 访问 https://jimeng.jianying.com/ai-tool/video/generate\n2. 登录后查看您的创作历史\n3. 如果视频已生成，您可以直接在官网下载或分享\n\n您也可以继续等待，系统将在后台继续尝试获取视频（最长约20分钟）。"
		sendStreamChunk(stream, done, buildChunk(displayModel, 1, "assistant", message, "stop"))
	})
	defer func() {
		progressTicker.Stop()
		timeoutTimer.Stop()
	}()

	sendStreamChunk(stream, done, buildChunk(displayModel, 0, "assistant", "\n\n🎬 视频生成已开始，这可能需要几分钟时间...", nil))

	modelName := payload.Original
	if strings.TrimSpace(modelName) == "" {
		modelName = payload.Model
	}

	videoURL, err := GenerateVideo(modelName, prompt, &VideoOptions{
		Ratio:      "1:1",
		Resolution: "720p",
		Duration:   5,
	}, refreshToken)
	if err != nil {
		logger.Error(fmt.Sprintf("视频生成失败: %v", err))
		errorMessage := formatVideoErrorMessage(err)
		sendStreamChunk(stream, done, buildChunk(displayModel, 1, "assistant", "\n\n"+errorMessage, "stop"))
		sendStreamDone(stream, done)
		return
	}

	success := fmt.Sprintf("\n\n✅ 视频生成完成！\n\n![video](%s)\n\n您可以：\n1. 直接查看上方视频\n2. 使用以下链接下载或分享：%s", videoURL, videoURL)
	sendStreamChunk(stream, done, buildChunk(displayModel, 1, "assistant", success, nil))
	sendStreamChunk(stream, done, buildChunk(displayModel, 2, "assistant", "", "stop"))
	sendStreamDone(stream, done)
}

func parseChatModel(model string) chatModelPayload {
	payload := chatModelPayload{
		Original: model,
		Model:    defaultChatModel,
		Width:    1024,
		Height:   1024,
	}
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		payload.Original = defaultChatModel
		trimmed = defaultChatModel
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		payload.Model = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		matches := chatModelSizePattern.FindStringSubmatch(parts[1])
		if len(matches) == 3 {
			if width, err := strconv.Atoi(matches[1]); err == nil && width > 0 {
				payload.Width = ensureEven(width)
			}
			if height, err := strconv.Atoi(matches[2]); err == nil && height > 0 {
				payload.Height = ensureEven(height)
			}
		}
	}
	return payload
}

func ensureEven(value int) int {
	if value%2 != 0 {
		return value + 1
	}
	return value
}

func isVideoModel(model string) bool {
	target := strings.ToLower(strings.TrimSpace(model))
	if target == "" {
		return false
	}
	return strings.HasPrefix(target, "jimeng-video")
}

func chatResponse(model, message string) map[string]interface{} {
	return map[string]interface{}{
		"id":      utils.UUID(true),
		"model":   model,
		"object":  "chat.completion",
		"created": utils.UnixTimestamp(),
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": message,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     1,
			"completion_tokens": len(message),
			"total_tokens":      len(message) + 1,
		},
	}
}

func buildChunk(model string, index int, role, content string, finishReason interface{}) string {
	chunk := map[string]interface{}{
		"id":      utils.UUID(true),
		"model":   model,
		"object":  "chat.completion.chunk",
		"created": utils.UnixTimestamp(),
		"choices": []map[string]interface{}{
			{
				"index": index,
				"delta": map[string]string{
					"role":    role,
					"content": content,
				},
				"finish_reason": finishReason,
			},
		},
	}
	body, _ := json.Marshal(chunk)
	return fmt.Sprintf("data: %s\n\n", string(body))
}

func sendStreamChunk(stream chan<- string, done <-chan struct{}, payload string) {
	select {
	case <-done:
		return
	case stream <- payload:
	}
}

func sendStreamDone(stream chan<- string, done <-chan struct{}) {
	sendStreamChunk(stream, done, "data: [DONE]\n\n")
}

func formatVideoErrorMessage(err error) string {
	message := fmt.Sprintf("⚠️ 视频生成过程中遇到问题: %v", err)
	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "历史记录不存在"):
		message += "\n\n可能原因：\n1. 视频生成请求已发送，但API无法获取历史记录\n2. 视频生成服务暂时不可用\n3. 历史记录ID无效或已过期\n\n建议操作：\n1. 请前往即梦官网查看您的视频是否已生成：https://jimeng.jianying.com/ai-tool/video/generate\n2. 如果官网已显示视频，但这里无法获取，可能是API连接问题\n3. 如果官网也没有显示，请稍后再试或重新生成视频"
	case strings.Contains(errStr, "获取视频生成结果超时"):
		message += "\n\n视频生成可能仍在进行中，但等待时间已超过系统设定的限制。\n\n请前往即梦官网查看您的视频：https://jimeng.jianying.com/ai-tool/video/generate\n\n如果您在官网上看到视频已生成，但这里无法显示，可能是因为：\n1. 获取结果的过程超时\n2. 网络连接问题\n3. API访问限制"
	default:
		message += "\n\n如果您在即梦官网看到已生成的视频，可能是获取结果时出现了问题。\n\n请访问即梦官网查看您的创作历史：https://jimeng.jianying.com/ai-tool/video/generate"
	}
	return message
}

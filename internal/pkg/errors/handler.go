package errors

import (
	stderrs "errors"
	"fmt"
	"net"
	"time"

	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
)

// JimengErrorResponse 即梦API错误响应接口
type JimengErrorResponse struct {
	Ret       string      `json:"ret"`
	ErrMsg    string      `json:"errmsg"`
	Data      interface{} `json:"data,omitempty"`
	HistoryID string      `json:"historyId,omitempty"`
}

// ErrorHandlerOptions 错误处理选项
type ErrorHandlerOptions struct {
	Context    string
	HistoryID  string
	RetryCount int
	MaxRetries int
	Operation  string
}

// HTTPStatusError 表示 HTTP 状态码错误
type HTTPStatusError struct {
	Status int
	URL    string
	Body   string
}

// Error 实现 error 接口
func (e *HTTPStatusError) Error() string {
	if e.URL != "" {
		return fmt.Sprintf("HTTP %d: %s", e.Status, e.URL)
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}

// HandleAPIResponse 处理即梦API响应错误
func HandleAPIResponse(response *JimengErrorResponse, options *ErrorHandlerOptions) error {
	if options == nil {
		options = &ErrorHandlerOptions{}
	}

	ret := response.Ret
	errmsg := response.ErrMsg
	historyID := response.HistoryID

	context := options.Context
	if context == "" {
		context = "即梦API请求"
	}

	operation := options.Operation
	if operation == "" {
		operation = "操作"
	}

	logger.Error(fmt.Sprintf("%s失败: ret=%s, errmsg=%s%s",
		context, ret, errmsg,
		func() string {
			if historyID != "" {
				return fmt.Sprintf(", historyId=%s", historyID)
			}
			return ""
		}()))

	// 根据错误码分类处理
	switch ret {
	case "1015":
		return ErrAPITokenExpires(fmt.Sprintf("[登录失效]: %s。请重新获取refresh_token并更新配置", errmsg))

	case "5000":
		return ErrAPIImageGenerationInsufficientPoints(
			fmt.Sprintf("[积分不足]: %s。建议：1)尝试使用1024x1024分辨率，2)检查是否需要购买积分，3)确认账户状态正常", errmsg))

	case "4001":
		return ErrAPIContentFiltered(fmt.Sprintf("[内容违规]: %s", errmsg)).SetHTTPStatusCode(400)

	case "4002":
		return ErrAPIRequestParamsInvalid(fmt.Sprintf("[参数错误]: %s", errmsg)).SetHTTPStatusCode(400)

	case "5001":
		return ErrAPIImageGenerationFailed(fmt.Sprintf("[生成失败]: %s", errmsg))

	case "5002":
		return ErrAPIVideoGenerationFailed(fmt.Sprintf("[视频生成失败]: %s", errmsg))

	default:
		return ErrAPIRequestFailed(fmt.Sprintf("[%s失败]: %s (错误码: %s)", operation, errmsg, ret)).SetHTTPStatusCode(429)
	}
}

// HandleNetworkError 处理网络请求错误
func HandleNetworkError(err error, options *ErrorHandlerOptions) error {
	if options == nil {
		options = &ErrorHandlerOptions{}
	}

	context := options.Context
	if context == "" {
		context = "网络请求"
	}

	retryCount := options.RetryCount
	maxRetries := options.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	logger.Error(fmt.Sprintf("%s网络错误 (尝试 %d/%d): %v", context, retryCount+1, maxRetries+1, err))

	var netErr net.Error
	if stderrs.As(err, &netErr) {
		if netErr.Timeout() {
			return ErrAPIRequestFailed(fmt.Sprintf("[请求超时]: %s超时，请稍后重试", context))
		}
		if !netErr.Timeout() {
			return ErrAPIRequestFailed(fmt.Sprintf("[网络错误]: %s失败 (%v)", context, netErr))
		}
	}

	if statusErr, ok := err.(*HTTPStatusError); ok {
		switch {
		case statusErr.Status == 429:
			return ErrAPIRequestFailed("[请求频率限制]: 请求过于频繁，请稍后重试").SetHTTPStatusCode(429)
		case statusErr.Status == 404:
			return ErrAPIRequestFailed("[资源不存在]: 目标接口不可用").SetHTTPStatusCode(404)
		case statusErr.Status >= 500:
			return ErrAPIRequestFailed(fmt.Sprintf("[服务器错误]: 即梦服务器暂时不可用 (%d)", statusErr.Status)).SetHTTPStatusCode(500)
		case statusErr.Status >= 400:
			return ErrAPIRequestFailed(fmt.Sprintf("[请求错误]: HTTP %d", statusErr.Status)).SetHTTPStatusCode(statusErr.Status)
		}
	}

	return ErrAPIRequestFailed(fmt.Sprintf("[%s失败]: %v", context, err))
}

// HandlePollingTimeout 处理轮询超时错误
func HandlePollingTimeout(pollCount, maxPollCount int, elapsedTime float64, status, itemCount int, historyID string) error {
	message := fmt.Sprintf("轮询超时: 已轮询 %d 次，耗时 %.0f 秒，最终状态: %d，图片数量: %d",
		pollCount, elapsedTime, status, itemCount)

	if historyID != "" {
		message += fmt.Sprintf("，历史ID: %s", historyID)
	}

	logger.Warn(message)

	if itemCount == 0 {
		return ErrAPIImageGenerationFailed(
			fmt.Sprintf("生成超时且无结果，状态码: %d%s",
				status,
				func() string {
					if historyID != "" {
						return fmt.Sprintf("，历史ID: %s", historyID)
					}
					return ""
				}()))
	}

	// 如果有部分结果，不抛出异常
	logger.Info(fmt.Sprintf("轮询超时但已获得 %d 张图片，将返回现有结果", itemCount))
	return nil
}

// HandleGenerationFailure 处理生成失败错误
func HandleGenerationFailure(status int, failCode string, historyID string, itemType string) error {
	if itemType == "" {
		itemType = "image"
	}

	var typeText string
	var exception func(string) *APIException

	if itemType == "video" {
		typeText = "视频"
		exception = ErrAPIVideoGenerationFailed
	} else {
		typeText = "图像"
		exception = ErrAPIImageGenerationFailed
	}

	message := fmt.Sprintf("%s生成最终失败: status=%d, failCode=%s%s",
		typeText, status, failCode,
		func() string {
			if historyID != "" {
				return fmt.Sprintf(", historyId=%s", historyID)
			}
			return ""
		}())

	logger.Error(message)

	return exception(fmt.Sprintf("%s生成失败，状态码: %d%s",
		typeText, status,
		func() string {
			if failCode != "" {
				return fmt.Sprintf("，错误码: %s", failCode)
			}
			return ""
		}()))
}

// WithRetry 包装重试逻辑
func WithRetry(operation func() error, options *ErrorHandlerOptions) error {
	if options == nil {
		options = &ErrorHandlerOptions{}
	}

	maxRetries := options.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	retryDelay := 5 * time.Second
	context := options.Context
	if context == "" {
		context = "操作"
	}

	var lastError error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastError = err

		// 如果是 APIException，直接返回，不重试
		if _, ok := err.(*APIException); ok {
			return err
		}

		if attempt < maxRetries {
			logger.Warn(fmt.Sprintf("%s失败 (尝试 %d/%d): %v", context, attempt+1, maxRetries+1, err))
			logger.Info(fmt.Sprintf("%.0f秒后重试...", retryDelay.Seconds()))
			time.Sleep(retryDelay)
		}
	}

	// 所有重试都失败了
	return HandleNetworkError(lastError, &ErrorHandlerOptions{
		Context:    context,
		RetryCount: maxRetries,
		MaxRetries: maxRetries,
		Operation:  options.Operation,
	})
}

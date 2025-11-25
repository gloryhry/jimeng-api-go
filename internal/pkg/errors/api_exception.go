package errors

import "github.com/gloryhry/jimeng-api-go/internal/api/consts"

// APIException API 异常类型
type APIException struct {
	*Exception
}

// NewAPIException 创建 API 异常
func NewAPIException(code, message string) *APIException {
	return &APIException{
		Exception: NewException(code, message),
	}
}

// SetHTTPStatusCode 设置 HTTP 状态码
func (e *APIException) SetHTTPStatusCode(status int) *APIException {
	e.Exception.SetHTTPStatusCode(status)
	return e
}

// WithCause 设置原因错误
func (e *APIException) WithCause(cause error) *APIException {
	e.Exception.WithCause(cause)
	return e
}

// 预定义的 API 异常
var (
	ErrAPIRequestFailed = func(message string) *APIException {
		return NewAPIException(consts.ExceptionAPIRequestFailed, message)
	}

	ErrAPIRequestParamsInvalid = func(message string) *APIException {
		return NewAPIException(consts.ExceptionAPIRequestParamsInvalid, message)
	}

	ErrAPIContentFiltered = func(message string) *APIException {
		return NewAPIException(consts.ExceptionAPIContentFiltered, message)
	}

	ErrAPITokenExpires = func(message string) *APIException {
		return NewAPIException(consts.ExceptionAPITokenExpires, message).SetHTTPStatusCode(401)
	}

	ErrAPIImageGenerationFailed = func(message string) *APIException {
		return NewAPIException(consts.ExceptionAPIImageGenerationFailed, message).SetHTTPStatusCode(429)
	}

	ErrAPIVideoGenerationFailed = func(message string) *APIException {
		return NewAPIException(consts.ExceptionAPIVideoGenerationFailed, message).SetHTTPStatusCode(429)
	}

	ErrAPIImageGenerationInsufficientPoints = func(message string) *APIException {
		return NewAPIException(consts.ExceptionAPIImageGenerationInsufficientPoints, message).SetHTTPStatusCode(429)
	}

	ErrFileUploadFailed = func(message string) *APIException {
		return NewAPIException(consts.ExceptionFileUploadFailed, message)
	}
)

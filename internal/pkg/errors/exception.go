package errors

import "fmt"

// Exception 基础异常类型
type Exception struct {
	code       string
	message    string
	httpStatus int
	cause      error
}

// NewException 创建新的异常
func NewException(code, message string) *Exception {
	return &Exception{
		code:       code,
		message:    message,
		httpStatus: 500,
	}
}

// Error 实现 error 接口
func (e *Exception) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.code, e.message)
}

// Code 获取异常代码
func (e *Exception) Code() string {
	return e.code
}

// Message 获取异常消息
func (e *Exception) Message() string {
	return e.message
}

// HTTPStatusCode 获取 HTTP 状态码
func (e *Exception) HTTPStatusCode() int {
	return e.httpStatus
}

// SetHTTPStatusCode 设置 HTTP 状态码
func (e *Exception) SetHTTPStatusCode(status int) *Exception {
	e.httpStatus = status
	return e
}

// WithCause 设置原因错误
func (e *Exception) WithCause(cause error) *Exception {
	e.cause = cause
	return e
}

// Cause 获取原因错误
func (e *Exception) Cause() error {
	return e.cause
}

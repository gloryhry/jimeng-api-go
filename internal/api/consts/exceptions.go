package consts

// 异常代码常量
const (
	// 通用异常
	ExceptionUnknown          = "UNKNOWN"
	ExceptionRequest          = "REQUEST"
	ExceptionRequestParams    = "REQUEST_INVALID_PARAMS"
	ExceptionResponse         = "RESPONSE"
	ExceptionResponseType     = "RESPONSE_TYPE_INVALID"

	// API 异常
	ExceptionAPIRequestFailed                = "API_REQUEST_FAILED"
	ExceptionAPIRequestParamsInvalid         = "API_REQUEST_PARAMS_INVALID"
	ExceptionAPIContentFiltered              = "API_CONTENT_FILTERED"
	ExceptionAPITokenExpires                 = "API_TOKEN_EXPIRES"
	ExceptionAPIImageGenerationFailed        = "API_IMAGE_GENERATION_FAILED"
	ExceptionAPIVideoGenerationFailed        = "API_VIDEO_GENERATION_FAILED"
	ExceptionAPIImageGenerationInsufficientPoints = "API_IMAGE_GENERATION_INSUFFICIENT_POINTS"
	ExceptionAPIVideoGenerationInsufficientPoints = "API_VIDEO_GENERATION_INSUFFICIENT_POINTS"
	
	// 文件异常
	ExceptionFileNotFound    = "FILE_NOT_FOUND"
	ExceptionFileInvalidType = "FILE_INVALID_TYPE"
	ExceptionFileUploadFailed = "FILE_UPLOAD_FAILED"
)

// 异常消息映射
var ExceptionMessages = map[string]string{
	ExceptionUnknown:                           "未知错误",
	ExceptionRequest:                           "请求错误",
	ExceptionRequestParams:                     "请求参数错误",
	ExceptionResponse:                          "响应错误",
	ExceptionResponseType:                      "响应类型错误",
	ExceptionAPIRequestFailed:                  "API请求失败",
	ExceptionAPIRequestParamsInvalid:           "API请求参数无效",
	ExceptionAPIContentFiltered:                "内容被过滤",
	ExceptionAPITokenExpires:                   "令牌已过期",
	ExceptionAPIImageGenerationFailed:          "图像生成失败",
	ExceptionAPIVideoGenerationFailed:          "视频生成失败",
	ExceptionAPIImageGenerationInsufficientPoints: "图像生成积分不足",
	ExceptionAPIVideoGenerationInsufficientPoints: "视频生成积分不足",
	ExceptionFileNotFound:                      "文件未找到",
	ExceptionFileInvalidType:                   "文件类型无效",
	ExceptionFileUploadFailed:                  "文件上传失败",
}

package controllers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gloryhry/jimeng-api-go/internal/api/consts"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/errors"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/utils"
	"github.com/go-resty/resty/v2"
)

var (
	DeviceID = rand.Int63n(999999999999999999) + 7000000000000000000
	WebID    = rand.Int63n(999999999999999999) + 7000000000000000000
	UserID   = utils.UUID(false)
)

const fileMaxSize = 100 * 1024 * 1024

// 伪装 headers
var FakeHeaders = map[string]string{
	"Accept":             "application/json, text/plain, */*",
	"Accept-Encoding":    "gzip, deflate, br, zstd",
	"Accept-Language":    "zh-CN,zh;q=0.9",
	"Cache-Control":      "no-cache",
	"Last-Event-Id":      "undefined",
	"Appvr":              consts.VersionCode,
	"Pragma":             "no-cache",
	"Priority":           "u=1, i",
	"Pf":                 consts.PlatformCode,
	"Sec-Ch-Ua":          "\"Google Chrome\";v=\"142\", \"Chromium\";v=\"142\", \"Not_A Brand\";v=\"24\"",
	"Sec-Ch-Ua-Mobile":   "?0",
	"Sec-Ch-Ua-Platform": "\"Windows\"",
	"Sec-Fetch-Dest":     "empty",
	"Sec-Fetch-Mode":     "cors",
	"Sec-Fetch-Site":     "same-origin",
	"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
}

// RegionInfo 暴露区域信息类型
type RegionInfo = utils.RegionInfo

// RequestOptions HTTP 请求配置
type RequestOptions struct {
	Headers         map[string]string
	Params          map[string]interface{}
	Body            interface{}
	Timeout         time.Duration
	NoDefaultParams bool
}

// CreditInfo 积分详情
type CreditInfo struct {
	GiftCredit     int64 `json:"gift_credit"`
	PurchaseCredit int64 `json:"purchase_credit"`
	VipCredit      int64 `json:"vip_credit"`
	TotalCredit    int64 `json:"total_credit"`
}

// UploadFileResult 上传文件结果
type UploadFileResult struct {
	ImageURI string `json:"image_uri"`
	URI      string `json:"uri"`
}

// AcquireToken 去除地区前缀
func AcquireToken(refreshToken string) string {
	return utils.RemoveRegionPrefix(refreshToken)
}

// ParseRegionFromToken 解析区域
func ParseRegionFromToken(token string) *RegionInfo {
	return utils.ParseRegionFromToken(token)
}

// GenerateCookie 构造 Cookie
func GenerateCookie(refreshToken string) string {
	regionInfo := ParseRegionFromToken(refreshToken)
	token := AcquireToken(refreshToken)
	storeRegion := "cn-gd"
	switch {
	case regionInfo.IsUS:
		storeRegion = "us"
	case regionInfo.IsHK:
		storeRegion = "hk"
	case regionInfo.IsJP:
		storeRegion = "hk"
	case regionInfo.IsSG:
		storeRegion = "hk"
	}
	timestamp := utils.UnixTimestamp()
	sidGuard := fmt.Sprintf("sid_guard=%s%%7C%d%%7C5184000%%7CMon%%2C+03-Feb-2025+08%%3A17%%3A09+GMT", token, timestamp)
	return strings.Join([]string{
		fmt.Sprintf("_tea_web_id=%d", WebID),
		"is_staff_user=false",
		fmt.Sprintf("store-region=%s", storeRegion),
		"store-region-src=uid",
		sidGuard,
		fmt.Sprintf("uid_tt=%s", UserID),
		fmt.Sprintf("uid_tt_ss=%s", UserID),
		fmt.Sprintf("sid_tt=%s", token),
		fmt.Sprintf("sessionid=%s", token),
		fmt.Sprintf("sessionid_ss=%s", token),
		fmt.Sprintf("sid_tt=%s", token),
	}, "; ")
}

// TokenSplit 解析 Authorization
func TokenSplit(authorization string) []string {
	if authorization == "" {
		return nil
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	if token == "" {
		return nil
	}
	parts := strings.Split(token, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// GetRefererByRegion 获取 Referer
func GetRefererByRegion(refreshToken string, cnPath string) string {
	regionInfo := ParseRegionFromToken(refreshToken)
	if regionInfo.IsUS || regionInfo.IsHK || regionInfo.IsJP || regionInfo.IsSG {
		return "https://dreamina.capcut.com/"
	}
	if cnPath == "" {
		cnPath = "/"
	}
	return fmt.Sprintf("https://jimeng.jianying.com%s", cnPath)
}

// GetAssistantID 选择默认助手
func GetAssistantID(regionInfo *RegionInfo) int {
	switch {
	case regionInfo.IsUS:
		return consts.DefaultAssistantIDUS
	case regionInfo.IsHK:
		return consts.DefaultAssistantIDHK
	case regionInfo.IsJP:
		return consts.DefaultAssistantIDJP
	case regionInfo.IsSG:
		return consts.DefaultAssistantIDSG
	default:
		return consts.DefaultAssistantIDCN
	}
}

// Request 调用即梦接口
func Request(method string, uri string, refreshToken string, options *RequestOptions) (map[string]interface{}, error) {
	if options == nil {
		options = &RequestOptions{}
	}
	regionInfo := ParseRegionFromToken(refreshToken)
	baseURL, region := resolveBaseURL(uri, regionInfo)
	fullURL := fmt.Sprintf("%s%s", baseURL, uri)
	deviceTime := utils.UnixTimestamp()
	sign := utils.MD5(fmt.Sprintf("9e2c|%s|%s|%s|%d||11ac", lastPathSegment(uri), consts.PlatformCode, consts.VersionCode, deviceTime))

	origin := baseURL
	if parsed, err := url.Parse(baseURL); err == nil {
		origin = parsed.Scheme + "://" + parsed.Host
	}

	params := map[string]interface{}{}
	if !options.NoDefaultParams {
		params = map[string]interface{}{
			"aid":                     GetAssistantID(regionInfo),
			"device_platform":         "web",
			"region":                  region,
			"da_version":              consts.DAVersion,
			"os":                      "windows",
			"web_component_open_flag": 1,
			"web_version":             consts.WebVersion,
			"aigc_features":           consts.AIGCFeatures,
		}
		if !regionInfo.IsInternational {
			params["webId"] = WebID
		}
	}
	for k, v := range options.Params {
		params[k] = v
	}

	headers := map[string]string{}
	for k, v := range FakeHeaders {
		headers[k] = v
	}
	headers["Origin"] = origin
	headers["Referer"] = origin
	headers["Appid"] = fmt.Sprintf("%d", GetAssistantID(regionInfo))
	headers["Cookie"] = GenerateCookie(refreshToken)
	headers["Device-Time"] = fmt.Sprintf("%d", deviceTime)
	headers["Sign"] = sign
	headers["Sign-Ver"] = "1"
	for k, v := range options.Headers {
		headers[k] = v
	}

	var response map[string]interface{}
	exec := func() error {
		client := resty.New()
		timeout := 45 * time.Second
		if options.Timeout > 0 {
			timeout = options.Timeout
		}
		client.SetTimeout(timeout)

		req := client.R().SetHeaders(headers)
		for k, v := range params {
			req.SetQueryParam(k, fmt.Sprintf("%v", v))
		}
		if options.Body != nil {
			req.SetBody(options.Body)
		}

		logger.Debug(fmt.Sprintf("请求: %s %s", strings.ToUpper(method), fullURL))
		resp, err := req.Execute(method, fullURL)
		if err != nil {
			return err
		}

		if resp.StatusCode() >= 400 {
			return &errors.HTTPStatusError{Status: resp.StatusCode(), URL: fullURL, Body: string(resp.Body())}
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(resp.Body(), &payload); err != nil {
			return err
		}

		data, err := checkResult(payload)
		if err != nil {
			return err
		}
		response = data
		return nil
	}

	if err := errors.WithRetry(exec, &errors.ErrorHandlerOptions{
		Context:   fmt.Sprintf("%s %s", strings.ToUpper(method), uri),
		Operation: "即梦API请求",
	}); err != nil {
		return nil, err
	}
	return response, nil
}
func checkResult(result map[string]interface{}) (map[string]interface{}, error) {
	retVal, ok := result["ret"]
	if !ok {
		return result, nil
	}
	retStr := fmt.Sprintf("%v", retVal)
	if retStr == "0" {
		if data, ok := result["data"].(map[string]interface{}); ok {
			return data, nil
		}
		return result, nil
	}
	return nil, errors.HandleAPIResponse(&errors.JimengErrorResponse{
		Ret:    retStr,
		ErrMsg: fmt.Sprintf("%v", result["errmsg"]),
		Data:   result["data"],
	}, &errors.ErrorHandlerOptions{Context: "即梦API请求"})
}

func resolveBaseURL(uri string, regionInfo *RegionInfo) (string, string) {
	if regionInfo.IsUS {
		if strings.HasPrefix(uri, "/commerce/") {
			return consts.BaseURLUSCommerce, consts.RegionUS
		}
		return consts.BaseURLDreaminaUS, consts.RegionUS
	}
	if regionInfo.IsHK || regionInfo.IsJP || regionInfo.IsSG {
		if strings.HasPrefix(uri, "/commerce/") {
			return consts.BaseURLHKCommerce, regionByInfo(regionInfo)
		}
		return consts.BaseURLDreaminaHK, regionByInfo(regionInfo)
	}
	return consts.BaseURLCN, consts.RegionCN
}

func regionByInfo(info *RegionInfo) string {
	switch {
	case info.IsHK:
		return consts.RegionHK
	case info.IsJP:
		return consts.RegionJP
	case info.IsSG:
		return consts.RegionSG
	default:
		return consts.RegionHK
	}
}

func lastPathSegment(uri string) string {
	segment := uri
	if len(uri) > 7 {
		segment = uri[len(uri)-7:]
	}
	return segment
}

// GetCredit 查询积分
func GetCredit(refreshToken string) (*CreditInfo, error) {
	data, err := Request("POST", "/commerce/v1/benefits/user_credit", refreshToken, &RequestOptions{
		Body:            map[string]interface{}{},
		NoDefaultParams: true,
		Headers: map[string]string{
			"Referer": GetRefererByRegion(refreshToken, "/ai-tool/image/generate"),
		},
	})
	if err != nil {
		return nil, err
	}
	credit := mapValue(data, "credit")
	info := &CreditInfo{}
	info.GiftCredit = int64(numberValue(credit["gift_credit"]))
	info.PurchaseCredit = int64(numberValue(credit["purchase_credit"]))
	info.VipCredit = int64(numberValue(credit["vip_credit"]))
	info.TotalCredit = info.GiftCredit + info.PurchaseCredit + info.VipCredit
	logger.Info(fmt.Sprintf("积分信息: 赠送=%d, 购买=%d, VIP=%d", info.GiftCredit, info.PurchaseCredit, info.VipCredit))
	return info, nil
}

// ReceiveCredit 收取积分
func ReceiveCredit(refreshToken string) (int64, error) {
	data, err := Request("POST", "/commerce/v1/benefits/credit_receive", refreshToken, &RequestOptions{
		Body: map[string]interface{}{
			"time_zone": "Asia/Shanghai",
		},
		Headers: map[string]string{
			"Referer": GetRefererByRegion(refreshToken, "/ai-tool/home"),
		},
	})
	if err != nil {
		return 0, err
	}
	cur := int64(numberValue(data["cur_total_credits"]))
	logger.Info(fmt.Sprintf("今日积分收取完成，当前积分 %d", cur))
	return cur, nil
}

// CheckFileURL 校验文件 URL 是否可用并限制大小
func CheckFileURL(fileURL string) error {
	if fileURL == "" {
		return errors.ErrAPIRequestParamsInvalid("File URL 不能为空")
	}
	if utils.IsBASE64(fileURL) || utils.IsDataURI(fileURL) {
		return nil
	}

	req, err := http.NewRequest("HEAD", fileURL, nil)
	if err != nil {
		return errors.ErrAPIRequestFailed(fmt.Sprintf("构建请求失败: %v", err))
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errors.ErrAPIRequestFailed(fmt.Sprintf("文件 %s 不可访问: %v", fileURL, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return errors.ErrAPIRequestFailed(fmt.Sprintf("文件 %s 无效: [%d] %s", fileURL, resp.StatusCode, resp.Status))
	}
	if resp.ContentLength > 0 && resp.ContentLength > fileMaxSize {
		return errors.ErrAPIRequestFailed(fmt.Sprintf("文件 %s 超过大小限制(>100MB)", fileURL))
	}
	return nil
}

// UploadFile 上传远程或本地文件
func UploadFile(refreshToken string, fileURL string, isVideoImage bool) (*UploadFileResult, error) {
	logger.Info(fmt.Sprintf("开始上传文件: %s, 视频图像模式: %v", fileURL, isVideoImage))
	if err := CheckFileURL(fileURL); err != nil {
		return nil, err
	}

	var (
		filename string
		fileData []byte
		mimeType string
		err      error
	)

	switch {
	case utils.IsDataURI(fileURL):
		mimeType = utils.ExtractDataURIMimeType(fileURL)
		ext := utils.GuessFileExtension(mimeType)
		filename = fmt.Sprintf("%s.%s", utils.UUID(false), ext)
		decoded, decodeErr := base64.StdEncoding.DecodeString(utils.RemoveDataURIHeader(fileURL))
		if decodeErr != nil {
			return nil, errors.ErrAPIRequestFailed(fmt.Sprintf("解析BASE64数据失败: %v", decodeErr))
		}
		fileData = decoded
	case utils.IsBASE64(fileURL):
		filename = fmt.Sprintf("%s.bin", utils.UUID(false))
		data, decodeErr := base64.StdEncoding.DecodeString(fileURL)
		if decodeErr != nil {
			return nil, errors.ErrAPIRequestFailed(fmt.Sprintf("解析BASE64失败: %v", decodeErr))
		}
		fileData = data
		mimeType = "application/octet-stream"
	default:
		filename = filepath.Base(fileURL)
		if filename == "" {
			filename = fmt.Sprintf("%s.bin", utils.UUID(false))
		}
		req, reqErr := http.NewRequest("GET", fileURL, nil)
		if reqErr != nil {
			return nil, errors.ErrAPIRequestFailed(fmt.Sprintf("构建下载请求失败: %v", reqErr))
		}
		req.Header.Set("User-Agent", FakeHeaders["User-Agent"])
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

		client := &http.Client{Timeout: 60 * time.Second}
		resp, respErr := client.Do(req)
		if respErr != nil {
			return nil, errors.ErrAPIRequestFailed(fmt.Sprintf("下载文件失败: %v", respErr))
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, errors.ErrAPIRequestFailed(fmt.Sprintf("下载文件失败: [%d] %s", resp.StatusCode, resp.Status))
		}
		fileData, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.ErrAPIRequestFailed(fmt.Sprintf("读取文件失败: %v", err))
		}
		if resp.ContentLength > 0 && resp.ContentLength > fileMaxSize {
			return nil, errors.ErrAPIRequestFailed("文件大小超过限制(>100MB)")
		}
	}

	if len(fileData) == 0 {
		return nil, errors.ErrAPIRequestFailed("文件内容为空")
	}
	if mimeType == "" {
		if ext := strings.ToLower(filepath.Ext(filename)); ext != "" {
			mimeType = mime.TypeByExtension(ext)
		}
		if mimeType == "" && len(fileData) >= 512 {
			mimeType = http.DetectContentType(fileData[:512])
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	scene := "aigc_image"
	if isVideoImage {
		scene = "video_cover"
	}
	proofRequest := map[string]interface{}{
		"scene":     scene,
		"file_name": filename,
		"file_size": len(fileData),
	}

	proofResult, err := Request("POST", "/mweb/v1/get_upload_image_proof", refreshToken, &RequestOptions{Body: proofRequest})
	if err != nil {
		return nil, err
	}
	proofInfo := mapValue(proofResult, "proof_info")
	if len(proofInfo) == 0 {
		return nil, errors.ErrAPIRequestFailed("获取上传凭证失败")
	}
	imageURI := fmt.Sprintf("%v", proofInfo["image_uri"])
	if imageURI == "" {
		return nil, errors.ErrAPIRequestFailed("上传凭证缺少 image_uri")
	}

	headerMap := mapStringMap(proofInfo["headers"])
	queryMap := mapStringMap(proofInfo["query_params"])

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, errors.ErrAPIRequestFailed(fmt.Sprintf("创建表单失败: %v", err))
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, errors.ErrAPIRequestFailed(fmt.Sprintf("写入文件失败: %v", err))
	}
	writer.Close()

	uploadURL := "https://imagex.bytedanceapi.com/"
	req, err := http.NewRequest("POST", uploadURL, &buffer)
	if err != nil {
		return nil, errors.ErrAPIRequestFailed(fmt.Sprintf("构建上传请求失败: %v", err))
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for k, v := range headerMap {
		if strings.EqualFold(k, "Content-Type") {
			req.Header.Set(k, writer.FormDataContentType())
			continue
		}
		req.Header.Set(k, v)
	}
	query := req.URL.Query()
	for k, v := range queryMap {
		query.Set(k, v)
	}
	req.URL.RawQuery = query.Encode()

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.ErrFileUploadFailed(fmt.Sprintf("上传文件失败: %v", err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, errors.ErrFileUploadFailed(fmt.Sprintf("上传文件失败: 状态码 %d, 响应: %s", resp.StatusCode, string(body)))
	}

	logger.Info(fmt.Sprintf("文件上传成功: %s", imageURI))
	return &UploadFileResult{ImageURI: imageURI, URI: imageURI}, nil
}

// GetTokenLiveStatus 校验 token
func GetTokenLiveStatus(refreshToken string) (bool, error) {
	_, err := Request("POST", "/passport/account/info/v2", refreshToken, &RequestOptions{
		Params: map[string]interface{}{"account_sdk_source": "web"},
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func numberValue(v interface{}) float64 {
	switch value := v.(type) {
	case float64:
		return value
	case json.Number:
		f, _ := value.Float64()
		return f
	default:
		return 0
	}
}

func mapValue(v interface{}, key string) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		if key == "" {
			return m
		}
		if child, ok := m[key].(map[string]interface{}); ok {
			return child
		}
	}
	return map[string]interface{}{}
}

func mapStringMap(value interface{}) map[string]string {
	result := map[string]string{}
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

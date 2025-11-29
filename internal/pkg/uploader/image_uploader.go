package uploader

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/gloryhry/jimeng-api-go/internal/pkg/errors"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/signature"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/utils"
)

const uploaderUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36"

// RequestFunc 封装的请求函数，避免包之间的循环引用
type RequestFunc func(method, uri, refreshToken string, options *RequestOptions) (map[string]interface{}, error)

// RequestOptions 轻量请求配置
type RequestOptions struct {
	Headers         map[string]string
	Params          map[string]interface{}
	Body            interface{}
	NoDefaultParams bool
}

// UploadImageBuffer 上传图片缓冲区到 ImageX
func UploadImageBuffer(
	requestFn RequestFunc,
	imageBuffer []byte,
	refreshToken string,
	regionInfo *utils.RegionInfo,
) (string, error) {
	if requestFn == nil {
		return "", errors.ErrFileUploadFailed("request 函数未提供")
	}

	logger.Info("开始上传图片 Buffer 到 ImageX")
	// 1. 获取上传令牌
	uploadToken, err := requestFn("POST", "/mweb/v1/get_upload_token", refreshToken, &RequestOptions{
		Body: map[string]interface{}{
			"scene": 2,
		},
	})
	if err != nil {
		return "", errors.ErrFileUploadFailed(fmt.Sprintf("获取上传令牌失败: %v", err))
	}

	accessKey := toString(uploadToken["access_key_id"])
	secretKey := toString(uploadToken["secret_access_key"])
	sessionToken := toString(uploadToken["session_token"])
	serviceID := utils.GetServiceID(regionInfo)
	if val := toString(uploadToken["service_id"]); val != "" {
		serviceID = val
	}
	if regionInfo != nil && regionInfo.IsInternational {
		if spaceName := toString(uploadToken["space_name"]); spaceName != "" {
			serviceID = spaceName
		}
	}

	if accessKey == "" || secretKey == "" {
		return "", errors.ErrFileUploadFailed("上传凭证不完整")
	}

	fileSize := len(imageBuffer)
	crc32Value := utils.CalculateCRC32(imageBuffer)
	logger.Info(fmt.Sprintf("图片大小: %d, CRC32: %s", fileSize, crc32Value))

	imageXBase := utils.GetImageXURL(regionInfo)
	randomStr := randomString(10)
	applyURL := fmt.Sprintf("%s/?Action=ApplyImageUpload&Version=2018-08-01&ServiceId=%s&FileSize=%d&s=%s", imageXBase, serviceID, fileSize, randomStr)
	if regionInfo != nil && regionInfo.IsInternational {
		applyURL += "&device_platform=web"
	}

	authHeaders := map[string]string{
		"x-amz-date": currentTimestamp(),
	}
	if sessionToken != "" {
		authHeaders["x-amz-security-token"] = sessionToken
	}

	authorization := signature.CreateSignature(
		"GET",
		applyURL,
		authHeaders,
		accessKey,
		secretKey,
		sessionToken,
		"",
		utils.GetAWSRegion(regionInfo),
	)

	logger.Info(fmt.Sprintf("申请上传 URL: %s", applyURL))

	applyRespBody, err := doHTTP(requestConfig{
		Method:      "GET",
		URL:         applyURL,
		Headers:     buildUploadHeaders(regionInfo, authorization, authHeaders, true),
		Timeout:     45 * time.Second,
		AllowNot200: false,
	})
	if err != nil {
		logger.Error(fmt.Sprintf("申请上传请求失败: %v", err))
		return "", err
	}
	logger.Debug(fmt.Sprintf("申请上传响应: %s", string(applyRespBody)))

	applyResult := make(map[string]interface{})
	if err := json.Unmarshal(applyRespBody, &applyResult); err != nil {
		return "", errors.ErrFileUploadFailed(fmt.Sprintf("解析申请上传响应失败: %v", err))
	}

	if meta := mapValue(applyResult, "ResponseMetadata"); meta != nil {
		if errInfo := mapValue(meta, "Error"); errInfo != nil {
			return "", errors.ErrFileUploadFailed(fmt.Sprintf("申请上传失败: %v", errInfo))
		}
	}

	result := mapValue(applyResult, "Result")
	if result == nil {
		return "", errors.ErrFileUploadFailed("申请上传结果为空")
	}
	uploadAddress := mapValue(result, "UploadAddress")
	if uploadAddress == nil {
		return "", errors.ErrFileUploadFailed("缺少上传地址信息")
	}
	storeInfos := sliceValue(uploadAddress["StoreInfos"])
	if len(storeInfos) == 0 {
		return "", errors.ErrFileUploadFailed("StoreInfos 为空")
	}
	storeInfo := mapValue(storeInfos[0], "")
	uploadHosts := sliceValue(uploadAddress["UploadHosts"])
	if len(uploadHosts) == 0 {
		return "", errors.ErrFileUploadFailed("UploadHosts 为空")
	}
	uploadHost := toString(uploadHosts[0])
	sessionKey := toString(uploadAddress["SessionKey"])
	storeURI := toString(storeInfo["StoreUri"])
	fileAuth := toString(storeInfo["Auth"])

	if uploadHost == "" || storeURI == "" {
		return "", errors.ErrFileUploadFailed("上传地址信息不完整")
	}

	uploadURL := fmt.Sprintf("https://%s/upload/v1/%s", uploadHost, storeURI)
	logger.Info(fmt.Sprintf("上传文件到 %s", uploadURL))

	uploadHeaders := map[string]string{
		"Authorization":       fileAuth,
		"Content-Type":        "application/octet-stream",
		"Content-CRC32":       crc32Value,
		"Content-Disposition": "attachment; filename=\"upload.bin\"",
	}

	if _, err := doHTTP(requestConfig{
		Method:  "POST",
		URL:     uploadURL,
		Body:    bytes.NewReader(imageBuffer),
		Headers: mergeHeaders(buildUploadHeaders(regionInfo, "", nil, true), uploadHeaders),
		Timeout: 60 * time.Second,
	}); err != nil {
		logger.Error(fmt.Sprintf("文件上传请求失败: %v", err))
		return "", err
	}

	commitBody := map[string]interface{}{
		"SessionKey": sessionKey,
	}
	commitBytes, _ := json.Marshal(commitBody)
	commitURL := fmt.Sprintf("%s/?Action=CommitImageUpload&Version=2018-08-01&ServiceId=%s", imageXBase, serviceID)
	commitHeaders := map[string]string{
		"x-amz-date":           currentTimestamp(),
		"x-amz-content-sha256": sha256Hex(commitBytes),
	}
	if sessionToken != "" {
		commitHeaders["x-amz-security-token"] = sessionToken
	}
	authorization = signature.CreateSignature(
		"POST",
		commitURL,
		commitHeaders,
		accessKey,
		secretKey,
		sessionToken,
		string(commitBytes),
		utils.GetAWSRegion(regionInfo),
	)

	commitResp, err := doHTTP(requestConfig{
		Method:  "POST",
		URL:     commitURL,
		Body:    bytes.NewReader(commitBytes),
		Headers: mergeHeaders(buildUploadHeaders(regionInfo, authorization, commitHeaders, false), map[string]string{"Content-Type": "application/json"}),
		Timeout: 45 * time.Second,
	})
	if err != nil {
		logger.Error(fmt.Sprintf("提交上传请求失败: %v", err))
		return "", err
	}

	commitResult := make(map[string]interface{})
	if err := json.Unmarshal(commitResp, &commitResult); err != nil {
		return "", errors.ErrFileUploadFailed(fmt.Sprintf("解析提交响应失败: %v", err))
	}
	result = mapValue(commitResult, "Result")
	if result == nil {
		return "", errors.ErrFileUploadFailed("提交上传没有返回结果")
	}
	results := sliceValue(result["Results"])
	if len(results) == 0 {
		return "", errors.ErrFileUploadFailed("提交上传返回空结果")
	}
	first := mapValue(results[0], "")
	imageURI := toString(first["Uri"])
	if imageURI == "" {
		return "", errors.ErrFileUploadFailed("提交上传未返回 Uri")
	}

	logger.Info(fmt.Sprintf("图片上传完成: %s", imageURI))
	return imageURI, nil
}

// UploadImageFromURL 下载图片并上传
func UploadImageFromURL(
	requestFn RequestFunc,
	imageURL string,
	refreshToken string,
	regionInfo *utils.RegionInfo,
) (string, error) {
	client := &http.Client{Timeout: 45 * time.Second}
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", errors.ErrFileUploadFailed(fmt.Sprintf("构建下载请求失败: %v", err))
	}
	req.Header.Set("User-Agent", uploaderUserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", errors.ErrFileUploadFailed(fmt.Sprintf("下载图片失败: %v", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", errors.ErrFileUploadFailed(fmt.Sprintf("下载图片失败: HTTP %d", resp.StatusCode))
	}
	buffer, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return UploadImageBuffer(requestFn, buffer, refreshToken, regionInfo)
}

// 工具函数
type requestConfig struct {
	Method      string
	URL         string
	Body        io.Reader
	Headers     map[string]string
	Timeout     time.Duration
	AllowNot200 bool
}

func doHTTP(cfg requestConfig) ([]byte, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		ForceAttemptHTTP2: false,
		TLSNextProto:      make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}
	client := &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}
	req, err := http.NewRequest(cfg.Method, cfg.URL, cfg.Body)
	if err != nil {
		return nil, err
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.ErrFileUploadFailed(fmt.Sprintf("请求 %s 失败: %v", cfg.URL, err))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 && !cfg.AllowNot200 {
		return nil, errors.ErrFileUploadFailed(fmt.Sprintf("请求 %s 失败: %s", cfg.URL, string(body)))
	}
	return body, nil
}

func currentTimestamp() string {
	return time.Now().UTC().Format("20060102T150405Z")
}

func randomString(length int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func buildUploadHeaders(regionInfo *utils.RegionInfo, authorization string, extra map[string]string, includeUA bool) map[string]string {
	headers := map[string]string{
		"Accept":             "*/*",
		"Accept-Language":    "zh-CN,zh;q=0.9",
		"Origin":             utils.GetOrigin(regionInfo),
		"Referer":            utils.GetRefererPath(regionInfo, "/ai-tool/generate"),
		"sec-ch-ua":          `"Not A(Brand";v="8", "Chromium";v="132", "Google Chrome";v="132"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "cross-site",
	}
	if includeUA {
		headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36"
	}
	if authorization != "" {
		headers["Authorization"] = authorization
	}
	for k, v := range extra {
		headers[k] = v
	}
	return headers
}

func mergeHeaders(base, overrides map[string]string) map[string]string {
	result := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overrides {
		result[k] = v
	}
	return result
}

func mapValue(v interface{}, key string) map[string]interface{} {
	switch data := v.(type) {
	case map[string]interface{}:
		if key == "" {
			return data
		}
		if child, ok := data[key].(map[string]interface{}); ok {
			return child
		}
	}
	return nil
}

func sliceValue(v interface{}) []interface{} {
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return []interface{}{}
}

func toString(v interface{}) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		return fmt.Sprintf("%g", value)
	case json.Number:
		return value.String()
	default:
		return ""
	}
}

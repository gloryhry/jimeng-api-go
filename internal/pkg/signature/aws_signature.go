package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// CreateSignature AWS4-HMAC-SHA256 签名生成函数
// 用于即梦API的请求签名
func CreateSignature(
	method string,
	urlStr string,
	headers map[string]string,
	accessKeyID string,
	secretAccessKey string,
	sessionToken string,
	payload string,
	region string,
) string {
	if region == "" {
		region = "cn-north-1"
	}

	parsedURL, _ := url.Parse(urlStr)
	pathname := parsedURL.Path
	if pathname == "" {
		pathname = "/"
	}
	search := parsedURL.RawQuery

	// 创建规范请求
	timestamp := headers["x-amz-date"]
	date := timestamp[:8]
	service := "imagex"

	// 规范化查询参数
	queryParams := [][]string{}
	if search != "" {
		params, _ := url.ParseQuery(search)
		for key, values := range params {
			for _, value := range values {
				queryParams = append(queryParams, []string{key, value})
			}
		}
	}

	// 按键名排序
	sort.Slice(queryParams, func(i, j int) bool {
		return queryParams[i][0] < queryParams[j][0]
	})

	canonicalQueryString := ""
	parts := []string{}
	for _, param := range queryParams {
		parts = append(parts, fmt.Sprintf("%s=%s", param[0], param[1]))
	}
	canonicalQueryString = strings.Join(parts, "&")

	// 规范化头部
	headersToSign := map[string]string{
		"x-amz-date": timestamp,
	}

	if sessionToken != "" {
		headersToSign["x-amz-security-token"] = sessionToken
	}

	// 计算 payload hash
	payloadHash := sha256Hash("")
	if strings.ToUpper(method) == "POST" && payload != "" {
		payloadHash = sha256Hash(payload)
		headersToSign["x-amz-content-sha256"] = payloadHash
	}

	// 构建签名头部列表
	signedHeadersList := []string{}
	for key := range headersToSign {
		signedHeadersList = append(signedHeadersList, strings.ToLower(key))
	}
	sort.Strings(signedHeadersList)
	signedHeaders := strings.Join(signedHeadersList, ";")

	// 构建规范头部字符串
	canonicalHeadersParts := []string{}
	for _, key := range signedHeadersList {
		canonicalHeadersParts = append(canonicalHeadersParts, 
			fmt.Sprintf("%s:%s\n", key, strings.TrimSpace(headersToSign[key])))
	}
	canonicalHeaders := strings.Join(canonicalHeadersParts, "")

	// 构建规范请求
	canonicalRequest := strings.Join([]string{
		strings.ToUpper(method),
		pathname,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	// 创建待签名字符串
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", date, region, service)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		timestamp,
		credentialScope,
		sha256Hash(canonicalRequest),
	}, "\n")

	// 计算签名
	kDate := hmacSHA256([]byte("AWS4"+secretAccessKey), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	return fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKeyID, credentialScope, signedHeaders, signature)
}

// sha256Hash 计算 SHA256 哈希
func sha256Hash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// hmacSHA256 计算 HMAC-SHA256
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

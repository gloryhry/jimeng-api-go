package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// UUID 生成 UUID
func UUID(separator bool) string {
	id := uuid.New().String()
	if !separator {
		return strings.ReplaceAll(id, "-", "")
	}
	return id
}

// UnixTimestamp 获取 Unix 时间戳（秒）
func UnixTimestamp() int64 {
	return time.Now().Unix()
}

// Timestamp 获取 Unix 时间戳（毫秒）
func Timestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// GetDateString 获取日期字符串
func GetDateString() string {
	return time.Now().Format("2006-01-02")
}

// MD5 计算 MD5 哈希
func MD5(value string) string {
	hash := md5.Sum([]byte(value))
	return hex.EncodeToString(hash[:])
}

// CRC32 计算 CRC32 值
func CRC32(value string) uint32 {
	return crc32.ChecksumIEEE([]byte(value))
}

// CalculateCRC32 计算 ArrayBuffer 的 CRC32 值
func CalculateCRC32(buffer []byte) string {
	crcTable := crc32.MakeTable(crc32.IEEE)
	crcValue := crc32.Checksum(buffer, crcTable)

	// 转换为 4 字节数组
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, crcValue)

	// 转换为十六进制字符串
	return hex.EncodeToString(bytes)
}

// EncodeBASE64 编码为 BASE64
func EncodeBASE64(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}

// DecodeBASE64 解码 BASE64
func DecodeBASE64(value string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// IsBASE64 判断是否为 BASE64 编码
func IsBASE64(value string) bool {
	_, err := base64.StdEncoding.DecodeString(value)
	return err == nil
}

var dataURIPattern = regexp.MustCompile(`^data:(.+);base64,`)

// IsDataURI 判断是否为 data:BASE64
func IsDataURI(value string) bool {
	return dataURIPattern.MatchString(value)
}

// ExtractDataURIMimeType 提取 data URI 的 mime 类型
func ExtractDataURIMimeType(value string) string {
	matches := dataURIPattern.FindStringSubmatch(value)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// RemoveDataURIHeader 去掉 data URI 头部
func RemoveDataURIHeader(value string) string {
	if idx := strings.Index(value, ","); idx >= 0 {
		return value[idx+1:]
	}
	return value
}

// IsURL 判断是否为 URL
func IsURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// BuildDataBASE64 构建 BASE64 数据 URI
func BuildDataBASE64(mimeType, ext string, buffer []byte) string {
	base64Data := base64.StdEncoding.EncodeToString(buffer)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
}

// GuessFileExtension 根据 MIME 猜测文件扩展名
func GuessFileExtension(mimeType string) string {
	if mimeType == "" {
		return "bin"
	}
	if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
		return strings.TrimPrefix(exts[0], ".")
	}
	switch mimeType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	default:
		return "bin"
	}
}

// FetchFileBASE64 从  URL 获取文件并转换为 BASE64
func FetchFileBASE64(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// Generate SSEData 生成 SSE 数据格式
func GenerateSSEData(event, data string, retry int) string {
	var result strings.Builder

	if event != "" {
		result.WriteString(fmt.Sprintf("event: %s\n", event))
	}

	if data != "" {
		result.WriteString(fmt.Sprintf("data: %s\n", data))
	}

	if retry > 0 {
		result.WriteString(fmt.Sprintf("retry: %d\n", retry))
	}

	result.WriteString("\n")
	return result.String()
}

// ExtractImageUrls 从响应数据中提取图片 URL
func ExtractImageUrls(data interface{}) []string {
	urls := []string{}

	switch value := data.(type) {
	case []interface{}:
		for _, item := range value {
			if url := extractImageURLFromItem(item); url != "" {
				urls = append(urls, url)
			}
		}
	case map[string]interface{}:
		if images, ok := value["images"].([]interface{}); ok {
			for _, img := range images {
				if url := extractImageURLFromItem(img); url != "" {
					urls = append(urls, url)
				}
			}
		}
		if infos, ok := value["image_infos"].([]interface{}); ok {
			for _, info := range infos {
				if infoMap, ok := info.(map[string]interface{}); ok {
					if url, ok := infoMap["image_url"].(string); ok {
						urls = append(urls, sanitizeURL(url))
					}
				}
			}
		}
	}

	return urls
}

func extractImageURLFromItem(item interface{}) string {
	itemMap, ok := item.(map[string]interface{})
	if !ok {
		return ""
	}
	if image, ok := itemMap["image"].(map[string]interface{}); ok {
		if largeImages, ok := image["large_images"].([]interface{}); ok && len(largeImages) > 0 {
			first := largeImages[0]
			if info, ok := first.(map[string]interface{}); ok {
				if url, ok := info["image_url"].(string); ok {
					return sanitizeURL(url)
				}
			}
		}
	}
	if url, ok := itemMap["url"].(string); ok {
		return sanitizeURL(url)
	}
	return ""
}

func sanitizeURL(url string) string {
	return strings.ReplaceAll(url, "\\u0026", "&")
}

// ExtractVideoUrl 从响应数据中提取视频 URL
func ExtractVideoUrl(data interface{}) string {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return ""
	}

	// 检查顶层 video_url
	if videoURL, ok := dataMap["video_url"].(string); ok && videoURL != "" {
		return videoURL
	}

	// 检查 video 对象
	video, ok := dataMap["video"].(map[string]interface{})
	if !ok {
		return ""
	}

	// 优先尝试 transcoded_video.origin.video_url
	if transcodedVideo, ok := video["transcoded_video"].(map[string]interface{}); ok {
		if origin, ok := transcodedVideo["origin"].(map[string]interface{}); ok {
			if url, ok := origin["video_url"].(string); ok && url != "" {
				return url
			}
		}
	}

	// 尝试 play_url
	if url, ok := video["play_url"].(string); ok && url != "" {
		return url
	}

	// 尝试 download_url
	if url, ok := video["download_url"].(string); ok && url != "" {
		return url
	}

	// 尝试 url
	if url, ok := video["url"].(string); ok && url != "" {
		return url
	}

	// 尝试 video_list.video_1.main_url (base64 编码)
	if videoList, ok := video["video_list"].(map[string]interface{}); ok {
		if video1, ok := videoList["video_1"].(map[string]interface{}); ok {
			if mainURL, ok := video1["main_url"].(string); ok && mainURL != "" {
				// main_url 是 base64 编码的，需要解码
				decoded, err := base64.StdEncoding.DecodeString(mainURL)
				if err == nil && len(decoded) > 0 {
					return string(decoded)
				}
			}
			// 备用 URL
			if backupURL, ok := video1["backup_url_1"].(string); ok && backupURL != "" {
				decoded, err := base64.StdEncoding.DecodeString(backupURL)
				if err == nil && len(decoded) > 0 {
					return string(decoded)
				}
			}
		}
	}

	return ""
}

// SHA256Hash 计算 SHA256 哈希
func SHA256Hash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// ParseImageRatio 从图片尺寸解析比例
func ParseImageRatio(width, height int) string {
	// 计算最大公约数
	gcd := func(a, b int) int {
		for b != 0 {
			a, b = b, a%b
		}
		return a
	}

	divisor := gcd(width, height)
	ratioW := width / divisor
	ratioH := height / divisor

	return fmt.Sprintf("%d:%d", ratioW, ratioH)
}

// ParseImageRatioFromSize 根据尺寸字符串推断比例
func ParseImageRatioFromSize(size string) string {
	separators := []string{"x", "X", "*", ":"}
	for _, sep := range separators {
		if strings.Contains(size, sep) {
			parts := strings.Split(size, sep)
			if len(parts) == 2 {
				w := toInt(parts[0])
				h := toInt(parts[1])
				if w > 0 && h > 0 {
					return ParseImageRatio(w, h)
				}
			}
		}
	}
	return "1:1"
}

func toInt(value string) int {
	v := strings.TrimSpace(value)
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

// RemoveURLQueryParams 移除 URL 中的查询参数
func RemoveURLQueryParams(url string) string {
	idx := strings.Index(url, "?")
	if idx != -1 {
		return url[:idx]
	}
	return url
}

// IsValidRatio 判断比例是否有效
func IsValidRatio(ratio string) bool {
	validRatios := []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3", "21:9"}
	for _, r := range validRatios {
		if r == ratio {
			return true
		}
	}
	return false
}

// InferRatioFromPrompt 从提示词推断智能比例
func InferRatioFromPrompt(prompt string) string {
	prompt = strings.ToLower(prompt)

	// 检查横向关键词
	landscapeKeywords := []string{"landscape", "横向", "风景", "panorama", "全景"}
	for _, kw := range landscapeKeywords {
		if strings.Contains(prompt, kw) {
			return "16:9"
		}
	}

	// 检查纵向关键词
	portraitKeywords := []string{"portrait", "纵向", "竖向", "人像", "肖像"}
	for _, kw := range portraitKeywords {
		if strings.Contains(prompt, kw) {
			return "9:16"
		}
	}

	// 默认返回 1:1
	return "1:1"
}

// CleanPrompt 清理提示词
func CleanPrompt(prompt string) string {
	// 移除多余的空白字符
	re := regexp.MustCompile(`\s+`)
	prompt = re.ReplaceAllString(prompt, " ")
	return strings.TrimSpace(prompt)
}

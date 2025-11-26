package proxy

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
)

// Setup 初始化代理配置
// 如果设置了 ALL_PROXY 但未设置 HTTP_PROXY 或 HTTPS_PROXY，则将其作为默认值
func Setup() {
	allProxy := os.Getenv("ALL_PROXY")
	if allProxy != "" {
		logger.Info(fmt.Sprintf("检测到 ALL_PROXY: %s", mask(allProxy)))
		// 检查并设置 HTTP_PROXY
		if os.Getenv("HTTP_PROXY") == "" {
			os.Setenv("HTTP_PROXY", allProxy)
		}
		// 检查并设置 HTTPS_PROXY
		if os.Getenv("HTTPS_PROXY") == "" {
			os.Setenv("HTTPS_PROXY", allProxy)
		}
	}

	// 打印代理配置信息
	logProxyInfo()
}

func logProxyInfo() {
	httpProxy := os.Getenv("HTTP_PROXY")
	httpsProxy := os.Getenv("HTTPS_PROXY")
	noProxy := os.Getenv("NO_PROXY")

	if httpProxy == "" && httpsProxy == "" {
		logger.Info("未配置代理")
		return
	}

	var info []string
	if httpProxy != "" {
		info = append(info, fmt.Sprintf("http=%s", mask(httpProxy)))
		validateProxyURL("HTTP_PROXY", httpProxy)
	}
	if httpsProxy != "" {
		info = append(info, fmt.Sprintf("https=%s", mask(httpsProxy)))
		validateProxyURL("HTTPS_PROXY", httpsProxy)
	}
	if noProxy != "" {
		info = append(info, fmt.Sprintf("no_proxy=[%s]", noProxy))
	}

	logger.Info(fmt.Sprintf("已启用代理: %s", strings.Join(info, " ")))
}

func mask(u string) string {
	if u == "" {
		return ""
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return u // 解析失败直接返回原值
	}
	if parsed.User != nil {
		password, set := parsed.User.Password()
		if set {
			return strings.Replace(u, password, "***", 1)
		}
	}
	return u
}

func validateProxyURL(name, u string) {
	parsed, err := url.Parse(u)
	if err != nil {
		logger.Error(fmt.Sprintf("代理 %s 配置无效: %v", name, err))
		return
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5" {
		logger.Warn(fmt.Sprintf("代理 %s 使用了非标准协议: %s (通常支持 http, https, socks5)", name, parsed.Scheme))
	}
	if parsed.Host == "" {
		logger.Warn(fmt.Sprintf("代理 %s 未指定主机", name))
	}
}

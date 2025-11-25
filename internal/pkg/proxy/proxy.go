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
	if allProxy == "" {
		return
	}

	// 检查并设置 HTTP_PROXY
	if os.Getenv("HTTP_PROXY") == "" {
		os.Setenv("HTTP_PROXY", allProxy)
	}

	// 检查并设置 HTTPS_PROXY
	if os.Getenv("HTTPS_PROXY") == "" {
		os.Setenv("HTTPS_PROXY", allProxy)
	}

	// 打印代理配置信息
	logProxyInfo()
}

func logProxyInfo() {
	httpProxy := os.Getenv("HTTP_PROXY")
	httpsProxy := os.Getenv("HTTPS_PROXY")
	noProxy := os.Getenv("NO_PROXY")

	if httpProxy == "" && httpsProxy == "" {
		return
	}

	mask := func(u string) string {
		if u == "" {
			return ""
		}
		parsed, err := url.Parse(u)
		if err != nil {
			return u // 解析失败直接返回原值（虽然不太可能）
		}
		if parsed.User != nil {
			password, set := parsed.User.Password()
			if set {
				return strings.Replace(u, password, "***", 1)
			}
		}
		return u
	}

	info := fmt.Sprintf("已启用代理: http=%s https=%s", mask(httpProxy), mask(httpsProxy))
	if noProxy != "" {
		info += fmt.Sprintf(" no_proxy=[%s]", noProxy)
	}
	logger.Info(info)
}

package proxy

import (
	"os"
	"testing"
)

func TestSetup(t *testing.T) {
	// 保存原始环境变量
	origHTTP := os.Getenv("HTTP_PROXY")
	origHTTPS := os.Getenv("HTTPS_PROXY")
	origAll := os.Getenv("ALL_PROXY")
	defer func() {
		os.Setenv("HTTP_PROXY", origHTTP)
		os.Setenv("HTTPS_PROXY", origHTTPS)
		os.Setenv("ALL_PROXY", origAll)
	}()

	tests := []struct {
		name           string
		env            map[string]string
		wantHTTPProxy  string
		wantHTTPSProxy string
	}{
		{
			name: "ALL_PROXY sets defaults",
			env: map[string]string{
				"ALL_PROXY":   "socks5://127.0.0.1:1080",
				"HTTP_PROXY":  "",
				"HTTPS_PROXY": "",
			},
			wantHTTPProxy:  "socks5://127.0.0.1:1080",
			wantHTTPSProxy: "socks5://127.0.0.1:1080",
		},
		{
			name: "User provided SOCKS5 proxy",
			env: map[string]string{
				"ALL_PROXY":   "socks5://192.168.10.1:1080",
				"HTTP_PROXY":  "",
				"HTTPS_PROXY": "",
			},
			wantHTTPProxy:  "socks5://192.168.10.1:1080",
			wantHTTPSProxy: "socks5://192.168.10.1:1080",
		},
		{
			name: "HTTP_PROXY takes precedence",
			env: map[string]string{
				"ALL_PROXY":   "socks5://127.0.0.1:1080",
				"HTTP_PROXY":  "http://127.0.0.1:8080",
				"HTTPS_PROXY": "",
			},
			wantHTTPProxy:  "http://127.0.0.1:8080",
			wantHTTPSProxy: "socks5://127.0.0.1:1080",
		},
		{
			name: "No proxy set",
			env: map[string]string{
				"ALL_PROXY":   "",
				"HTTP_PROXY":  "",
				"HTTPS_PROXY": "",
			},
			wantHTTPProxy:  "",
			wantHTTPSProxy: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清除环境变量
			os.Unsetenv("HTTP_PROXY")
			os.Unsetenv("HTTPS_PROXY")
			os.Unsetenv("ALL_PROXY")

			// 设置测试环境变量
			for k, v := range tt.env {
				if v != "" {
					os.Setenv(k, v)
				}
			}

			Setup()

			if got := os.Getenv("HTTP_PROXY"); got != tt.wantHTTPProxy {
				t.Errorf("HTTP_PROXY = %v, want %v", got, tt.wantHTTPProxy)
			}
			if got := os.Getenv("HTTPS_PROXY"); got != tt.wantHTTPSProxy {
				t.Errorf("HTTPS_PROXY = %v, want %v", got, tt.wantHTTPSProxy)
			}
		})
	}
}

func TestMask(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://user:pass@example.com", "http://user:***@example.com"},
		{"socks5://user:pass@127.0.0.1:1080", "socks5://user:***@127.0.0.1:1080"},
		{"http://example.com", "http://example.com"},
		{"", ""},
		{"://invalid-url", "://invalid-url"},
	}

	for _, tt := range tests {
		if got := mask(tt.input); got != tt.want {
			t.Errorf("mask(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

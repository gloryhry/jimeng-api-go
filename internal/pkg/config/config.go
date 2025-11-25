package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

var (
	// Service 服务配置
	Service *ServiceConfig
	// System 系统配置
	System *SystemConfig
	// Environment 环境名称
	Environment string
)

// Init 初始化配置
func Init() error {
	// 获取环境变量
	env := os.Getenv("ENV")
	if env == "" {
		env = "dev"
	}
	Environment = env

	// 加载服务配置
	serviceConfig, err := LoadServiceConfig(env)
	if err != nil {
		return fmt.Errorf("加载服务配置失败: %w", err)
	}
	Service = serviceConfig

	// 加载系统配置
	systemConfig, err := LoadSystemConfig(env)
	if err != nil {
		return fmt.Errorf("加载系统配置失败: %w", err)
	}
	System = systemConfig

	return nil
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	Name string `mapstructure:"name"`
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// BindAddress 获取绑定地址
func (c *ServiceConfig) BindAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// LoadServiceConfig 加载服务配置
func LoadServiceConfig(env string) (*ServiceConfig, error) {
	configPath := filepath.Join("configs", env, "service.yml")

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var config ServiceConfig
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SystemConfig 系统配置
type SystemConfig struct {
	RequestLog       bool   `mapstructure:"requestLog"`
	Debug            bool   `mapstructure:"debug"`
	LogLevel         string `mapstructure:"log_level"`
	TmpDir           string `mapstructure:"tmpDir"`
	LogDir           string `mapstructure:"logDir"`
	LogWriteInterval int    `mapstructure:"logWriteInterval"`
	LogFileExpires   int64  `mapstructure:"logFileExpires"`
	PublicDir        string `mapstructure:"publicDir"`
	TmpFileExpires   int64  `mapstructure:"tmpFileExpires"`
}

// RootDirPath 获取根目录路径
func (c *SystemConfig) RootDirPath() string {
	dir, _ := os.Getwd()
	return dir
}

// TmpDirPath 获取临时目录路径
func (c *SystemConfig) TmpDirPath() string {
	if filepath.IsAbs(c.TmpDir) {
		return c.TmpDir
	}
	return filepath.Join(c.RootDirPath(), c.TmpDir)
}

// LogDirPath 获取日志目录路径
func (c *SystemConfig) LogDirPath() string {
	if filepath.IsAbs(c.LogDir) {
		return c.LogDir
	}
	return filepath.Join(c.RootDirPath(), c.LogDir)
}

// PublicDirPath 获取公共目录路径
func (c *SystemConfig) PublicDirPath() string {
	if filepath.IsAbs(c.PublicDir) {
		return c.PublicDir
	}
	return filepath.Join(c.RootDirPath(), c.PublicDir)
}

// LoadSystemConfig 加载系统配置
func LoadSystemConfig(env string) (*SystemConfig, error) {
	configPath := filepath.Join("configs", env, "system.yml")

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// 设置默认值
	v.SetDefault("requestLog", false)
	v.SetDefault("debug", true)
	v.SetDefault("log_level", "info")
	v.SetDefault("tmpDir", "./tmp")
	v.SetDefault("logDir", "./logs")
	v.SetDefault("logWriteInterval", 200)
	v.SetDefault("logFileExpires", 2626560000)
	v.SetDefault("publicDir", "./public")
	v.SetDefault("tmpFileExpires", 86400000)

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var config SystemConfig
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

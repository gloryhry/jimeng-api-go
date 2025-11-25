# Jimeng API (Go Version)

🎨 **免费的AI图像和视频生成API服务** - 基于即梦AI的逆向工程实现，Go 语言重构版本。

这是原 [jimeng-api](https://github.com/iptag/jimeng-api) TypeScript 项目的完整 Go 语言重构版本。

## 功能特性

- 🎨 **AI图像生成**：支持多种模型和分辨率（默认2K，支持4K、1K）
- 🖼️ **图生图合成**：支持本地图片或图片URL
- 🎬 **AI视频生成**：支持文本生成视频，以及本地图片上传的图生视频
- 🌐 **国际站支持**：支持即梦国内站和 Dreamina 国际站（美国、香港、日本、新加坡）
- 🔄 **智能轮询**：自适应轮询机制，优化生成效率
- 🛡️ **统一异常处理**：完善的错误处理和重试机制
- 📊 **详细日志**：结构化日志，便于调试
- ⚙️ **日志级别控制**：通过配置文件动态调整日志输出级别
- 🧩 **OpenAI 格式兼容**：`/v1/images/edits` 接受 `size`、`quality`、`response_format`

## 快速开始

### 环境要求

- Go 1.21+

### 编译和运行

```bash
# 下载依赖
go mod download

# 编译
go build -o jimeng-api ./cmd/server

# 运行
./jimeng-api
```

服务将在 `http://localhost:5100` 启动。

### 配置

编辑 `configs/dev/service.yml` 和 `configs/dev/system.yml` 配置文件。

## API 文档

完整 API 文档请参考原项目 [README](https://github.com/iptag/jimeng-api/blob/main/README.md)。

所有 API 端点、请求参数、响应格式均与 TypeScript 版本完全一致。

## 项目结构

```
jimeng-api-go/
├── cmd/server/          # 程序入口
├── internal/
│   ├── api/            # API 层
│   │   ├── controllers/ # 控制器
│   │   ├── routes/      # 路由
│   │   └── consts/      # 常量
│   └── pkg/            # 核心库
│       ├── config/      # 配置管理
│       ├── logger/      # 日志系统
│       ├── errors/      # 错误处理
│       ├── poller/      # 智能轮询器
│       ├── uploader/    # 图片上传
│       ├── signature/   # AWS 签名
│       ├── utils/       # 工具函数
│       ├── server/      # HTTP 服务器
│       └── ...
├── configs/            # 配置文件
└── public/             # 静态文件
```

## 技术栈

- **Web 框架**：Gin
- **配置管理**：Viper
- **日志系统**：Zap
- **HTTP 客户端**：Resty

## 许可证

GPL v3 License

## 致谢

本项目是 [jimeng-api](https://github.com/iptag/jimeng-api) 的 Go 语言重构版本。

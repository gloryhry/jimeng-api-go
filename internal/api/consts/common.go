package consts

// API基础URL
const (
	BaseURLCN         = "https://jimeng.jianying.com"
	BaseURLUSCommerce = "https://commerce.us.capcut.com"
	BaseURLHKCommerce = "https://commerce-api-sg.capcut.com"
	BaseURLHK         = "https://mweb-api-sg.capcut.com"
)

// 默认助手ID
const (
	DefaultAssistantIDCN = 513695
	DefaultAssistantIDUS = 513641
	DefaultAssistantIDHK = 513641
	DefaultAssistantIDJP = 513641
	DefaultAssistantIDSG = 513641
)

// 地区
const (
	RegionCN = "cn"
	RegionUS = "US"
	RegionHK = "HK"
	RegionJP = "JP"
	RegionSG = "SG"
)

// 平台代码
const PlatformCode = "7"

// 版本代码
const VersionCode = "8.4.0"

// 默认模型
const (
	DefaultImageModel = "jimeng-4.0"
	DefaultVideoModel = "jimeng-video-3.5-pro"
)

// 草稿版本
const (
	DraftVersion    = "3.3.8"
	DraftMinVersion = "3.0.2"
)

// 图像模型映射
var ImageModelMap = map[string]string{
	"jimeng-4.5":     "high_aes_general_v40l",
	"jimeng-4.1":     "high_aes_general_v41",
	"jimeng-4.0":     "high_aes_general_v40",
	"jimeng-3.1":     "high_aes_general_v30l_art_fangzhou:general_v3.0_18b",
	"jimeng-3.0":     "high_aes_general_v30l:general_v3.0_18b",
	"jimeng-2.1":     "high_aes_general_v21_L:general_v2.1_L",
	"jimeng-2.0-pro": "high_aes_general_v20_L:general_v2.0_L",
	"jimeng-2.0":     "high_aes_general_v20:general_v2.0",
	"jimeng-1.4":     "high_aes_general_v14:general_v1.4",
	"jimeng-xl-pro":  "text2img_xl_sft",
}

var ImageModelMapUS = map[string]string{
	"jimeng-4.5":    "high_aes_general_v40l",
	"jimeng-4.1":    "high_aes_general_v41",
	"jimeng-4.0":    "high_aes_general_v40",
	"jimeng-3.0":    "high_aes_general_v30l:general_v3.0_18b",
	"nanobanana":    "external_model_gemini_flash_image_v25",
	"nanobananapro": "dreamina_image_lib_1",
}

// 视频模型映射 - 国内站 (CN)
var VideoModelMap = map[string]string{
	"jimeng-video-4.0-pro":  "dreamina_seedance_40_pro",
	"jimeng-video-4.0":      "dreamina_seedance_40",
	"jimeng-video-3.5-pro":  "dreamina_ic_generate_video_model_vgfm_3.5_pro",
	"jimeng-video-3.0-pro":  "dreamina_ic_generate_video_model_vgfm_3.0_pro",
	"jimeng-video-3.0":      "dreamina_ic_generate_video_model_vgfm_3.0",
	"jimeng-video-3.0-fast": "dreamina_ic_generate_video_model_vgfm_3.0_fast",
	"jimeng-video-2.0":      "dreamina_ic_generate_video_model_vgfm_lite",
	"jimeng-video-2.0-pro":  "dreamina_ic_generate_video_model_vgfm1.0",
}

// 视频模型映射 - 美国站 (US) - 仅保留 3.0 和 3.5-pro
var VideoModelMapUS = map[string]string{
	"jimeng-video-3.5-pro": "dreamina_ic_generate_video_model_vgfm_3.5_pro",
	"jimeng-video-3.0":     "dreamina_ic_generate_video_model_vgfm_3.0",
}

// 视频模型映射 - 亚洲国际站 (HK/JP/SG)
var VideoModelMapAsia = map[string]string{
	"jimeng-video-veo3":     "dreamina_veo3_generate_video",
	"jimeng-video-veo3.1":   "dreamina_veo3.1_generate_video",
	"jimeng-video-sora2":    "dreamina_sora2_generate_video",
	"jimeng-video-3.5-pro":  "dreamina_ic_generate_video_model_vgfm_3.5_pro",
	"jimeng-video-3.0-pro":  "dreamina_ic_generate_video_model_vgfm_3.0_pro",
	"jimeng-video-3.0":      "dreamina_ic_generate_video_model_vgfm_3.0",
	"jimeng-video-3.0-fast": "dreamina_ic_generate_video_model_vgfm_3.0_fast",
	"jimeng-video-2.0":      "dreamina_ic_generate_video_model_vgfm_lite",
	"jimeng-video-2.0-pro":  "dreamina_ic_generate_video_model_vgfm1.0",
}

// 状态码映射
var StatusCodeMap = map[int]string{
	20: "PROCESSING",
	10: "SUCCESS",
	30: "FAILED",
	42: "POST_PROCESSING",
	45: "FINALIZING",
	50: "COMPLETED",
}

// 重试配置
const (
	MaxRetryCount = 3
	RetryDelay    = 5000 // 毫秒
)

// 轮询配置
const (
	MaxPollCount   = 40   // 最大轮询次数
	PollInterval   = 5000 // 5秒（毫秒）
	StableRounds   = 3    // 稳定轮次
	TimeoutSeconds = 180  // 3分钟超时
)

// ResolutionParams 分辨率参数
type ResolutionParams struct {
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Ratio          int    `json:"ratio"`
	ResolutionType string `json:"resolution_type"`
}

// ResolutionOptions 支持的图片比例和分辨率
var ResolutionOptions = map[string]map[string]ResolutionParams{
	"1k": {
		"1:1":  {Width: 1328, Height: 1328, Ratio: 1, ResolutionType: "1k"},
		"4:3":  {Width: 1472, Height: 1104, Ratio: 4, ResolutionType: "1k"},
		"3:4":  {Width: 1104, Height: 1472, Ratio: 2, ResolutionType: "1k"},
		"16:9": {Width: 1664, Height: 936, Ratio: 3, ResolutionType: "1k"},
		"9:16": {Width: 936, Height: 1664, Ratio: 5, ResolutionType: "1k"},
		"3:2":  {Width: 1584, Height: 1056, Ratio: 7, ResolutionType: "1k"},
		"2:3":  {Width: 1056, Height: 1584, Ratio: 6, ResolutionType: "1k"},
		"21:9": {Width: 2016, Height: 864, Ratio: 8, ResolutionType: "1k"},
	},
	"2k": {
		"1:1":  {Width: 2048, Height: 2048, Ratio: 1, ResolutionType: "2k"},
		"4:3":  {Width: 2304, Height: 1728, Ratio: 4, ResolutionType: "2k"},
		"3:4":  {Width: 1728, Height: 2304, Ratio: 2, ResolutionType: "2k"},
		"16:9": {Width: 2560, Height: 1440, Ratio: 3, ResolutionType: "2k"},
		"9:16": {Width: 1440, Height: 2560, Ratio: 5, ResolutionType: "2k"},
		"3:2":  {Width: 2496, Height: 1664, Ratio: 7, ResolutionType: "2k"},
		"2:3":  {Width: 1664, Height: 2496, Ratio: 6, ResolutionType: "2k"},
		"21:9": {Width: 3024, Height: 1296, Ratio: 8, ResolutionType: "2k"},
	},
	"4k": {
		"1:1":  {Width: 4096, Height: 4096, Ratio: 1, ResolutionType: "4k"},
		"4:3":  {Width: 4693, Height: 3520, Ratio: 4, ResolutionType: "4k"},
		"3:4":  {Width: 3520, Height: 4693, Ratio: 2, ResolutionType: "4k"},
		"16:9": {Width: 5404, Height: 3040, Ratio: 3, ResolutionType: "4k"},
		"9:16": {Width: 3040, Height: 5404, Ratio: 5, ResolutionType: "4k"},
		"3:2":  {Width: 4992, Height: 3328, Ratio: 7, ResolutionType: "4k"},
		"2:3":  {Width: 3328, Height: 4992, Ratio: 6, ResolutionType: "4k"},
		"21:9": {Width: 6197, Height: 2656, Ratio: 8, ResolutionType: "4k"},
	},
}

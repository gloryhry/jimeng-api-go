package consts

// Dreamina 国际站相关常量
const (
	// API 域名
	BaseURLDreaminaUS = "https://dreamina-api.us.capcut.com"
	BaseURLDreaminaHK = "https://mweb-api-sg.capcut.com"

	// ImageX 上传域名
	BaseURLImageXUS = "https://imagex16-normal-us-ttp.capcutapi.us"
	BaseURLImageXHK = "https://imagex-normal-sg.capcutapi.com"

	// Web 配置
	WebVersion   = "7.5.0"
	DAVersion    = "3.3.8"
	AIGCFeatures = "app_lip_sync"
)

// DreaminaRefererMap 各区域 referer
var DreaminaRefererMap = map[string]string{
	RegionUS: "https://dreamina.capcut.com/ai-tool/image/generate",
	RegionHK: "https://dreamina.capcut.com/ai-tool/image/generate",
	RegionJP: "https://dreamina.capcut.com/ai-tool/image/generate",
	RegionSG: "https://dreamina.capcut.com/ai-tool/image/generate",
}

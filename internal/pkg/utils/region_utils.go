package utils

import (
	"fmt"

	"github.com/gloryhry/jimeng-api-go/internal/api/consts"
)

// RegionInfo 地区信息
type RegionInfo struct {
	IsUS            bool
	IsHK            bool
	IsJP            bool
	IsSG            bool
	IsInternational bool
	IsCN            bool
}

// ParseRegionFromToken 从 token 中解析地区信息
func ParseRegionFromToken(token string) *RegionInfo {
	info := &RegionInfo{}

	if len(token) > 3 {
		prefix := token[:3]
		switch prefix {
		case "us-":
			info.IsUS = true
			info.IsInternational = true
		case "hk-":
			info.IsHK = true
			info.IsInternational = true
		case "jp-":
			info.IsJP = true
			info.IsInternational = true
		case "sg-":
			info.IsSG = true
			info.IsInternational = true
		default:
			info.IsCN = true
		}
	} else {
		info.IsCN = true
	}

	return info
}

// GetServiceID 获取对应区域的 service id
func GetServiceID(regionInfo *RegionInfo) string {
	if regionInfo == nil {
		return "tb4s082cfz"
	}
	if regionInfo.IsUS || regionInfo.IsHK || regionInfo.IsJP || regionInfo.IsSG {
		return "wopfjsm1ax"
	}
	return "tb4s082cfz"
}

// GetImageXURL 获取 ImageX 上传地址
func GetImageXURL(regionInfo *RegionInfo) string {
	if regionInfo == nil {
		return "https://imagex.bytedanceapi.com"
	}
	if regionInfo.IsUS {
		return consts.BaseURLImageXUS
	}
	if regionInfo.IsHK || regionInfo.IsJP || regionInfo.IsSG {
		return consts.BaseURLImageXHK
	}
	return "https://imagex.bytedanceapi.com"
}

// GetOrigin 获取请求来源
func GetOrigin(regionInfo *RegionInfo) string {
	if regionInfo == nil {
		return "https://jimeng.jianying.com"
	}
	if regionInfo.IsUS {
		return consts.BaseURLDreaminaUS
	}
	if regionInfo.IsHK || regionInfo.IsJP || regionInfo.IsSG {
		return consts.BaseURLDreaminaHK
	}
	return "https://jimeng.jianying.com"
}

// GetRefererPath 构造 referer
func GetRefererPath(regionInfo *RegionInfo, path string) string {
	if path == "" {
		path = "/ai-tool/generate"
	}
	return fmt.Sprintf("%s%s", GetOrigin(regionInfo), path)
}

// GetAWSRegion 获取 AWS 区域
func GetAWSRegion(regionInfo *RegionInfo) string {
	if regionInfo == nil {
		return "cn-north-1"
	}
	if regionInfo.IsUS {
		return "us-east-1"
	}
	if regionInfo.IsHK || regionInfo.IsJP || regionInfo.IsSG {
		return "ap-southeast-1"
	}
	return "cn-north-1"
}

// GetRegionCode 获取地区代码
func GetRegionCode(regionInfo *RegionInfo) string {
	if regionInfo.IsUS {
		return "US"
	}
	if regionInfo.IsHK {
		return "HK"
	}
	if regionInfo.IsJP {
		return "JP"
	}
	if regionInfo.IsSG {
		return "SG"
	}
	return "cn"
}

// RemoveRegionPrefix 移除地区前缀
func RemoveRegionPrefix(token string) string {
	if len(token) > 3 {
		prefix := token[:3]
		if prefix == "us-" || prefix == "hk-" || prefix == "jp-" || prefix == "sg-" {
			return token[3:]
		}
	}
	return token
}

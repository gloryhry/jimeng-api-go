package builders

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gloryhry/jimeng-api-go/internal/api/consts"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/utils"
)

type RegionKey string

const (
	RegionKeyCN RegionKey = "CN"
	RegionKeyUS RegionKey = "US"
	RegionKeyHK RegionKey = "HK"
	RegionKeyJP RegionKey = "JP"
	RegionKeySG RegionKey = "SG"
)

type RegionInfo interface {
	IsUSRegion() bool
	IsHKRegion() bool
	IsJPRegion() bool
	IsSGRegion() bool
}

type ResolutionResult struct {
	Width          int
	Height         int
	ImageRatio     float64
	ResolutionType string
	IsForced       bool
}

func GetRegionKey(regionInfo RegionInfo) RegionKey {
	if regionInfo.IsUSRegion() {
		return RegionKeyUS
	}
	if regionInfo.IsHKRegion() {
		return RegionKeyHK
	}
	if regionInfo.IsJPRegion() {
		return RegionKeyJP
	}
	if regionInfo.IsSGRegion() {
		return RegionKeySG
	}
	return RegionKeyCN
}

func LookupResolution(resolution string, ratio string) (*ResolutionResult, error) {
	if resolution == "" {
		resolution = "2k"
	}
	if ratio == "" {
		ratio = "1:1"
	}

	resolutionGroup, ok := consts.ResolutionOptions[resolution]
	if !ok {
		// supportedResolutions := ... (omitted for brevity, can be added if needed)
		return nil, fmt.Errorf("不支持的分辨率 \"%s\"", resolution)
	}

	ratioConfig, ok := resolutionGroup[ratio]
	if !ok {
		// supportedRatios := ...
		return nil, fmt.Errorf("在 \"%s\" 分辨率下，不支持的比例 \"%s\"", resolution, ratio)
	}

	return &ResolutionResult{
		Width:          ratioConfig.Width,
		Height:         ratioConfig.Height,
		ImageRatio:     float64(ratioConfig.Ratio),
		ResolutionType: resolution,
		IsForced:       false,
	}, nil
}

// ResolveResolution 统一分辨率处理逻辑
func ResolveResolution(userModel string, regionInfo RegionInfo, resolution string, ratio string) (*ResolutionResult, error) {
	regionKey := GetRegionKey(regionInfo)

	// ⚠️ 国内站不支持nano系列模型
	if regionKey == RegionKeyCN && (userModel == "nanobanana" || userModel == "nanobananapro") {
		return nil, fmt.Errorf("国内站不支持%s模型,请使用jimeng系列模型", userModel)
	}

	// ⚠️ nanobanana 模型的站点差异处理
	if userModel == "nanobanana" {
		if regionKey == RegionKeyUS {
			// US 站: 强制 1024x1024@2k, ratio 固定为 1
			return &ResolutionResult{
				Width:          1024,
				Height:         1024,
				ImageRatio:     1,
				ResolutionType: "2k",
				IsForced:       true,
			}, nil
		} else if regionKey == RegionKeyHK || regionKey == RegionKeyJP || regionKey == RegionKeySG {
			// HK/JP/SG 站: 强制 1k 分辨率，但 ratio 可自定义
			params, err := LookupResolution("1k", ratio)
			if err != nil {
				return nil, err
			}
			return &ResolutionResult{
				Width:          params.Width,
				Height:         params.Height,
				ImageRatio:     params.ImageRatio,
				ResolutionType: "1k",
				IsForced:       true,
			}, nil
		}
	}

	// 其他所有情况: 使用用户指定的 resolution 和 ratio
	return LookupResolution(resolution, ratio)
}

// GetBenefitCount 规则
func GetBenefitCount(userModel string, regionInfo RegionInfo, isMultiImage bool) *int {
	if isMultiImage {
		return nil
	}

	regionKey := GetRegionKey(regionInfo)

	if regionKey == RegionKeyCN {
		return nil
	}

	if regionKey == RegionKeyUS {
		if userModel == "jimeng-4.0" || userModel == "jimeng-3.0" {
			val := 4
			return &val
		}
		return nil
	}

	if regionKey == RegionKeyHK || regionKey == RegionKeyJP || regionKey == RegionKeySG {
		if userModel == "nanobanana" {
			return nil
		}
		val := 4
		return &val
	}

	return nil
}

type GenerateMode string

const (
	GenerateModeText2Img GenerateMode = "text2img"
	GenerateModeImg2Img  GenerateMode = "img2img"
)

type BuildCoreParamOptions struct {
	UserModel        string
	Model            string
	Prompt           string
	ImageCount       int
	NegativePrompt   string
	Seed             int64
	SampleStrength   float64
	Resolution       *ResolutionResult
	IntelligentRatio bool
	Mode             GenerateMode
}

func BuildCoreParam(options BuildCoreParamOptions) map[string]interface{} {
	// ⚠️ intelligent_ratio 仅对 jimeng-4.0 模型有效
	effectiveIntelligentRatio := false
	if options.UserModel == "jimeng-4.0" {
		effectiveIntelligentRatio = options.IntelligentRatio
	}

	// 图生图时，prompt 前缀规则: 每张图片对应 2 个 #
	promptPrefix := ""
	if options.Mode == GenerateModeImg2Img {
		promptPrefix = strings.Repeat("#", options.ImageCount*2)
	}

	coreParam := map[string]interface{}{
		"type":            "",
		"id":              utils.UUID(true),
		"model":           options.Model,
		"prompt":          promptPrefix + options.Prompt,
		"sample_strength": options.SampleStrength,
		"large_image_info": map[string]interface{}{
			"type":            "",
			"id":              utils.UUID(true),
			"height":          options.Resolution.Height,
			"width":           options.Resolution.Width,
			"resolution_type": options.Resolution.ResolutionType,
		},
		"intelligent_ratio": effectiveIntelligentRatio,
	}

	if options.Mode == GenerateModeImg2Img {
		coreParam["image_ratio"] = options.Resolution.ImageRatio
	} else if !effectiveIntelligentRatio {
		coreParam["image_ratio"] = options.Resolution.ImageRatio
	}

	if options.NegativePrompt != "" {
		coreParam["negative_prompt"] = options.NegativePrompt
	}

	if options.Seed != 0 {
		coreParam["seed"] = options.Seed
	}

	return coreParam
}

type SceneType string

const (
	SceneTypeImageBasicGenerate SceneType = "ImageBasicGenerate"
	SceneTypeImageMultiGenerate SceneType = "ImageMultiGenerate"
)

type Ability struct {
	AbilityName string  `json:"abilityName"`
	Strength    float64 `json:"strength"`
	Source      *struct {
		ImageURL string `json:"imageUrl"`
	} `json:"source,omitempty"`
}

type BuildMetricsExtraOptions struct {
	UserModel      string
	Model          string // 映射后的模型名称
	RegionInfo     RegionInfo
	SubmitID       string
	Scene          SceneType
	ResolutionType string
	AbilityList    []Ability
	IsMultiImage   bool
}

func BuildMetricsExtra(options BuildMetricsExtraOptions) string {
	benefitCount := GetBenefitCount(options.UserModel, options.RegionInfo, options.IsMultiImage)

	sceneOption := map[string]interface{}{
		"type":           "image",
		"scene":          options.Scene,
		"modelReqKey":    options.Model, // 使用映射后的模型名称
		"resolutionType": options.ResolutionType,
		"abilityList":    options.AbilityList,
		"reportParams": map[string]interface{}{
			"enterSource":                      "generate",
			"vipSource":                        "generate",
			"extraVipFunctionKey":              fmt.Sprintf("%s-%s", options.UserModel, options.ResolutionType),
			"useVipFunctionDetailsReporterHoc": true,
		},
	}

	if benefitCount != nil {
		sceneOption["benefitCount"] = *benefitCount
	}

	metrics := map[string]interface{}{
		"promptSource":  "custom",
		"generateCount": 1,
		"enterFrom":     "click",
		"sceneOptions":  mustJSON([]interface{}{sceneOption}), // Note: sceneOptions is a JSON string in TS, but here we might need to be careful. In TS it is `JSON.stringify([sceneOption])`.
		"generateId":    options.SubmitID,
		"isRegenerate":  false,
	}
	// Correcting sceneOptions to be a string as per TS
	// metrics["sceneOptions"] = mustJSON([]interface{}{sceneOption}) // Wait, in TS: sceneOptions: JSON.stringify([sceneOption])

	if options.IsMultiImage {
		metrics["templateId"] = ""
		metrics["templateSource"] = ""
		metrics["lastRequestId"] = ""
		metrics["originRequestId"] = ""
	}

	return mustJSON(metrics)
}

type BuildDraftContentOptions struct {
	ComponentID               string
	GenerateType              string // "generate" | "blend"
	CoreParam                 map[string]interface{}
	AbilityList               []map[string]interface{}
	PromptPlaceholderInfoList []map[string]interface{}
	PosteditParam             map[string]interface{}
	ImageCount                int
}

func BuildDraftContent(options BuildDraftContentOptions) string {
	abilities := map[string]interface{}{
		"type": "",
		"id":   utils.UUID(true),
	}

	isBlend := options.GenerateType == "blend"
	draftMinVersion := consts.DraftMinVersion
	if isBlend {
		draftMinVersion = "3.2.9"
	}

	if options.GenerateType == "generate" {
		abilities["generate"] = map[string]interface{}{
			"type":       "",
			"id":         utils.UUID(true),
			"core_param": options.CoreParam,
			"gen_option": map[string]interface{}{
				"type":         "",
				"id":           utils.UUID(true),
				"generate_all": false,
			},
		}
	} else {
		blend := map[string]interface{}{
			"type":                         "",
			"id":                           utils.UUID(true),
			"min_features":                 []interface{}{},
			"core_param":                   options.CoreParam,
			"ability_list":                 options.AbilityList,
			"prompt_placeholder_info_list": options.PromptPlaceholderInfoList,
			"postedit_param":               options.PosteditParam,
		}
		if options.ImageCount >= 2 {
			blend["min_version"] = "3.2.9"
		}
		abilities["blend"] = blend
		abilities["gen_option"] = map[string]interface{}{
			"type":         "",
			"id":           utils.UUID(true),
			"generate_all": false,
		}
	}

	draftContent := map[string]interface{}{
		"type":              "draft",
		"id":                utils.UUID(true),
		"min_version":       draftMinVersion,
		"min_features":      []interface{}{},
		"is_from_tsn":       true,
		"version":           consts.DraftVersion,
		"main_component_id": options.ComponentID,
		"component_list": []map[string]interface{}{
			{
				"type":        "image_base_component",
				"id":          options.ComponentID,
				"min_version": consts.DraftMinVersion,
				"aigc_mode":   "workbench",
				"metadata": map[string]interface{}{
					"type":                     "",
					"id":                       utils.UUID(true),
					"created_platform":         3,
					"created_platform_version": "",
					"created_time_in_ms":       fmt.Sprintf("%d", time.Now().UnixMilli()),
					"created_did":              "",
				},
				"generate_type": options.GenerateType,
				"abilities":     abilities,
			},
		},
	}

	return mustJSON(draftContent)
}

type BuildGenerateRequestOptions struct {
	Model        string
	RegionInfo   RegionInfo
	SubmitID     string
	DraftContent string
	MetricsExtra string
	AssistantID  int
}

func BuildGenerateRequest(options BuildGenerateRequestOptions) map[string]interface{} {
	return map[string]interface{}{
		"extend": map[string]interface{}{
			"root_model": options.Model,
		},
		"submit_id":     options.SubmitID,
		"metrics_extra": options.MetricsExtra,
		"draft_content": options.DraftContent,
		"http_common_info": map[string]interface{}{
			"aid": options.AssistantID,
		},
	}
}

func BuildBlendAbilityList(uploadedImageIds []string, strength float64) []map[string]interface{} {
	list := make([]map[string]interface{}, len(uploadedImageIds))
	for i, imageID := range uploadedImageIds {
		list[i] = map[string]interface{}{
			"type":           "",
			"id":             utils.UUID(true),
			"name":           "byte_edit",
			"image_uri_list": []string{imageID},
			"image_list": []map[string]interface{}{
				{
					"type":          "image",
					"id":            utils.UUID(true),
					"source_from":   "upload",
					"platform_type": 1,
					"name":          "",
					"image_uri":     imageID,
					"width":         0,
					"height":        0,
					"format":        "",
					"uri":           imageID,
				},
			},
			"strength": strength,
		}
	}
	return list
}

func BuildPromptPlaceholderList(count int) []map[string]interface{} {
	list := make([]map[string]interface{}, count)
	for i := 0; i < count; i++ {
		list[i] = map[string]interface{}{
			"type":          "",
			"id":            utils.UUID(true),
			"ability_index": i,
		}
	}
	return list
}

func mustJSON(data interface{}) string {
	bytes, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

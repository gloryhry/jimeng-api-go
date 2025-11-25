package controllers

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gloryhry/jimeng-api-go/internal/api/consts"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/errors"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/poller"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/task"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/uploader"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/utils"
)

const (
	defaultImageModel = consts.DefaultImageModel
	maxBlendImages    = 10
)

var (
	multiImageKeywords   = []string{"连续", "绘本", "故事"}
	multiImageCountRegex = regexp.MustCompile(`(\d+)张`)
)

// ImageOptions 描述图像生成参数
type ImageOptions struct {
	Ratio            string
	Resolution       string
	SampleStrength   float64
	NegativePrompt   string
	IntelligentRatio bool
}

// GetResolutionParams 返回分辨率配置信息
func GetResolutionParams(resolution, ratio string) (consts.ResolutionParams, error) {
	if resolution == "" {
		resolution = "2k"
	}
	if ratio == "" {
		ratio = "1:1"
	}

	resMap, ok := consts.ResolutionOptions[resolution]
	if !ok {
		return consts.ResolutionParams{}, errors.ErrAPIRequestParamsInvalid(
			fmt.Sprintf("不支持的分辨率 \"%s\"。支持的分辨率: %s", resolution, strings.Join(sortedMapKeys(consts.ResolutionOptions), ", ")),
		)
	}

	if params, ok := resMap[ratio]; ok {
		return params, nil
	}

	return consts.ResolutionParams{}, errors.ErrAPIRequestParamsInvalid(
		fmt.Sprintf("在 \"%s\" 分辨率下，不支持的比例 \"%s\"。支持的比例: %s", resolution, ratio, strings.Join(sortedResolutionKeys(resMap), ", ")),
	)
}

// GetImageModel 根据区域返回映射模型
func GetImageModel(model string, isInternational bool) (string, error) {
	if model == "" {
		model = defaultImageModel
	}
	if isInternational {
		if mapped, ok := consts.ImageModelMapUS[model]; ok {
			return mapped, nil
		}
		return "", errors.ErrAPIRequestParamsInvalid(
			fmt.Sprintf("国际版不支持模型 \"%s\"。支持的模型: %s", model, strings.Join(sortedModelKeys(consts.ImageModelMapUS), ", ")),
		)
	}
	if mapped, ok := consts.ImageModelMap[model]; ok {
		return mapped, nil
	}
	if mapped, ok := consts.ImageModelMap[defaultImageModel]; ok {
		return mapped, nil
	}
	return "", errors.ErrAPIRequestParamsInvalid(fmt.Sprintf("不支持的模型 \"%s\"", model))
}

// GenerateImages 文生图
func GenerateImages(model string, prompt string, opts *ImageOptions, refreshToken string) ([]string, error) {
	tm := task.NewTaskManager()
	result, err := tm.ExecuteTask(
		func() (string, error) {
			return SubmitImageGeneration(model, prompt, opts, refreshToken)
		},
		func(taskID string) (interface{}, error) {
			return PollImageResult(taskID, refreshToken, 4)
		},
	)
	if err != nil {
		return nil, err
	}
	return result.([]string), nil
}

// SubmitImageGeneration 提交文生图任务
func SubmitImageGeneration(model string, prompt string, opts *ImageOptions, refreshToken string) (string, error) {
	if opts == nil {
		opts = &ImageOptions{}
	}
	ensureImageOptionDefaults(opts)
	region := ParseRegionFromToken(refreshToken)
	mappedModel, err := GetImageModel(model, region.IsInternational)
	if err != nil {
		return "", err
	}

	logger.Info(fmt.Sprintf("使用模型: %s 映射模型: %s 分辨率: %s 比例: %s 精细度: %.2f 智能比例: %v",
		model, mappedModel, opts.Resolution, opts.Ratio, opts.SampleStrength, opts.IntelligentRatio))

	return submitImagesInternal(mappedModel, model, prompt, opts, refreshToken, region)
}

// GenerateImageComposition 图生图
func GenerateImageComposition(model string, prompt string, images []interface{}, opts *ImageOptions, refreshToken string) ([]string, error) {
	tm := task.NewTaskManager()
	result, err := tm.ExecuteTask(
		func() (string, error) {
			return SubmitImageComposition(model, prompt, images, opts, refreshToken)
		},
		func(taskID string) (interface{}, error) {
			return PollImageResult(taskID, refreshToken, 1)
		},
	)
	if err != nil {
		return nil, err
	}
	return result.([]string), nil
}

// SubmitImageComposition 提交图生图任务
func SubmitImageComposition(model string, prompt string, images []interface{}, opts *ImageOptions, refreshToken string) (string, error) {
	if len(images) == 0 {
		return "", errors.ErrAPIRequestParamsInvalid("至少需要提供1张图片")
	}
	if len(images) > maxBlendImages {
		return "", errors.ErrAPIRequestParamsInvalid("最多支持10张图片")
	}
	if opts == nil {
		opts = &ImageOptions{}
	}
	ensureImageOptionDefaults(opts)

	region := ParseRegionFromToken(refreshToken)
	mappedModel, err := GetImageModel(model, region.IsInternational)
	if err != nil {
		return "", err
	}
	params, err := resolveResolutionForModel(model, opts)
	if err != nil {
		return "", err
	}

	logger.Info(fmt.Sprintf("使用模型: %s 映射模型: %s 图生图功能 %d张图片 %dx%d 精细度: %.2f",
		model, mappedModel, len(images), params.Width, params.Height, opts.SampleStrength))

	if credit, err := ensureCredit(refreshToken); err != nil {
		logger.Warn(fmt.Sprintf("获取积分失败: %v", err))
	} else if credit.TotalCredit <= 0 {
		_, _ = ReceiveCredit(refreshToken)
	}

	uploaderExec := adaptRequestForUploader()
	uploadIDs := make([]string, 0, len(images))
	for idx, item := range images {
		id, err := uploadImageSource(uploaderExec, item, refreshToken, region)
		if err != nil {
			return "", errors.ErrAPIRequestFailed(fmt.Sprintf("图片 %d 上传失败: %v", idx+1, err))
		}
		uploadIDs = append(uploadIDs, id)
	}

	componentID := utils.UUID(true)
	submitID := utils.UUID(true)
	metrics := mustJSON(map[string]interface{}{
		"promptSource":  "custom",
		"generateCount": 1,
		"enterFrom":     "click",
		"generateId":    submitID,
		"isRegenerate":  false,
	})

	coreParam := map[string]interface{}{
		"type":              "",
		"id":                utils.UUID(true),
		"model":             mappedModel,
		"prompt":            fmt.Sprintf("##%s", prompt),
		"sample_strength":   opts.SampleStrength,
		"image_ratio":       params.Ratio,
		"large_image_info":  map[string]interface{}{"type": "", "id": utils.UUID(true), "height": params.Height, "width": params.Width, "resolution_type": params.ResolutionType},
		"intelligent_ratio": opts.IntelligentRatio,
	}

	abilityList := buildBlendAbilities(uploadIDs, opts.SampleStrength)
	component := buildBlendComponent(componentID, abilityList, coreParam)
	draft := buildDraft(componentID, component)

	payload := map[string]interface{}{
		"extend": map[string]interface{}{
			"root_model": mappedModel,
		},
		"submit_id":     submitID,
		"metrics_extra": metrics,
		"draft_content": string(mustJSONBytes(draft)),
		"http_common_info": map[string]interface{}{
			"aid": GetAssistantID(region),
		},
	}

	response, err := Request("POST", "/mweb/v1/aigc_draft/generate", refreshToken, &RequestOptions{Body: payload})
	if err != nil {
		return "", err
	}
	data := mapValue(response, "aigc_data")
	historyID := fmt.Sprintf("%v", data["history_record_id"])
	if historyID == "" {
		return "", errors.ErrAPIImageGenerationFailed("记录ID不存在")
	}

	return historyID, nil
}

// GenerateImageEdits 兼容 OpenAI 接口
func GenerateImageEdits(model string, prompt string, images []interface{}, opts *ImageOptions, refreshToken string) ([]string, error) {
	if opts == nil {
		opts = &ImageOptions{}
	}
	ensureImageOptionDefaults(opts)
	return GenerateImageComposition(model, prompt, images, opts, refreshToken)
}

// SubmitImageEdits 提交编辑任务
func SubmitImageEdits(model string, prompt string, images []interface{}, opts *ImageOptions, refreshToken string) (string, error) {
	if opts == nil {
		opts = &ImageOptions{}
	}
	ensureImageOptionDefaults(opts)
	return SubmitImageComposition(model, prompt, images, opts, refreshToken)
}

func submitImagesInternal(mappedModel, requestedModel, prompt string, opts *ImageOptions, refreshToken string, region *RegionInfo) (string, error) {
	params, err := resolveResolutionForModel(requestedModel, opts)
	if err != nil {
		return "", err
	}
	logger.Info(fmt.Sprintf("生成参数: 分辨率=%s 比例=%s", opts.Resolution, opts.Ratio))

	if credit, err := ensureCredit(refreshToken); err != nil {
		logger.Warn(fmt.Sprintf("获取积分失败: %v", err))
	} else {
		logger.Info(fmt.Sprintf("当前积分状态: 总计=%d, 赠送=%d, 购买=%d, VIP=%d", credit.TotalCredit, credit.GiftCredit, credit.PurchaseCredit, credit.VipCredit))
		if credit.TotalCredit <= 0 {
			_, _ = ReceiveCredit(refreshToken)
		}
	}

	if shouldUseMultiImage(requestedModel, prompt) {
		return submitJimeng40MultiImages(mappedModel, requestedModel, prompt, opts, refreshToken, region)
	}

	componentID := utils.UUID(true)
	submitID := utils.UUID(true)
	generateID := utils.UUID(true)
	metrics := mustJSON(map[string]interface{}{
		"promptSource":  "custom",
		"generateCount": 1,
		"enterFrom":     "click",
		"generateId":    generateID,
		"isRegenerate":  false,
	})

	coreParam := map[string]interface{}{
		"type":              "",
		"id":                utils.UUID(true),
		"model":             mappedModel,
		"prompt":            prompt,
		"negative_prompt":   opts.NegativePrompt,
		"seed":              randomSeed(),
		"sample_strength":   opts.SampleStrength,
		"large_image_info":  map[string]interface{}{"type": "", "id": utils.UUID(true), "height": params.Height, "width": params.Width, "resolution_type": params.ResolutionType},
		"intelligent_ratio": opts.IntelligentRatio,
	}
	if !opts.IntelligentRatio {
		coreParam["image_ratio"] = params.Ratio
	}

	component := buildGenerateComponent(componentID, coreParam)
	draft := buildDraft(componentID, component)

	payload := map[string]interface{}{
		"extend": map[string]interface{}{
			"root_model": mappedModel,
		},
		"submit_id":     submitID,
		"metrics_extra": metrics,
		"draft_content": string(mustJSONBytes(draft)),
		"http_common_info": map[string]interface{}{
			"aid": GetAssistantID(region),
		},
	}

	response, err := Request("POST", "/mweb/v1/aigc_draft/generate", refreshToken, &RequestOptions{Body: payload})
	if err != nil {
		return "", err
	}
	data := mapValue(response, "aigc_data")
	historyID := fmt.Sprintf("%v", data["history_record_id"])
	if historyID == "" {
		return "", errors.ErrAPIImageGenerationFailed("记录ID不存在")
	}

	return historyID, nil
}

// PollImageResult 轮询图片生成结果
func PollImageResult(historyID string, refreshToken string, expectedCount int) ([]string, error) {
	finalData, pollResult, err := pollHistory(historyID, refreshToken, &poller.PollingOptions{
		ExpectedItemCount: expectedCount,
		MaxPollCount:      900,
		Type:              "image",
	}, standardImageInfo())
	if err != nil {
		return nil, err
	}
	urls := utils.ExtractImageUrls(finalData["item_list"])
	if len(urls) == 0 && len(sliceValue(finalData["item_list"])) > 0 {
		return nil, errors.ErrAPIImageGenerationFailed(
			fmt.Sprintf("图像生成失败: item_list有 %d 个项目，但无法提取任何图片URL，所有item都缺少 image.large_images[0].image_url 字段",
				len(sliceValue(finalData["item_list"]))),
		)
	}
	logger.Info(fmt.Sprintf("图像生成完成: %d 张，耗时 %.1fs", len(urls), pollResult.ElapsedTime))
	return urls, nil
}

func submitJimeng40MultiImages(mappedModel, requestedModel, prompt string, opts *ImageOptions, refreshToken string, region *RegionInfo) (string, error) {
	targetCount := extractTargetCount(prompt)
	params, err := resolveResolutionForModel(requestedModel, opts)
	if err != nil {
		return "", err
	}
	componentID := utils.UUID(true)
	submitID := utils.UUID(true)
	metrics := mustJSON(map[string]interface{}{
		"templateId":      "",
		"generateCount":   1,
		"promptSource":    "custom",
		"templateSource":  "",
		"lastRequestId":   "",
		"originRequestId": "",
	})

	coreParam := map[string]interface{}{
		"type":              "",
		"id":                utils.UUID(true),
		"model":             mappedModel,
		"prompt":            prompt,
		"negative_prompt":   opts.NegativePrompt,
		"seed":              randomSeed(),
		"sample_strength":   opts.SampleStrength,
		"large_image_info":  map[string]interface{}{"type": "", "id": utils.UUID(true), "height": params.Height, "width": params.Width, "resolution_type": params.ResolutionType},
		"intelligent_ratio": opts.IntelligentRatio,
	}
	if !opts.IntelligentRatio {
		coreParam["image_ratio"] = params.Ratio
	}

	logger.Info(fmt.Sprintf("使用 多图生成: %d张图片 %dx%d 精细度: %.2f", targetCount, params.Width, params.Height, opts.SampleStrength))

	component := buildGenerateComponent(componentID, coreParam)
	draft := buildDraft(componentID, component)

	payload := map[string]interface{}{
		"extend": map[string]interface{}{
			"root_model": mappedModel,
		},
		"submit_id":     submitID,
		"metrics_extra": metrics,
		"draft_content": string(mustJSONBytes(draft)),
		"http_common_info": map[string]interface{}{
			"aid": GetAssistantID(region),
		},
	}

	response, err := Request("POST", "/mweb/v1/aigc_draft/generate", refreshToken, &RequestOptions{Body: payload})
	if err != nil {
		return "", err
	}
	data := mapValue(response, "aigc_data")
	historyID := fmt.Sprintf("%v", data["history_record_id"])
	if historyID == "" {
		return "", errors.ErrAPIImageGenerationFailed("记录ID不存在")
	}

	return historyID, nil
}

func ensureImageOptionDefaults(opts *ImageOptions) {
	if opts.Ratio == "" {
		opts.Ratio = "1:1"
	}
	if opts.Resolution == "" {
		opts.Resolution = "2k"
	}
	if opts.SampleStrength <= 0 {
		opts.SampleStrength = 0.5
	}
}

func resolveResolutionForModel(requestedModel string, opts *ImageOptions) (consts.ResolutionParams, error) {
	if requestedModel == "nanobanana" {
		logger.Warn("nanobanana模型当前固定使用1024x1024分辨率和2k的清晰度，您输入的参数将被忽略。")
		return consts.ResolutionParams{
			Width:          1024,
			Height:         1024,
			Ratio:          1,
			ResolutionType: "2k",
		}, nil
	}
	return GetResolutionParams(opts.Resolution, opts.Ratio)
}

func buildComponentMetadata() map[string]interface{} {
	return map[string]interface{}{
		"type":                     "",
		"id":                       utils.UUID(true),
		"created_platform":         3,
		"created_platform_version": "",
		"created_time_in_ms":       fmt.Sprintf("%d", time.Now().UnixMilli()),
		"created_did":              "",
	}
}

func buildBlendAbilities(uploadIDs []string, strength float64) []map[string]interface{} {
	abilityList := make([]map[string]interface{}, 0, len(uploadIDs))
	for _, uri := range uploadIDs {
		abilityList = append(abilityList, map[string]interface{}{
			"type":           "",
			"id":             utils.UUID(true),
			"name":           "byte_edit",
			"image_uri_list": []string{uri},
			"image_list": []map[string]interface{}{
				{
					"type":          "image",
					"id":            utils.UUID(true),
					"source_from":   "upload",
					"platform_type": 1,
					"name":          "",
					"image_uri":     uri,
					"width":         0,
					"height":        0,
					"format":        "",
					"uri":           uri,
				},
			},
			"strength": strength,
		})
	}
	return abilityList
}

func buildBlendComponent(componentID string, abilityList []map[string]interface{}, coreParam map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":          "image_base_component",
		"id":            componentID,
		"min_version":   consts.DraftMinVersion,
		"aigc_mode":     "workbench",
		"metadata":      buildComponentMetadata(),
		"generate_type": "blend",
		"abilities": map[string]interface{}{
			"type": "",
			"id":   utils.UUID(true),
			"blend": map[string]interface{}{
				"type":                         "",
				"id":                           utils.UUID(true),
				"min_features":                 []interface{}{},
				"core_param":                   coreParam,
				"ability_list":                 abilityList,
				"prompt_placeholder_info_list": buildPlaceholderInfo(len(abilityList)),
				"postedit_param":               map[string]interface{}{"type": "", "id": utils.UUID(true), "generate_type": 0},
			},
		},
	}
}

func buildGenerateComponent(componentID string, coreParam map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":          "image_base_component",
		"id":            componentID,
		"min_version":   consts.DraftMinVersion,
		"aigc_mode":     "workbench",
		"metadata":      buildComponentMetadata(),
		"generate_type": "generate",
		"abilities": map[string]interface{}{
			"type": "",
			"id":   utils.UUID(true),
			"generate": map[string]interface{}{
				"type":       "",
				"id":         utils.UUID(true),
				"core_param": coreParam,
			},
		},
	}
}

func buildDraft(componentID string, component map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":              "draft",
		"id":                utils.UUID(true),
		"min_version":       consts.DraftMinVersion,
		"min_features":      []interface{}{},
		"is_from_tsn":       true,
		"version":           consts.DraftVersion,
		"main_component_id": componentID,
		"component_list":    []map[string]interface{}{component},
	}
}

func ensureCredit(refreshToken string) (*CreditInfo, error) {
	return GetCredit(refreshToken)
}

func shouldUseMultiImage(model, prompt string) bool {
	if model != "jimeng-4.0" {
		return false
	}
	for _, keyword := range multiImageKeywords {
		if strings.Contains(prompt, keyword) {
			return true
		}
	}
	return multiImageCountRegex.MatchString(prompt)
}

func extractTargetCount(prompt string) int {
	matches := multiImageCountRegex.FindStringSubmatch(prompt)
	if len(matches) >= 2 {
		if val, err := strconv.Atoi(matches[1]); err == nil && val > 0 {
			return val
		}
	}
	return 4
}

func pollHistory(historyID, refreshToken string, pollOptions *poller.PollingOptions, imageInfo map[string]interface{}) (map[string]interface{}, *poller.PollingResult, error) {
	if historyID == "" {
		return nil, nil, errors.ErrAPIImageGenerationFailed("记录ID不存在")
	}
	options := &poller.PollingOptions{ExpectedItemCount: 1, Type: "image"}
	if pollOptions != nil {
		*options = *pollOptions
		if options.Type == "" {
			options.Type = "image"
		}
	}
	if imageInfo == nil {
		imageInfo = standardImageInfo()
	}
	smartPoller := poller.NewSmartPoller(options)

	result, data, err := poller.Poll(smartPoller, func() (*poller.PollingStatus, map[string]interface{}, error) {
		response, err := Request("POST", "/mweb/v1/get_history_by_ids", refreshToken, &RequestOptions{
			Body: map[string]interface{}{
				"history_ids": []string{historyID},
				"image_info":  imageInfo,
			},
		})
		if err != nil {
			return nil, nil, err
		}

		taskRaw, ok := response[historyID]
		if !ok {
			logger.Error(fmt.Sprintf("历史记录不存在: historyId=%s", historyID))
			return nil, nil, errors.ErrAPIImageGenerationFailed("记录不存在")
		}
		task, ok := taskRaw.(map[string]interface{})
		if !ok {
			return nil, nil, errors.ErrAPIImageGenerationFailed("记录数据结构异常")
		}

		status := int(numberValue(task["status"]))
		failCode := fmt.Sprintf("%v", task["fail_code"])
		items := sliceValue(task["item_list"])
		finishTime := int64(0)
		if taskValue, ok := task["task"].(map[string]interface{}); ok {
			finishTime = int64(numberValue(taskValue["finish_time"]))
		}

		return &poller.PollingStatus{
			Status:     status,
			FailCode:   failCode,
			ItemCount:  len(items),
			FinishTime: finishTime,
			HistoryID:  historyID,
		}, task, nil
	}, historyID)
	return data, result, err
}

func standardImageInfo() map[string]interface{} {
	return map[string]interface{}{
		"width":  2048,
		"height": 2048,
		"format": "webp",
		"image_scene_list": []map[string]interface{}{
			sceneEntry("smart_crop", 360, 360, "smart_crop-w:360-h:360"),
			sceneEntry("smart_crop", 480, 480, "smart_crop-w:480-h:480"),
			sceneEntry("smart_crop", 720, 720, "smart_crop-w:720-h:720"),
			sceneEntry("smart_crop", 720, 480, "smart_crop-w:720-h:480"),
			sceneEntry("smart_crop", 360, 240, "smart_crop-w:360-h:240"),
			sceneEntry("smart_crop", 240, 320, "smart_crop-w:240-h:320"),
			sceneEntry("smart_crop", 480, 640, "smart_crop-w:480-h:640"),
			sceneEntry("normal", 2400, 2400, "2400"),
			sceneEntry("normal", 1080, 1080, "1080"),
			sceneEntry("normal", 720, 720, "720"),
			sceneEntry("normal", 480, 480, "480"),
			sceneEntry("normal", 360, 360, "360"),
		},
	}
}

func blendImageInfo() map[string]interface{} {
	return map[string]interface{}{
		"width":  2048,
		"height": 2048,
		"format": "webp",
		"image_scene_list": []map[string]interface{}{
			sceneEntry("smart_crop", 360, 360, "smart_crop-w:360-h:360"),
			sceneEntry("smart_crop", 480, 480, "smart_crop-w:480-h:480"),
			sceneEntry("smart_crop", 720, 720, "smart_crop-w:720-h:720"),
			sceneEntry("smart_crop", 720, 480, "smart_crop-w:720-h:480"),
			sceneEntry("normal", 2400, 2400, "2400"),
			sceneEntry("normal", 1080, 1080, "1080"),
			sceneEntry("normal", 720, 720, "720"),
			sceneEntry("normal", 480, 480, "480"),
			sceneEntry("normal", 360, 360, "360"),
		},
	}
}

func sceneEntry(scene string, width, height int, key string) map[string]interface{} {
	return map[string]interface{}{
		"scene":    scene,
		"width":    width,
		"height":   height,
		"uniq_key": key,
		"format":   "webp",
	}
}

func buildPlaceholderInfo(count int) []map[string]interface{} {
	placeholders := make([]map[string]interface{}, 0, count)
	for i := 0; i < count; i++ {
		placeholders = append(placeholders, map[string]interface{}{
			"type":          "",
			"id":            utils.UUID(true),
			"ability_index": i,
		})
	}
	return placeholders
}

func randomSeed() int64 {
	return int64(math.Floor(rand.Float64()*100000000) + 2500000000)
}

func uploadImageSource(exec uploader.RequestFunc, image interface{}, refreshToken string, region *RegionInfo) (string, error) {
	switch value := image.(type) {
	case []byte:
		return uploader.UploadImageBuffer(exec, value, refreshToken, region)
	case string:
		return uploader.UploadImageFromURL(exec, value, refreshToken, region)
	default:
		return "", fmt.Errorf("不支持的图片输入类型: %T", image)
	}
}

func adaptRequestForUploader() uploader.RequestFunc {
	return func(method, uri, refreshToken string, options *uploader.RequestOptions) (map[string]interface{}, error) {
		var reqOpts *RequestOptions
		if options != nil {
			reqOpts = &RequestOptions{
				Headers:         options.Headers,
				Params:          options.Params,
				Body:            options.Body,
				NoDefaultParams: options.NoDefaultParams,
			}
		}
		return Request(method, uri, refreshToken, reqOpts)
	}
}

func mustJSON(data map[string]interface{}) string {
	return string(mustJSONBytes(data))
}

func mustJSONBytes(data interface{}) []byte {
	bytes, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return bytes
}

func sliceValue(v interface{}) []interface{} {
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return []interface{}{}
}

func sortedMapKeys(m map[string]map[string]consts.ResolutionParams) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedResolutionKeys(m map[string]consts.ResolutionParams) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedModelKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

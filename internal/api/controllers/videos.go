package controllers

import (
	"encoding/json"
	"fmt"
	"regexp"
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

const defaultVideoModel = consts.DefaultVideoModel

// VideoOptions 视频生成选项
type VideoOptions struct {
	Ratio       string
	Resolution  string
	Duration    int
	FilePaths   []string
	FileBuffers [][]byte
}

// GenerateVideo 文生视频
func GenerateVideo(model string, prompt string, opts *VideoOptions, refreshToken string) (string, error) {
	tm := task.NewTaskManager()
	result, err := tm.ExecuteTask(
		func() (string, error) {
			return SubmitVideoGeneration(model, prompt, opts, refreshToken)
		},
		func(taskID string) (interface{}, error) {
			return PollVideoResult(taskID, refreshToken)
		},
	)
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

// SubmitVideoGeneration 提交视频生成任务
func SubmitVideoGeneration(model string, prompt string, opts *VideoOptions, refreshToken string) (string, error) {
	if opts == nil {
		opts = &VideoOptions{}
	}
	if strings.TrimSpace(opts.Ratio) == "" {
		opts.Ratio = "1:1"
	}
	if strings.TrimSpace(opts.Resolution) == "" {
		opts.Resolution = "720p"
	}
	region := ParseRegionFromToken(refreshToken)
	mappedModel := getVideoModel(model, region)

	// 判断模型类型
	isVeo3 := strings.Contains(mappedModel, "veo3")
	isSora2 := strings.Contains(mappedModel, "sora2")
	is35Pro := strings.Contains(mappedModel, "3.5_pro")
	is40 := strings.Contains(mappedModel, "40") || strings.Contains(mappedModel, "seedance_40")
	// 只有 video-3.0 和 video-3.0-fast 支持 resolution 参数（3.0-pro 和 3.5-pro 不支持）
	supportsResolution := (strings.Contains(mappedModel, "vgfm_3.0") || strings.Contains(mappedModel, "vgfm_3.0_fast")) && !strings.Contains(mappedModel, "_pro")

	// 计算实际时长
	var durationMS int
	var actualDuration int
	if isVeo3 {
		// VEO3 模型固定 8 秒
		durationMS = 8000
		actualDuration = 8
	} else if isSora2 {
		// Sora2 模型支持 4/8/12 秒
		switch opts.Duration {
		case 12:
			durationMS = 12000
			actualDuration = 12
		case 8:
			durationMS = 8000
			actualDuration = 8
		default:
			durationMS = 4000
			actualDuration = 4
		}
	} else if is35Pro {
		// 3.5-pro 模型支持 5/10/12 秒
		switch opts.Duration {
		case 12:
			durationMS = 12000
			actualDuration = 12
		case 10:
			durationMS = 10000
			actualDuration = 10
		default:
			durationMS = 5000
			actualDuration = 5
		}
	} else if is40 {
		// 4.0 模型支持 5/10/15 秒
		switch opts.Duration {
		case 15:
			durationMS = 15000
			actualDuration = 15
		case 10:
			durationMS = 10000
			actualDuration = 10
		default:
			durationMS = 5000
			actualDuration = 5
		}
	} else {
		// 其他模型支持 5/10 秒
		if opts.Duration == 10 {
			durationMS = 10000
			actualDuration = 10
		} else {
			durationMS = 5000
			actualDuration = 5
		}
	}

	resolutionStr := "不支持"
	if supportsResolution {
		resolutionStr = opts.Resolution
	}
	logger.Info(fmt.Sprintf("使用模型: %s 映射模型: %s 比例: %s 分辨率: %s 时长: %ds", model, mappedModel, opts.Ratio, resolutionStr, actualDuration))

	credit, err := ensureCredit(refreshToken)
	if err != nil {
		logger.Warn(fmt.Sprintf("获取积分失败: %v", err))
	} else if credit.TotalCredit <= 0 {
		_, _ = ReceiveCredit(refreshToken)
	}

	uploadIDs := make([]string, 0)
	exec := adaptRequestForUploader()
	for _, buf := range opts.FileBuffers {
		if buf == nil {
			continue
		}
		id, err := uploader.UploadImageBuffer(exec, buf, refreshToken, region)
		if err != nil {
			return "", errors.ErrFileUploadFailed(fmt.Sprintf("上传本地图片失败: %v", err))
		}
		uploadIDs = append(uploadIDs, id)
	}
	for _, path := range opts.FilePaths {
		if path == "" {
			continue
		}
		id, err := uploader.UploadImageFromURL(exec, path, refreshToken, region)
		if err != nil {
			return "", errors.ErrFileUploadFailed(fmt.Sprintf("上传URL图片失败: %v", err))
		}
		uploadIDs = append(uploadIDs, id)
	}

	var firstFrame, endFrame map[string]interface{}
	if len(uploadIDs) > 0 {
		firstFrame = buildVideoFrame(uploadIDs[0])
	}
	if len(uploadIDs) > 1 {
		endFrame = buildVideoFrame(uploadIDs[1])
	}

	// 构建 genInputs
	genInput := map[string]interface{}{
		"type":              "",
		"id":                utils.UUID(true),
		"min_version":       "3.0.5",
		"prompt":            prompt,
		"video_mode":        2,
		"fps":               24,
		"duration_ms":       durationMS,
		"first_frame_image": firstFrame,
		"end_frame_image":   endFrame,
		"idip_meta_list":    []interface{}{},
	}
	// 只有支持的模型才传递 resolution
	if supportsResolution {
		genInput["resolution"] = opts.Resolution
	}
	genInputs := []map[string]interface{}{genInput}

	componentID := utils.UUID(true)
	originSubmitID := utils.UUID(true)

	// 构建 sceneOption
	sceneOption := map[string]interface{}{
		"type":          "video",
		"scene":         "BasicVideoGenerateButton",
		"modelReqKey":   mappedModel,
		"videoDuration": actualDuration,
		"reportParams": map[string]interface{}{
			"enterSource":                     "generate",
			"vipSource":                       "generate",
			"extraVipFunctionKey":             mappedModel,
			"useVipFunctionDetailsReporterHoc": true,
		},
	}
	if supportsResolution {
		sceneOption["resolution"] = opts.Resolution
		sceneOption["reportParams"].(map[string]interface{})["extraVipFunctionKey"] = fmt.Sprintf("%s-%s", mappedModel, opts.Resolution)
	}

	sceneOptionsJSON, _ := json.Marshal([]map[string]interface{}{sceneOption})
	metrics := mustJSON(map[string]interface{}{
		"promptSource":   "custom",
		"isDefaultSeed":  1,
		"originSubmitId": originSubmitID,
		"isRegenerate":   false,
		"enterFrom":      "click",
		"functionMode":   "first_last_frames",
		"sceneOptions":   string(sceneOptionsJSON),
	})

	draft := map[string]interface{}{
		"type":              "draft",
		"id":                utils.UUID(true),
		"min_version":       "3.0.5",
		"min_features":      []interface{}{},
		"is_from_tsn":       true,
		"version":           consts.DraftVersion,
		"main_component_id": componentID,
		"component_list": []map[string]interface{}{
			{
				"type":          "video_base_component",
				"id":            componentID,
				"min_version":   "1.0.0",
				"aigc_mode":     "workbench",
				"generate_type": "gen_video",
				"metadata": map[string]interface{}{
					"type":                     "",
					"id":                       utils.UUID(true),
					"created_platform":         3,
					"created_platform_version": "",
					"created_time_in_ms":       fmt.Sprintf("%d", time.Now().UnixMilli()),
					"created_did":              "",
				},
				"abilities": map[string]interface{}{
					"type": "",
					"id":   utils.UUID(true),
					"gen_video": map[string]interface{}{
						"id":   utils.UUID(true),
						"type": "",
						"text_to_video_params": map[string]interface{}{
							"type":               "",
							"id":                 utils.UUID(true),
							"video_gen_inputs":   genInputs,
							"video_aspect_ratio": opts.Ratio,
							"seed":               randomSeed(),
							"model_req_key":      mappedModel,
							"priority":           0,
						},
						"video_task_extra": metrics,
					},
				},
			},
		},
	}

	// 始终使用映射后的 model 作为 root_model
	benefitType := getVideoBenefitType(mappedModel)

	payload := map[string]interface{}{
		"extend": map[string]interface{}{
			"root_model": mappedModel,
			"m_video_commerce_info": map[string]interface{}{
				"benefit_type":      benefitType,
				"resource_id":       "generate_video",
				"resource_id_type":  "str",
				"resource_sub_type": "aigc",
			},
			"m_video_commerce_info_list": []map[string]interface{}{
				{
					"benefit_type":      benefitType,
					"resource_id":       "generate_video",
					"resource_id_type":  "str",
					"resource_sub_type": "aigc",
				},
			},
		},
		"submit_id":        utils.UUID(true),
		"metrics_extra":    metrics,
		"draft_content":    string(mustJSONBytes(draft)),
		"http_common_info": map[string]interface{}{"aid": GetAssistantID(region)},
	}

	response, err := Request("POST", "/mweb/v1/aigc_draft/generate", refreshToken, &RequestOptions{Body: payload})
	if err != nil {
		return "", err
	}
	data := mapValue(response, "aigc_data")
	historyID := fmt.Sprintf("%v", data["history_record_id"])
	if historyID == "" {
		return "", errors.ErrAPIVideoGenerationFailed("记录ID不存在")
	}

	return historyID, nil
}

// PollVideoResult 轮询视频生成结果
func PollVideoResult(historyID string, refreshToken string) (string, error) {
	logger.Info(fmt.Sprintf("视频生成任务已提交，history_id: %s，等待生成完成...", historyID))
	time.Sleep(5 * time.Second)

	finalData, err := pollVideoHistory(historyID, refreshToken)
	if err != nil {
		return "", err
	}

	// 调试日志：输出完整的 finalData
	finalDataJSON, _ := json.Marshal(finalData)
	logger.Info(fmt.Sprintf("轮询结果 finalData: %s", string(finalDataJSON)))

	items := sliceValue(finalData["item_list"])
	logger.Info(fmt.Sprintf("items 数量: %d", len(items)))

	if len(items) == 0 {
		return "", errors.ErrAPIVideoGenerationFailed("生成结果为空")
	}

	// 调试日志：输出 items[0] 的内容
	itemJSON, _ := json.Marshal(items[0])
	logger.Info(fmt.Sprintf("items[0] 内容: %s", string(itemJSON)))

	videoURL := utils.ExtractVideoUrl(items[0])
	logger.Info(fmt.Sprintf("提取的视频URL: %s", videoURL))

	if videoURL == "" {
		return "", errors.ErrAPIVideoGenerationFailed("未能提取视频链接")
	}
	logger.Info(fmt.Sprintf("视频生成成功: %s", videoURL))
	return videoURL, nil
}

// getVideoModel 根据区域获取视频模型映射
func getVideoModel(model string, region *RegionInfo) string {
	if model == "" {
		model = defaultVideoModel
	}
	// 根据区域选择不同的模型映射
	var modelMap map[string]string
	if region.IsUS {
		modelMap = consts.VideoModelMapUS
	} else if region.IsHK || region.IsJP || region.IsSG {
		modelMap = consts.VideoModelMapAsia
	} else {
		modelMap = consts.VideoModelMap
	}
	if mapped, ok := modelMap[model]; ok {
		return mapped
	}
	// 如果在当前区域映射中找不到，尝试使用默认模型
	if mapped, ok := modelMap[defaultVideoModel]; ok {
		return mapped
	}
	// 最后回退到全局默认
	return consts.VideoModelMap[defaultVideoModel]
}

// getVideoBenefitType 根据模型获取扣费类型
func getVideoBenefitType(model string) string {
	// veo3.1 模型 (需先于 veo3 检查)
	if strings.Contains(model, "veo3.1") {
		return "generate_video_veo3.1"
	}
	// veo3 模型
	if strings.Contains(model, "veo3") {
		return "generate_video_veo3"
	}
	// sora2 模型
	if strings.Contains(model, "sora2") {
		return "generate_video_sora2"
	}
	// 4.0 pro 模型 (seedance_40_pro)
	if strings.Contains(model, "40_pro") || strings.Contains(model, "seedance_40_pro") {
		return "dreamina_video_seedance_20_pro"
	}
	// 4.0 模型 (seedance_40)
	if strings.Contains(model, "40") || strings.Contains(model, "seedance_40") {
		return "dreamina_video_seedance_20"
	}
	// 3.5 pro 模型
	if strings.Contains(model, "3.5_pro") {
		return "dreamina_video_seedance_15_pro"
	}
	// 3.5 模型
	if strings.Contains(model, "3.5") {
		return "dreamina_video_seedance_15"
	}
	return "basic_video_operation_vgfm_v_three"
}

func buildVideoFrame(uri string) map[string]interface{} {
	return map[string]interface{}{
		"format":        "",
		"height":        0,
		"width":         0,
		"id":            utils.UUID(true),
		"image_uri":     uri,
		"type":          "image",
		"platform_type": 1,
		"source_from":   "upload",
		"uri":           uri,
	}
}

func pollVideoHistory(historyID string, refreshToken string) (map[string]interface{}, error) {
	smartPoller := poller.NewSmartPoller(&poller.PollingOptions{
		ExpectedItemCount: 1,
		Type:              "video",
		MaxPollCount:      900,
		TimeoutSeconds:    1200,
	})

	// 视频URL正则匹配模式
	videoURLPattern := regexp.MustCompile(`https://v[0-9]+-artist\.vlabvod\.com/[^"\s]+`)

	result, data, err := poller.Poll(smartPoller, func() (*poller.PollingStatus, map[string]interface{}, error) {
		response, err := Request("POST", "/mweb/v1/get_history_by_ids", refreshToken, &RequestOptions{
			Body: map[string]interface{}{"history_ids": []string{historyID}},
		})
		if err != nil {
			return nil, nil, err
		}

		// 尝试直接从响应中正则匹配视频URL
		responseBytes, _ := json.Marshal(response)
		if match := videoURLPattern.FindString(string(responseBytes)); match != "" {
			logger.Info(fmt.Sprintf("从API响应中直接提取到视频URL: %s", match))
			return &poller.PollingStatus{
				Status:    10, // SUCCESS
				ItemCount: 1,
				HistoryID: historyID,
			}, map[string]interface{}{
				"status": 10,
				"item_list": []interface{}{
					map[string]interface{}{
						"video": map[string]interface{}{
							"transcoded_video": map[string]interface{}{
								"origin": map[string]interface{}{
									"video_url": match,
								},
							},
						},
					},
				},
			}, nil
		}

		taskData := mapValue(response, historyID)
		// 检查是否有该 history_id 的数据
		if len(taskData) == 0 {
			logger.Warn(fmt.Sprintf("API未返回历史记录，historyId: %s，继续等待...", historyID))
			return &poller.PollingStatus{
				Status:    20, // PROCESSING
				ItemCount: 0,
				HistoryID: historyID,
			}, map[string]interface{}{"status": 20, "item_list": []interface{}{}}, nil
		}

		status := int(numberValue(taskData["status"]))
		failCode := fmt.Sprintf("%v", taskData["fail_code"])
		items := sliceValue(taskData["item_list"])
		finishTime := int64(numberValue(mapValue(taskData, "task")["finish_time"]))
		return &poller.PollingStatus{
			Status:     status,
			FailCode:   failCode,
			ItemCount:  len(items),
			FinishTime: finishTime,
			HistoryID:  historyID,
		}, taskData, nil
	}, historyID)
	if err != nil {
		return nil, err
	}
	logger.Info(fmt.Sprintf("视频生成完成，耗时 %.1fs", result.ElapsedTime))
	return data, nil
}

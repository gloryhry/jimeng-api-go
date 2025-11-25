package controllers

import (
	"fmt"
	"strings"
	"time"

	"github.com/gloryhry/jimeng-api-go/internal/api/consts"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/errors"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/poller"
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
	if opts == nil {
		opts = &VideoOptions{}
	}
	if strings.TrimSpace(opts.Ratio) == "" {
		opts.Ratio = "1:1"
	}
	if strings.TrimSpace(opts.Resolution) == "" {
		opts.Resolution = "720p"
	}
	if opts.Duration != 10 {
		opts.Duration = 5
	}
	region := ParseRegionFromToken(refreshToken)
	mappedModel := getVideoModel(model)
	logger.Info(fmt.Sprintf("开始生成视频，模型: %s -> %s", model, mappedModel))

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

	durationMS := 5000
	if opts.Duration == 10 {
		durationMS = 10000
	}

	genInputs := []map[string]interface{}{
		{
			"type":              "",
			"id":                utils.UUID(true),
			"min_version":       "3.0.5",
			"prompt":            prompt,
			"video_mode":        2,
			"fps":               24,
			"duration_ms":       durationMS,
			"resolution":        opts.Resolution,
			"first_frame_image": firstFrame,
			"end_frame_image":   endFrame,
			"idip_meta_list":    []interface{}{},
		},
	}

	componentID := utils.UUID(true)
	metrics := mustJSON(map[string]interface{}{
		"promptSource":   "custom",
		"isDefaultSeed":  1,
		"originSubmitId": utils.UUID(true),
		"isRegenerate":   false,
		"enterFrom":      "click",
		"functionMode":   "first_last_frames",
	})

	draft := map[string]interface{}{
		"type":              "draft",
		"id":                utils.UUID(true),
		"min_version":       "3.0.5",
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
					"type":               "",
					"id":                 utils.UUID(true),
					"created_platform":   3,
					"created_time_in_ms": fmt.Sprintf("%d", time.Now().UnixMilli()),
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

	rootModel := mappedModel
	if endFrame != nil {
		rootModel = consts.VideoModelMap["jimeng-video-3.0"]
	}

	payload := map[string]interface{}{
		"extend": map[string]interface{}{
			"root_model": rootModel,
			"m_video_commerce_info": map[string]interface{}{
				"benefit_type":      "basic_video_operation_vgfm_v_three",
				"resource_id":       "generate_video",
				"resource_id_type":  "str",
				"resource_sub_type": "aigc",
			},
			"m_video_commerce_info_list": []map[string]interface{}{
				{
					"benefit_type":      "basic_video_operation_vgfm_v_three",
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

	logger.Info(fmt.Sprintf("视频生成任务已提交，history_id: %s，等待生成完成...", historyID))
	time.Sleep(5 * time.Second)

	finalData, err := pollVideoHistory(historyID, refreshToken)
	if err != nil {
		return "", err
	}
	items := sliceValue(finalData["item_list"])
	if len(items) == 0 {
		return "", errors.ErrAPIVideoGenerationFailed("生成结果为空")
	}
	videoURL := utils.ExtractVideoUrl(items[0])
	if videoURL == "" {
		return "", errors.ErrAPIVideoGenerationFailed("未能提取视频链接")
	}
	logger.Info(fmt.Sprintf("视频生成成功: %s", videoURL))
	return videoURL, nil
}

func getVideoModel(model string) string {
	if model == "" {
		model = defaultVideoModel
	}
	if mapped, ok := consts.VideoModelMap[model]; ok {
		return mapped
	}
	return consts.VideoModelMap[defaultVideoModel]
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

	result, data, err := poller.Poll(smartPoller, func() (*poller.PollingStatus, map[string]interface{}, error) {
		response, err := Request("POST", "/mweb/v1/get_history_by_ids", refreshToken, &RequestOptions{
			Body: map[string]interface{}{"history_ids": []string{historyID}},
		})
		if err != nil {
			return nil, nil, err
		}
		task := mapValue(response, historyID)
		status := int(numberValue(task["status"]))
		failCode := fmt.Sprintf("%v", task["fail_code"])
		items := sliceValue(task["item_list"])
		finishTime := int64(numberValue(mapValue(task, "task")["finish_time"]))
		return &poller.PollingStatus{
			Status:     status,
			FailCode:   failCode,
			ItemCount:  len(items),
			FinishTime: finishTime,
			HistoryID:  historyID,
		}, task, nil
	}, historyID)
	if err != nil {
		return nil, err
	}
	logger.Info(fmt.Sprintf("视频生成完成，耗时 %.1fs", result.ElapsedTime))
	return data, nil
}

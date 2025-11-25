package poller

import (
	"fmt"
	"time"

	"github.com/gloryhry/jimeng-api-go/internal/api/consts"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/errors"
	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
)

// PollingStatus 轮询状态
type PollingStatus struct {
	Status     int    `json:"status"`
	FailCode   string `json:"failCode,omitempty"`
	ItemCount  int    `json:"itemCount"`
	FinishTime int64  `json:"finishTime,omitempty"`
	HistoryID  string `json:"historyId,omitempty"`
}

// PollingOptions 轮询配置
type PollingOptions struct {
	MaxPollCount      int
	PollInterval      int // 毫秒
	StableRounds      int
	TimeoutSeconds    int
	ExpectedItemCount int
	Type              string // "image" 或 "video"
}

// PollingResult 轮询结果
type PollingResult struct {
	Status      int
	FailCode    string
	ItemCount   int
	ElapsedTime float64
	PollCount   int
	ExitReason  string
}

// SmartPoller 智能轮询器
type SmartPoller struct {
	pollCount            int
	startTime            time.Time
	lastItemCount        int
	stableItemCountRounds int
	maxPollCount         int
	pollInterval         time.Duration
	stableRounds         int
	timeoutSeconds       int
	expectedItemCount    int
	itemType             string
}

// NewSmartPoller 创建智能轮询器
func NewSmartPoller(options *PollingOptions) *SmartPoller {
	if options == nil {
		options = &PollingOptions{}
	}

	maxPollCount := options.MaxPollCount
	if maxPollCount == 0 {
		maxPollCount = consts.MaxPollCount
	}

	pollInterval := options.PollInterval
	if pollInterval == 0 {
		pollInterval = consts.PollInterval
	}

	stableRounds := options.StableRounds
	if stableRounds == 0 {
		stableRounds = consts.StableRounds
	}

	timeoutSeconds := options.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = consts.TimeoutSeconds
	}

	itemType := options.Type
	if itemType == "" {
		itemType = "image"
	}

	return &SmartPoller{
		pollCount:            0,
		startTime:            time.Now(),
		lastItemCount:        0,
		stableItemCountRounds: 0,
		maxPollCount:         maxPollCount,
		pollInterval:         time.Duration(pollInterval) * time.Millisecond,
		stableRounds:         stableRounds,
		timeoutSeconds:       timeoutSeconds,
		expectedItemCount:    options.ExpectedItemCount,
		itemType:             itemType,
	}
}

// GetStatusName 获取状态名称
func (p *SmartPoller) GetStatusName(status int) string {
	if name, ok := consts.StatusCodeMap[status]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", status)
}

// GetSmartInterval 根据状态码计算智能轮询间隔
func (p *SmartPoller) GetSmartInterval(status, itemCount int) time.Duration {
	switch status {
	case 20: // PROCESSING
		if itemCount > 0 {
			// 有进度，缩短间隔
			return 3 * time.Second
		}
		// 无进度，正常间隔
		return 5 * time.Second

	case 42, 45: // POST_PROCESSING, FINALIZING
		// 接近完成，更频繁检查
		return 2 * time.Second

	default:
		return 5 * time.Second
	}
}

// ShouldExitPolling 检查是否应该退出轮询
func (p *SmartPoller) ShouldExitPolling(pollingStatus *PollingStatus) (bool, string) {
	status := pollingStatus.Status
	itemCount := pollingStatus.ItemCount
	failCode := pollingStatus.FailCode

	// 1. 成功状态 (status=10 或 50)
	if status == 10 || status == 50 {
		if itemCount > 0 {
			return true, fmt.Sprintf("生成成功，状态: %s, 数量: %d", p.GetStatusName(status), itemCount)
		}
	}

	// 2. 明确失败状态 (status=30)
	if status == 30 {
		return true, fmt.Sprintf("生成失败，状态: %s, 错误码: %s", p.GetStatusName(status), failCode)
	}

	// 3. 达到预期数量 (如果设置了)
	if p.expectedItemCount > 0 && itemCount >= p.expectedItemCount {
		return true, fmt.Sprintf("已达到预期数量: %d", itemCount)
	}

	// 4. 项目计数稳定轮次检查
	if itemCount > 0 && itemCount == p.lastItemCount {
		p.stableItemCountRounds++
		if p.stableItemCountRounds >= p.stableRounds {
			if status == 10 || status == 50 {
				return true, fmt.Sprintf("状态已稳定，数量: %d", itemCount)
			}
		}
	} else {
		// 计数变化，重置稳定轮次
		p.stableItemCountRounds = 0
		p.lastItemCount = itemCount
	}

	// 5. 超时检查
	elapsed := time.Since(p.startTime).Seconds()
	if elapsed > float64(p.timeoutSeconds) {
		return true, fmt.Sprintf("轮询超时 (%.0fs)", elapsed)
	}

	// 6. 最大轮询次数检查
	if p.pollCount >= p.maxPollCount {
		return true, fmt.Sprintf("达到最大轮询次数: %d", p.maxPollCount)
	}

	return false, ""
}

// Poll 执行轮询
func Poll[T any](p *SmartPoller, pollFunction func() (*PollingStatus, T, error), historyID string) (*PollingResult, T, error) {
	var zeroValue T

	for {
		p.pollCount++
		elapsed := time.Since(p.startTime).Seconds()

		logger.Info(fmt.Sprintf("轮询第 %d 次 (耗时: %.1fs)%s",
			p.pollCount, elapsed,
			func() string {
				if historyID != "" {
					return fmt.Sprintf(", historyId: %s", historyID)
				}
				return ""
			}()))

		// 执行轮询函数
		pollingStatus, data, err := pollFunction()
		if err != nil {
			return nil, zeroValue, err
		}

		status := pollingStatus.Status
		itemCount := pollingStatus.ItemCount
		failCode := pollingStatus.FailCode

		logger.Debug(fmt.Sprintf("轮询状态: %s, 项目数: %d, 错误码: %s",
			p.GetStatusName(status), itemCount, failCode))

		// 检查是否应该退出
		shouldExit, reason := p.ShouldExitPolling(pollingStatus)
		if shouldExit {
			logger.Info(fmt.Sprintf("轮询退出: %s", reason))

			result := &PollingResult{
				Status:      status,
				FailCode:    failCode,
				ItemCount:   itemCount,
				ElapsedTime: elapsed,
				PollCount:   p.pollCount,
				ExitReason:  reason,
			}

			// 处理失败情况
			if status == 30 {
				return result, zeroValue, errors.HandleGenerationFailure(status, failCode, historyID, p.itemType)
			}

			// 处理超时情况
			if p.pollCount >= p.maxPollCount || elapsed > float64(p.timeoutSeconds) {
				err := errors.HandlePollingTimeout(p.pollCount, p.maxPollCount, elapsed, status, itemCount, historyID)
				if err != nil {
					return result, zeroValue, err
				}
			}

			return result, data, nil
		}

		// 计算下次轮询间隔
		interval := p.GetSmartInterval(status, itemCount)
		logger.Debug(fmt.Sprintf("等待 %.1f 秒后继续轮询...", interval.Seconds()))
		time.Sleep(interval)
	}
}

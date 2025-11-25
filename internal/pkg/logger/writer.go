package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogWriter 日志写入器
type LogWriter struct {
	buffers       [][]byte
	logDirPath    string
	writeInterval time.Duration
	mu            sync.Mutex
	stopChan      chan struct{}
}

// NewLogWriter 创建日志写入器
func NewLogWriter(logDirPath string, writeInterval int) *LogWriter {
	// 确保日志目录存在
	if err := os.MkdirAll(logDirPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建日志目录失败: %v\n", err)
	}

	writer := &LogWriter{
		buffers:       make([][]byte, 0),
		logDirPath:    logDirPath,
		writeInterval: time.Duration(writeInterval) * time.Millisecond,
		stopChan:      make(chan struct{}),
	}

	// 启动后台写入
	go writer.work()

	return writer
}

// Push 添加日志到缓冲区
func (w *LogWriter) Push(content []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffers = append(w.buffers, content)
}

// WriteSync 同步写入日志
func (w *LogWriter) WriteSync(buffer []byte) {
	logFile := w.getLogFilePath()
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开日志文件失败: %v\n", err)
		return
	}
	defer f.Close()

	if _, err := f.Write(buffer); err != nil {
		fmt.Fprintf(os.Stderr, "写入日志文件失败: %v\n", err)
	}
}

// Write 异步写入日志
func (w *LogWriter) Write(buffer []byte) {
	logFile := w.getLogFilePath()
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开日志文件失败: %v\n", err)
		return
	}
	defer f.Close()

	if _, err := f.Write(buffer); err != nil {
		fmt.Fprintf(os.Stderr, "写入日志文件失败: %v\n", err)
	}
}

// Flush 刷新缓冲区
func (w *LogWriter) Flush() {
	w.mu.Lock()
	if len(w.buffers) == 0 {
		w.mu.Unlock()
		return
	}

	// 合并所有缓冲区
	totalSize := 0
	for _, buf := range w.buffers {
		totalSize += len(buf)
	}

	combined := make([]byte, 0, totalSize)
	for _, buf := range w.buffers {
		combined = append(combined, buf...)
	}

	// 清空缓冲区
	w.buffers = w.buffers[:0]
	w.mu.Unlock()

	// 写入文件
	logFile := w.getLogFilePath()
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开日志文件失败: %v\n", err)
		return
	}
	defer f.Close()

	if _, err := f.Write(combined); err != nil {
		fmt.Fprintf(os.Stderr, "写入日志文件失败: %v\n", err)
	}
}

// work 后台工作循环
func (w *LogWriter) work() {
	ticker := time.NewTicker(w.writeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.Flush()
		case <-w.stopChan:
			w.Flush()
			return
		}
	}
}

// Stop 停止日志写入器
func (w *LogWriter) Stop() {
	close(w.stopChan)
}

// getLogFilePath 获取日志文件路径
func (w *LogWriter) getLogFilePath() string {
	dateStr := time.Now().Format("2006-01-02")
	return filepath.Join(w.logDirPath, fmt.Sprintf("%s.log", dateStr))
}

package logger

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

// LogLevel 日志级别
type LogLevel int

const (
	// LevelDebug 调试级别
	LevelDebug LogLevel = iota
	// LevelInfo 信息级别
	LevelInfo
	// LevelWarning 警告级别
	LevelWarning
	// LevelError 错误级别
	LevelError
	// LevelSuccess 成功级别
	LevelSuccess
)

var (
	logLevelNames = map[LogLevel]string{
		LevelDebug:   "debug",
		LevelInfo:    "info",
		LevelWarning: "warning",
		LevelError:   "error",
		LevelSuccess: "success",
	}

	logLevelPriority = map[string]int{
		"error":   1,
		"warning": 2,
		"success": 3,
		"info":    4,
		"debug":   5,
	}

	// 彩色输出函数
	colorFuncs = map[LogLevel]func(...interface{}) string{
		LevelSuccess: color.New(color.FgGreen).SprintFunc(),
		LevelInfo:    color.New(color.FgCyan, color.Bold).SprintFunc(),
		LevelDebug:   color.New(color.FgWhite).SprintFunc(),
		LevelWarning: color.New(color.FgYellow, color.Bold).SprintFunc(),
		LevelError:   color.New(color.FgRed, color.Bold).SprintFunc(),
	}
)

// Logger 日志记录器
type Logger struct {
	writer       *LogWriter
	logLevel     string
	debug        bool
	logDirPath   string
	writeInterval int
	mu           sync.Mutex
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Init 初始化日志系统
func Init(logDirPath string, logLevel string, debug bool, writeInterval int) {
	once.Do(func() {
		defaultLogger = &Logger{
			writer:        NewLogWriter(logDirPath, writeInterval),
			logLevel:      logLevel,
			debug:         debug,
			logDirPath:    logDirPath,
			writeInterval: writeInterval,
		}
	})
}

// Header 输出日志头部
func Header() {
	if defaultLogger != nil {
		defaultLogger.Header()
	}
}

// Footer 输出日志尾部
func Footer() {
	if defaultLogger != nil {
		defaultLogger.Footer()
	}
}

// Success 成功日志
func Success(args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(LevelSuccess, args...)
	}
}

// Info 信息日志
func Info(args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(LevelInfo, args...)
	}
}

// Debug 调试日志
func Debug(args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(LevelDebug, args...)
	}
}

// Warn 警告日志
func Warn(args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(LevelWarning, args...)
	}
}

// Error 错误日志
func Error(args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(LevelError, args...)
	}
}

// Destroy 销毁日志系统
func Destroy() {
	if defaultLogger != nil {
		defaultLogger.writer.Flush()
	}
}

// Header 写入日志头部
func (l *Logger) Header() {
	header := fmt.Sprintf("\n\n===================== LOG START %s =====================\n\n",
		time.Now().Format("2006-01-02 15:04:05.000"))
	l.writer.WriteSync([]byte(header))
}

// Footer 写入日志尾部
func (l *Logger) Footer() {
	l.writer.Flush()
	footer := fmt.Sprintf("\n\n===================== LOG END %s =====================\n\n",
		time.Now().Format("2006-01-02 15:04:05.000"))
	l.writer.WriteSync([]byte(footer))
}

func (l *Logger) checkLevel(level LogLevel) bool {
	levelName := logLevelNames[level]
	currentPriority := logLevelPriority[l.logLevel]
	if currentPriority == 0 {
		currentPriority = 99
	}
	levelPriority := logLevelPriority[levelName]
	return levelPriority <= currentPriority
}

func (l *Logger) log(level LogLevel, args ...interface{}) {
	// 调试模式检查
	if level == LevelDebug && !l.debug {
		return
	}

	// 日志级别检查
	if !l.checkLevel(level) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 获取调用栈信息
	_, file, line, ok := runtime.Caller(2)
	var source string
	if ok {
		filename := filepath.Base(file)
		filename = strings.TrimSuffix(filename, filepath.Ext(filename))
		source = fmt.Sprintf("%s<%d,0>", filename, line)
	} else {
		source = "unknown<0,0>"
	}

	// 格式化消息
	message := fmt.Sprint(args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	levelName := logLevelNames[level]

	// 构建日志文本
	logText := fmt.Sprintf("[%s][%s][%s] %s\n", timestamp, levelName, source, message)

	// 彩色控制台输出
	colorFunc := colorFuncs[level]
	if colorFunc != nil {
		fmt.Print(colorFunc(logText))
	} else {
		fmt.Print(logText)
	}

	// 写入文件
	l.writer.Push([]byte(logText))
}

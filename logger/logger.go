package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func ParseLogLevel(level string) LogLevel {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

type Logger struct {
	level  LogLevel
	logger *log.Logger
	mutex  sync.Mutex
	output io.Writer
}

func NewLogger(level LogLevel, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}

	return &Logger{
		level:  level,
		logger: log.New(output, "", 0),
		mutex:  sync.Mutex{},
		output: output,
	}
}

func NewFileLogger(level LogLevel, filename string) (*Logger, error) {
	if filename == "" {
		return nil, fmt.Errorf("filename cannot be empty")
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Logger{
		level:  level,
		logger: log.New(file, "", 0),
		mutex:  sync.Mutex{},
		output: file,
	}, nil
}

func (l *Logger) SetLevel(level LogLevel) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.level = level
}

func (l *Logger) GetLevel() LogLevel {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.level
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	if l == nil {
		return
	}
	l.logSafe(DEBUG, msg, args...)
}

func (l *Logger) Info(msg string, args ...interface{}) {
	if l == nil {
		return
	}
	l.logSafe(INFO, msg, args...)
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	if l == nil {
		return
	}
	l.logSafe(WARN, msg, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	if l == nil {
		return
	}
	l.logSafe(ERROR, msg, args...)
}

func (l *Logger) logSafe(level LogLevel, msg string, args ...interface{}) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.level <= level {
		l.logUnsafe(level, msg, args...)
	}
}

func (l *Logger) logUnsafe(level LogLevel, msg string, args ...interface{}) {
	if l.logger == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	var formattedMsg string
	if len(args) > 0 {
		formattedMsg = fmt.Sprintf(msg, args...)
	} else {
		formattedMsg = msg
	}

	logLine := fmt.Sprintf("[%s] %s: %s", timestamp, level.String(), formattedMsg)
	l.logger.Println(logLine)
}

func (l *Logger) Close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if file, ok := l.output.(*os.File); ok && file != os.Stdout && file != os.Stderr {
		return file.Close()
	}
	return nil
}

func (l *Logger) WithFields(fields map[string]interface{}) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

type FieldLogger struct {
	logger *Logger
	fields map[string]interface{}
}

func (fl *FieldLogger) Debug(msg string, args ...interface{}) {
	fl.logWithFields(DEBUG, msg, args...)
}

func (fl *FieldLogger) Info(msg string, args ...interface{}) {
	fl.logWithFields(INFO, msg, args...)
}

func (fl *FieldLogger) Warn(msg string, args ...interface{}) {
	fl.logWithFields(WARN, msg, args...)
}

func (fl *FieldLogger) Error(msg string, args ...interface{}) {
	fl.logWithFields(ERROR, msg, args...)
}

func (fl *FieldLogger) logWithFields(level LogLevel, msg string, args ...interface{}) {
	var formattedMsg string
	if len(args) > 0 {
		formattedMsg = fmt.Sprintf(msg, args...)
	} else {
		formattedMsg = msg
	}

	if len(fl.fields) > 0 {
		var fieldParts []string
		for key, value := range fl.fields {
			fieldParts = append(fieldParts, fmt.Sprintf("%s=%v", key, value))
		}
		formattedMsg = fmt.Sprintf("%s [%s]", formattedMsg, strings.Join(fieldParts, " "))
	}

	fl.logger.logSafe(level, "%s", formattedMsg)
}

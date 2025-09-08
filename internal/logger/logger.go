package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
)

var (
	// Global logger instance
	Log *logrus.Logger

	// Context key for request ID
	RequestIDKey = "request_id"
)

// Config holds logger configuration
type Config struct {
	Level      string
	Format     string // "json" or "text"
	Output     string // "stdout", "stderr", or file path
	TimeFormat string
}

// Initialize sets up the global logger
func Initialize(cfg Config) error {
	Log = logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Log.SetLevel(level)

	// Set formatter
	if cfg.Format == "json" {
		Log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: cfg.TimeFormat,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "time",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyFunc:  "function",
			},
		})
	} else {
		Log.SetFormatter(&CustomTextFormatter{
			logrus.TextFormatter{
				TimestampFormat: cfg.TimeFormat,
				FullTimestamp:   true,
				ForceColors:     true,
				DisableColors:   false,
			},
		})
	}

	// Set output
	switch cfg.Output {
	case "stdout":
		Log.SetOutput(os.Stdout)
	case "stderr":
		Log.SetOutput(os.Stderr)
	default:
		// Assume it's a file path
		file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		Log.SetOutput(file)
	}

	// Add hook for caller information
	Log.AddHook(&CallerHook{})

	return nil
}

// CustomTextFormatter is a custom formatter for better readability
type CustomTextFormatter struct {
	logrus.TextFormatter
}

func (f *CustomTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Custom format: [TIME] [LEVEL] [REQUEST_ID] MESSAGE {FIELDS}
	timestamp := entry.Time.Format(f.TimestampFormat)
	level := strings.ToUpper(entry.Level.String())

	// Build the log message
	var b strings.Builder

	// Color codes
	levelColor := f.getLevelColor(entry.Level)
	resetColor := "\033[0m"

	if f.ForceColors && !f.DisableColors {
		b.WriteString(fmt.Sprintf("\033[36m[%s]\033[0m %s[%-5s]%s ", timestamp, levelColor, level, resetColor))
	} else {
		b.WriteString(fmt.Sprintf("[%s] [%-5s] ", timestamp, level))
	}

	// Add request ID if present
	if requestID, ok := entry.Data[RequestIDKey]; ok {
		b.WriteString(fmt.Sprintf("[%v] ", requestID))
		delete(entry.Data, RequestIDKey)
	}

	// Add caller info if present
	if caller, ok := entry.Data["caller"]; ok {
		b.WriteString(fmt.Sprintf("[%v] ", caller))
		delete(entry.Data, "caller")
	}

	// Add message
	b.WriteString(entry.Message)

	// Add remaining fields
	if len(entry.Data) > 0 {
		b.WriteString(" ")
		b.WriteString(fmt.Sprintf("%+v", entry.Data))
	}

	b.WriteString("\n")

	return []byte(b.String()), nil
}

func (f *CustomTextFormatter) getLevelColor(level logrus.Level) string {
	switch level {
	case logrus.DebugLevel:
		return "\033[37m" // White
	case logrus.InfoLevel:
		return "\033[34m" // Blue
	case logrus.WarnLevel:
		return "\033[33m" // Yellow
	case logrus.ErrorLevel:
		return "\033[31m" // Red
	case logrus.FatalLevel, logrus.PanicLevel:
		return "\033[35m" // Magenta
	default:
		return "\033[0m" // Reset
	}
}

// CallerHook adds caller information to log entries
type CallerHook struct{}

func (hook *CallerHook) Fire(entry *logrus.Entry) error {
	pc := make([]uintptr, 3)
	cnt := runtime.Callers(6, pc)

	for i := 0; i < cnt; i++ {
		fu := runtime.FuncForPC(pc[i] - 1)
		name := fu.Name()
		if !isLogrusPackage(name) {
			file, line := fu.FileLine(pc[i] - 1)
			entry.Data["caller"] = fmt.Sprintf("%s:%d", filepath.Base(file), line)
			break
		}
	}

	return nil
}

func (hook *CallerHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func isLogrusPackage(name string) bool {
	return strings.Contains(name, "sirupsen/logrus")
}

// WithContext creates an entry with context
func WithContext(ctx context.Context) *logrus.Entry {
	entry := Log.WithContext(ctx)

	// Add request ID from context if present
	if requestID := ctx.Value(RequestIDKey); requestID != nil {
		entry = entry.WithField(RequestIDKey, requestID)
	}

	return entry
}

// WithField creates an entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return Log.WithField(key, value)
}

// WithFields creates an entry with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Log.WithFields(fields)
}

// WithError creates an entry with an error field
func WithError(err error) *logrus.Entry {
	return Log.WithError(err)
}

// Helper functions for direct logging
func Debug(args ...interface{}) {
	Log.Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	Log.Debugf(format, args...)
}

func Info(args ...interface{}) {
	Log.Info(args...)
}

func Infof(format string, args ...interface{}) {
	Log.Infof(format, args...)
}

func Warn(args ...interface{}) {
	Log.Warn(args...)
}

func Warnf(format string, args ...interface{}) {
	Log.Warnf(format, args...)
}

func Error(args ...interface{}) {
	Log.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	Log.Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	Log.Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	Log.Fatalf(format, args...)
}

// GormLogger implements GORM's logger interface
type GormLogger struct {
	SlowThreshold         time.Duration
	SourceField           string
	SkipErrRecordNotFound bool
	LogLevel              logger.LogLevel
}

// NewGormLogger creates a new GORM logger
func NewGormLogger() logger.Interface {
	return &GormLogger{
		SlowThreshold:         200 * time.Millisecond,
		LogLevel:              logger.Info,
		SkipErrRecordNotFound: true,
		SourceField:           "source",
	}
}

func (l *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		WithContext(ctx).WithField("gorm", true).Infof(msg, data...)
	}
}

func (l GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		WithContext(ctx).WithField("gorm", true).Warnf(msg, data...)
	}
}

func (l GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		WithContext(ctx).WithField("gorm", true).Errorf(msg, data...)
	}
}

func (l GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	entry := WithContext(ctx).WithFields(logrus.Fields{
		"gorm":     true,
		"duration": elapsed.Milliseconds(),
		"rows":     rows,
		"sql":      sql,
	})

	switch {
	case err != nil && (!l.SkipErrRecordNotFound || !strings.Contains(err.Error(), "record not found")):
		entry.WithError(err).Error("Database error")
	case l.SlowThreshold != 0 && elapsed > l.SlowThreshold:
		entry.Warnf("Slow SQL query [%v]", elapsed)
	case l.LogLevel >= logger.Info:
		entry.Debugf("SQL query executed [%v]", elapsed)
	}
}

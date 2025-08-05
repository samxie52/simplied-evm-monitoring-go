package logger

import (
	"io"
	"os"
	"path/filepath"
	"simplied-evm-monitoring-go/internal/config"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger *logrus.Logger

func InitLogger(config config.LoggingConfig) error {
	Logger = logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		return err
	}
	Logger.SetLevel(level)

	// 设置日志格式
	if config.Format == "json" {
		Logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// 设置输出
	switch config.Output {
	case "file":
		Logger.SetOutput(getFileWriter(config))
	case "both":
		multiWriter := io.MultiWriter(os.Stdout, getFileWriter(config))
		Logger.SetOutput(multiWriter)
	default: // console
		Logger.SetOutput(os.Stdout)
	}
	return nil
}

func getFileWriter(config config.LoggingConfig) io.Writer {
	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logrus.Errorf("Failed to create log directory: %v", err)
		return os.Stdout
	}

	return &lumberjack.Logger{
		Filename:   config.FilePath,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   true,
	}
}

func Debug(args ...interface{}) {
	if Logger != nil {
		Logger.Debug(args...)
	}
}

func Info(args ...interface{}) {
	if Logger != nil {
		Logger.Info(args...)
	}
}

func Warn(args ...interface{}) {
	if Logger != nil {
		Logger.Warn(args...)
	}
}

func Error(args ...interface{}) {
	if Logger != nil {
		Logger.Error(args...)
	}
}

func Fatal(args ...interface{}) {
	if Logger != nil {
		Logger.Fatal(args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if Logger != nil {
		Logger.Debugf(format, args...)
	}
}

func Infof(format string, args ...interface{}) {
	if Logger != nil {
		Logger.Infof(format, args...)
	}
}

func Warnf(format string, args ...interface{}) {
	if Logger != nil {
		Logger.Warnf(format, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if Logger != nil {
		Logger.Errorf(format, args...)
	}
}

func Fatalf(format string, args ...interface{}) {
	if Logger != nil {
		Logger.Fatalf(format, args...)
	}
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	if Logger != nil {
		return Logger.WithFields(fields)
	}
	return logrus.NewEntry(logrus.New())
}

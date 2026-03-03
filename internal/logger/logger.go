package logger

import (
	"io"
	"os"
	"path/filepath"

	"pkuphysu-backend/internal/config"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

func Init() {
	formatter := &logrus.TextFormatter{
		ForceColors:               true,
		EnvironmentOverrideColors: true,
		TimestampFormat:           "2006-01-02 15:04:05",
		FullTimestamp:             true,
	}
	logrus.SetFormatter(formatter)

	if config.Conf.LogConfig.Level == "" {
		logrus.SetLevel(logrus.InfoLevel)
	} else {
		level, err := logrus.ParseLevel(config.Conf.LogConfig.Level)
		if err != nil {
			logrus.SetLevel(logrus.InfoLevel)
		} else {
			logrus.SetLevel(level)
		}
	}

	logConfig := config.Conf.LogConfig
	if logConfig.Enable && logConfig.FilePath != "" {
		dir := filepath.Dir(logConfig.FilePath)
		os.MkdirAll(dir, 0755)

		var w io.Writer = &lumberjack.Logger{
			Filename:   logConfig.FilePath,
			MaxSize:    logConfig.MaxSize,
			MaxBackups: logConfig.MaxBackups,
			MaxAge:     logConfig.MaxAge,
			Compress:   logConfig.Compress,
		}
		w = io.MultiWriter(os.Stdout, w)
		logrus.SetOutput(w)
	} else {
		logrus.SetOutput(os.Stdout)
	}
}

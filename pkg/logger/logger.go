package logger

import (
	"sync"

	"github.com/Roll-Play/togglelabs/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger
var lock = &sync.Mutex{}

func NewZapLogger() (*zap.Logger, error) {
	var level zapcore.Level
	if config.Environment == config.DevEnvironment || config.Environment == "" {
		level = zap.DebugLevel
	} else {
		level = zap.InfoLevel
	}

	config := zap.Config{
		Encoding:         "json",
		Level:            zap.NewAtomicLevelAt(level),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey: "message",

			LevelKey:    "level",
			EncodeLevel: zapcore.CapitalLevelEncoder,

			TimeKey:    "time",
			EncodeTime: zapcore.ISO8601TimeEncoder,

			CallerKey:    "caller",
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}
	logger, err := config.Build()

	if err != nil {
		return nil, err
	}

	return logger, nil
}

func GetInstance() (*zap.Logger, error) {
	if logger != nil {
		return logger, nil
	}

	lock.Lock()
	defer lock.Unlock()
	newLogger, err := NewZapLogger()
	if err != nil {
		return nil, err
	}

	logger = newLogger
	return logger, nil
}

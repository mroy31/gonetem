package logger

import (
	"fmt"
	"io"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

var (
	logger log.Logger
	isInit = false
)

func InitLogger(logWriter io.Writer, logOptions ...level.Option) {
	if !isInit {
		logger = log.NewLogfmtLogger(logWriter)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = level.NewFilter(logger, logOptions...)
		isInit = true
	}
}

func AddFilters(logOptions ...level.Option) error {
	if !isInit {
		return fmt.Errorf("Logger module is not initialized")
	}
	logger = level.NewFilter(logger, logOptions...)

	return nil
}

func DefaultLogger() {
	InitLogger(os.Stderr, level.AllowError(), level.AllowInfo(), level.AllowWarn())
}

func Debug(keyvals ...interface{}) {
	if !isInit {
		DefaultLogger()
	}
	level.Debug(logger).Log(keyvals...)
}

func Info(keyvals ...interface{}) {
	if !isInit {
		DefaultLogger()
	}
	level.Info(logger).Log(keyvals...)
}

func Warn(keyvals ...interface{}) {
	if !isInit {
		DefaultLogger()
	}
	level.Warn(logger).Log(keyvals...)
}

func Error(keyvals ...interface{}) {
	if !isInit {
		DefaultLogger()
	}
	level.Error(logger).Log(keyvals...)
}

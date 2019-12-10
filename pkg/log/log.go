package log

import (
	"context"
	"io"
	"os"

	"github.com/Sirupsen/logrus"
)

// consts ..
const (
	ProviderName = "provider"
)

type loggerKeyType int

const loggerKey loggerKeyType = iota

// Logger ..
type Logger interface {
	logrus.FieldLogger
	WriterLevel(logrus.Level) *io.PipeWriter
}

var (
	mainLogger  Logger
	logFilePath string
	logFile     *os.File
)

func init() {
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05.000"
	logrus.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
	mainLogger = logrus.WithFields(logrus.Fields{})
}

// Str ..
func Str(key, value string) func(logrus.Fields) {
	return func(fields logrus.Fields) {
		fields[key] = value
	}
}

// NewContext ..
func NewContext(ctx context.Context, opts ...func(logrus.Fields)) context.Context {
	fields := make(logrus.Fields)
	for _, opt := range opts {
		opt(fields)
	}
	return context.WithValue(ctx, loggerKey, WithContext(ctx).WithFields(fields))
}

// WithContext ..
func WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return mainLogger
	}
	if ctxLogger, ok := ctx.Value(loggerKey).(Logger); ok {
		return ctxLogger
	}
	return mainLogger
}

package logr

import (
	"fmt"
	"io"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	zc "go.uber.org/zap/zapcore"
)

const (
	DebugLevel = 1
)

type Logger struct {
	logr.Logger
}

func (l Logger) Debug(msg string, keyAndValues ...any) {
	l.V(DebugLevel).Info(msg, keyAndValues...)
}

// New returns a new Logger instance with specified logLevel and devMode.
//
// The writer can be used e.g. to save the logs in a file.
func New(logLevel string, isDev bool, writer io.Writer) (*Logger, error) {
	level, err := parseLogLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("error parsing logLevel: %w", err)
	}

	logger, err := getLoggerImplementation(writer, level, isDev)
	if err != nil {
		return nil, err
	}

	return &Logger{Logger: *logger}, nil
}

func getLoggerImplementation(writer io.Writer, level *zap.AtomicLevel, isDev bool) (*logr.Logger, error) {
	config := zap.NewDevelopmentConfig()

	config.Level = *level
	config.Development = isDev
	config.DisableStacktrace = !isDev
	config.DisableCaller = !isDev

	opts := getLoggerOptions(writer, level, config)

	zapLogger, err := config.Build(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	logger := zapr.NewLogger(zapLogger)

	return &logger, nil
}

func getLoggerOptions(writer io.Writer, level *zap.AtomicLevel, config zap.Config) []zap.Option {
	var opts = []zap.Option{
		zap.WithCaller(true),
	}

	// adds stacktrace to lof entry
	if level.Level() == zap.DebugLevel {
		withStacktrace := zap.AddStacktrace(zc.DebugLevel)
		opts = append(opts, withStacktrace)
	}

	// redirects log output to passed io.Writer
	if writer != nil {
		syncer := zc.AddSync(writer)
		encoder := zc.NewJSONEncoder(config.EncoderConfig)

		opts = append(opts, zap.WrapCore(func(core zc.Core) zc.Core {
			return zc.NewCore(encoder, syncer, *level)
		}))
	}

	return opts
}

func parseLogLevel(logLevel string) (*zap.AtomicLevel, error) {
	level, err := zc.ParseLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("error parsing to zap logLevel: %w", err)
	}

	var al zap.AtomicLevel
	if err := al.UnmarshalText([]byte(level.String())); err != nil {
		return nil, fmt.Errorf("error unmarshal logLevel to atomic: %w", err)
	}

	return &al, nil
}

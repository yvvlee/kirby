package infra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"

	"github.com/yvvlee/kirby/server/internal/conf"
)

// Logger adapts Zap to Kratos.
type Logger struct {
	zapLogger *zap.Logger
	stderr    bool
	*kratoslog.Helper
}

var _ kratoslog.Logger = (*Logger)(nil)

// NewLogger uses one structured stderr stream for the application.
//
// Parameters: cfg supplies logging settings.
// Returns: the logger and its cleanup function.
// Throws: an error when the log destination cannot be initialized.
func NewLogger(cfg *conf.Config) (*Logger, func(), error) {
	logger, err := NewLoggerWithWriter(os.Stderr, cfg.Log, zap.AddCaller(), zap.AddCallerSkip(3))
	if err != nil {
		return nil, nil, err
	}
	return logger, func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close logger: %v\n", err)
		}
	}, nil
}

// NewLoggerWithWriter permits controlled destinations and caller attribution.
//
// Parameters: writer is the destination; cfg supplies logging settings; options customize Zap.
// Returns: the structured logger.
// Throws: an error for a missing destination.
func NewLoggerWithWriter(writer io.Writer, cfg conf.LogConfig, options ...zap.Option) (*Logger, error) {
	if writer == nil {
		return nil, fmt.Errorf("log writer is nil")
	}
	level := parseLevel(cfg.Level)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.LevelKey = "_LEVEL_"
	encoderConfig.TimeKey = "_TS_"
	encoderConfig.MessageKey = "_MSG_"
	encoderConfig.CallerKey = "_CALLER_"
	encoderConfig.NameKey = "_NAME_"
	encoderConfig.StacktraceKey = "_STACKTRACE_"
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.DateTime)
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	logger := &Logger{
		zapLogger: zap.New(zapcore.NewCore(encoder, zapcore.Lock(zapcore.AddSync(writer)), level), options...),
	}
	logger.stderr = writer == os.Stderr
	logger.Helper = kratoslog.NewHelper(kratoslog.With(logger))
	return logger, nil
}

// WithContext attaches request metadata to subsequent log events.
//
// Parameters: ctx carries the request context.
// Returns: the context-bound Kratos logging helper.
// Throws: none.
func (l *Logger) WithContext(ctx context.Context) *kratoslog.Helper { return l.Helper.WithContext(ctx) }

// Close flushes buffered events while accepting unsupported stderr synchronization.
//
// Parameters: none.
// Returns: nil after flushing.
// Throws: an output synchronization error other than unsupported stderr sync.
func (l *Logger) Close() error {
	err := l.zapLogger.Sync()
	if l.stderr && (errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTTY)) {
		return nil
	}
	return err
}

func parseLevel(value string) zapcore.Level {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "DEBUG":
		return zap.DebugLevel
	case "WARNING":
		return zap.WarnLevel
	case "ERROR":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

// Log translates paired Kratos fields without losing protobuf structure.
//
// Parameters: level is the event severity; keyValues contains alternating field names and values.
// Returns: nil after processing the event.
// Throws: an error for unpaired fields or unsupported severity.
func (l *Logger) Log(level kratoslog.Level, keyValues ...any) error {
	if len(keyValues) == 0 || len(keyValues)%2 != 0 {
		return fmt.Errorf("log key values must appear in pairs")
	}
	var zapLevel zapcore.Level
	switch level {
	case kratoslog.LevelDebug:
		zapLevel = zap.DebugLevel
	case kratoslog.LevelInfo:
		zapLevel = zap.InfoLevel
	case kratoslog.LevelWarn:
		zapLevel = zap.WarnLevel
	case kratoslog.LevelError:
		zapLevel = zap.ErrorLevel
	case kratoslog.LevelFatal:
		zapLevel = zap.FatalLevel
	default:
		return fmt.Errorf("unsupported log level %v", level)
	}
	fields := make([]zap.Field, 0, len(keyValues)/2)
	message := ""
	for len(keyValues) >= 2 {
		key := fmt.Sprint(keyValues[0])
		value := keyValues[1]
		keyValues = keyValues[2:]
		if key == kratoslog.DefaultMessageKey {
			message = fmt.Sprint(value)
			continue
		}
		if message, ok := value.(proto.Message); ok && message != nil {
			fields = append(fields, zap.Reflect(key, value))
			continue
		}
		fields = append(fields, zap.Any(key, value))
	}
	entry := l.zapLogger.Check(zapLevel, message)
	if entry == nil {
		return nil
	}
	entry.Write(fields...)
	return nil
}

package infra

import (
	"context"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	"github.com/yvvlee/lorm"
)

var _ lorm.Logger = (*DBLogger)(nil)

// DBLogger routes lorm events through the application's structured logger.
type DBLogger struct {
	*Logger
}

// NewDBLogger shares the process logger so database events use the same output.
//
// Parameters: logger is the initialized process logger.
// Returns: the lorm logging adapter.
// Throws: none.
func NewDBLogger(logger *Logger) *DBLogger {
	return &DBLogger{
		Logger: logger,
	}
}

// DebugContext records a database debug event with request context.
//
// Parameters: ctx carries request metadata; msg describes the event; args are field pairs.
// Returns: none.
// Throws: none.
func (l *DBLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, kratoslog.LevelDebug, msg, args)
}

// InfoContext records a database informational event with request context.
//
// Parameters: ctx carries request metadata; msg describes the event; args are field pairs.
// Returns: none.
// Throws: none.
func (l *DBLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, kratoslog.LevelInfo, msg, args)
}

// WarnContext records a database warning with request context.
//
// Parameters: ctx carries request metadata; msg describes the event; args are field pairs.
// Returns: none.
// Throws: none.
func (l *DBLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, kratoslog.LevelWarn, msg, args)
}

// ErrorContext records a database error with request context.
//
// Parameters: ctx carries request metadata; msg describes the event; args are field pairs.
// Returns: none.
// Throws: none.
func (l *DBLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, kratoslog.LevelError, msg, args)
}

func (l *DBLogger) log(ctx context.Context, level kratoslog.Level, msg string, args []any) {
	fields := make([]any, 0, len(args)+2)
	fields = append(fields, "msg", msg)
	fields = append(fields, args...)
	l.WithContext(ctx).Log(level, fields...)
}

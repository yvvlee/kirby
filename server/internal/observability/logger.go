// Package observability provides structured logging with mandatory redaction.
package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"unicode"

	"github.com/yvvlee/kirby/server/internal/config"
)

const redacted = "[REDACTED]"

var (
	jsonSecretPattern = regexp.MustCompile(`(?i)("(?:password|passwd|authorization|cookie|access_token|refresh_token|jwt|api_key_pepper|api_key|access_key|secret_key|client_secret|signing_key|dsn|secret|signature)"\s*:\s*")[^"]*(")`)
	keyValuePattern   = regexp.MustCompile(`(?i)(\b(?:password|passwd|authorization|cookie|access_token|refresh_token|jwt|api_key_pepper|api_key|access_key|secret_key|client_secret|signing_key|dsn|secret|signature)\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\s,&]+)`)
	bearerPattern     = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	jwtPattern        = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
	signedURLPattern  = regexp.MustCompile(`(?i)(X-Amz-(?:Signature|Credential|Security-Token)=)[^&\s]+`)
)

var sensitiveKeys = map[string]struct{}{
	"password": {}, "passwd": {}, "pwd": {},
	"authorization": {}, "proxyauthorization": {},
	"cookie": {}, "setcookie": {},
	"token": {}, "accesstoken": {}, "refreshtoken": {}, "jwt": {},
	"apikey": {}, "apikeypepper": {}, "secret": {}, "clientsecret": {}, "signingkey": {},
	"signature": {}, "dsn": {}, "accesskey": {}, "secretkey": {},
	"body": {}, "requestbody": {}, "responsebody": {}, "payload": {}, "rawrequest": {},
}

// NewLogger constructs a JSON or text slog logger with redaction applied first.
func NewLogger(writer io.Writer, cfg config.LogConfig) (*slog.Logger, error) {
	if writer == nil {
		return nil, fmt.Errorf("log writer is nil")
	}
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(writer, options)
	case "text":
		handler = slog.NewTextHandler(writer, options)
	default:
		return nil, fmt.Errorf("unsupported log format %q", cfg.Format)
	}
	return slog.New(&redactingHandler{next: handler}), nil
}

func parseLevel(value string) (slog.Level, error) {
	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", value)
	}
}

type redactingHandler struct {
	next slog.Handler
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, sanitizeText(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(sanitizeAttr(attr))
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		clean = append(clean, sanitizeAttr(attr))
	}
	return &redactingHandler{next: h.next.WithAttrs(clean)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name)}
}

func sanitizeAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if isSensitiveKey(attr.Key) {
		return slog.String(attr.Key, redacted)
	}
	if attr.Value.Kind() == slog.KindGroup {
		children := attr.Value.Group()
		clean := make([]slog.Attr, 0, len(children))
		for _, child := range children {
			clean = append(clean, sanitizeAttr(child))
		}
		return slog.Group(attr.Key, attrsToAny(clean)...)
	}
	if attr.Value.Kind() == slog.KindString {
		return slog.String(attr.Key, sanitizeText(attr.Value.String()))
	}
	if attr.Value.Kind() == slog.KindAny {
		if err, ok := attr.Value.Any().(error); ok {
			return slog.String(attr.Key, sanitizeText(err.Error()))
		}
	}
	return attr
}

func attrsToAny(attrs []slog.Attr) []any {
	values := make([]any, len(attrs))
	for i := range attrs {
		values[i] = attrs[i]
	}
	return values
}

func isSensitiveKey(key string) bool {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, key)
	_, ok := sensitiveKeys[normalized]
	return ok
}

// sanitizeText removes common credential forms from otherwise useful messages.
func sanitizeText(value string) string {
	value = jsonSecretPattern.ReplaceAllString(value, `${1}`+redacted+`${2}`)
	value = keyValuePattern.ReplaceAllString(value, `${1}`+redacted)
	value = bearerPattern.ReplaceAllString(value, "Bearer "+redacted)
	value = jwtPattern.ReplaceAllString(value, redacted)
	value = signedURLPattern.ReplaceAllString(value, `${1}`+redacted)
	return value
}

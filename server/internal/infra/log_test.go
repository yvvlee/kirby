package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/yvvlee/kirby/server/internal/conf"
)

func TestParseLevelMatchesLegacyLogger(t *testing.T) {
	for _, test := range []struct {
		value string
		want  zapcore.Level
	}{
		{"DEBUG", zap.DebugLevel},
		{" debug ", zap.DebugLevel},
		{"WARNING", zap.WarnLevel},
		{" Warning ", zap.WarnLevel},
		{"ERROR", zap.ErrorLevel},
		{"error", zap.ErrorLevel},
		{"INFO", zap.InfoLevel},
		{"", zap.InfoLevel},
		{"unknown", zap.InfoLevel},
		{"warn", zap.InfoLevel},
	} {
		t.Run(test.value, func(t *testing.T) {
			assert.Equal(t, test.want, parseLevel(test.value))
		})
	}
}

func TestLoggerPassesMessagesAndFieldsThrough(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewLoggerWithWriter(&output, conf.LogConfig{Level: "INFO"})
	require.NoError(t, err)
	logger.Infow("msg", "password=message-value", "password", "field-value",
		"details", map[string]any{"access_token": "nested-value"},
		"error", errors.New("dsn=database-value"), "config", conf.NewSecret("configured-value"))
	var record map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &record))
	assert.Equal(t, "password=message-value", record["_MSG_"])
	assert.Equal(t, "field-value", record["password"])
	assert.Equal(t, map[string]any{"access_token": "nested-value"}, record["details"])
	assert.Equal(t, "dsn=database-value", record["error"])
	assert.Equal(t, "configured-value", record["config"])
}

func TestLoggerRejectsMalformedFieldsAndOutputErrors(t *testing.T) {
	logger, err := NewLoggerWithWriter(&bytes.Buffer{}, conf.LogConfig{Level: "info"})
	require.NoError(t, err)
	require.Error(t, logger.Log(kratoslog.LevelInfo, "odd"))
	var diagnostics bytes.Buffer
	failing, err := NewLoggerWithWriter(failingLogWriter{}, conf.LogConfig{Level: "info"}, zap.ErrorOutput(zapcore.AddSync(&diagnostics)))
	require.NoError(t, err)
	require.NoError(t, failing.Log(kratoslog.LevelInfo, "msg", "test"))
	assert.Contains(t, diagnostics.String(), "write failed")
}

type failingLogWriter struct{}

func (failingLogWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestLoggerFiltersDisabledLevels(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewLoggerWithWriter(&output, conf.LogConfig{Level: "WARNING"})
	require.NoError(t, err)
	require.NoError(t, logger.Log(kratoslog.LevelInfo, "msg", "hidden"))
	assert.Empty(t, output.String())
	require.NoError(t, logger.Log(kratoslog.LevelError, "msg", "visible"))
	assert.Contains(t, output.String(), "visible")
}

func TestLoggerReportsHelperCallerWithAndWithoutContext(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewLoggerWithWriter(&output, conf.LogConfig{Level: "info"}, zap.AddCaller(), zap.AddCallerSkip(3))
	require.NoError(t, err)
	logger.Info("plain")
	logger.WithContext(context.Background()).Info("context")
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		assert.Contains(t, record["_CALLER_"], "infra/log_test.go:")
	}
	require.Error(t, logger.Log(kratoslog.Level(99), "msg", "invalid"))
}

package logic

import (
	"github.com/stretchr/testify/require"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra"
	"io"
	"testing"
)

func newContentTestLogger(t *testing.T) *infra.Logger {
	t.Helper()
	logger, err := infra.NewLoggerWithWriter(io.Discard, conf.LogConfig{})
	require.NoError(t, err)
	return logger
}

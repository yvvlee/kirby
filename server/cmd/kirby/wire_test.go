package kirby

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/conf"
)

func TestInitializeNilConfigPanics(t *testing.T) {
	require.Panics(t, func() {
		_, _, _ = wireServer(context.Background(), nil, nil)
	})
}

func TestInitializeInvalidConfig(t *testing.T) {
	_, _, err := wireServer(context.Background(), &conf.Config{}, nil)
	require.Error(t, err)
}

package kirby

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitializeMissingConfigSourceFails(t *testing.T) {
	t.Setenv("KIRBY_CONFIG_FILE", "")
	_, _, err := wireServer(context.Background(), "", nil)
	require.EqualError(t, err, "configuration file required: set --config or KIRBY_CONFIG_FILE")
}

func TestInitializeInvalidConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kirby.yaml")
	require.NoError(t, os.WriteFile(path, []byte("mode: bogus\n"), 0o600))
	_, _, err := wireServer(context.Background(), path, nil)
	require.Error(t, err)
}

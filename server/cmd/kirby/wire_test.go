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
	t.Setenv("NACOS_SERVER_HOST", "")
	_, _, err := wireServer(context.Background(), "", nil)
	require.Error(t, err)
}

func TestInitializeInvalidConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kirby.yaml")
	require.NoError(t, os.WriteFile(path, []byte("mode: bogus\n"), 0o600))
	_, _, err := wireServer(context.Background(), path, nil)
	require.Error(t, err)
}

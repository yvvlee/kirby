package kirby

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionFlag(t *testing.T) {
	stdout := new(bytes.Buffer)
	command := newRootCommand(stdout, new(bytes.Buffer))
	command.SetArgs([]string{"--version"})

	require.NoError(t, command.Execute())
	require.True(t, strings.HasPrefix(stdout.String(), "kirby "))
}

func TestServeUsesConfigFlag(t *testing.T) {
	stdout := new(bytes.Buffer)
	command := newRootCommand(stdout, new(bytes.Buffer))
	command.SetArgs([]string{"serve", "--help"})
	require.NoError(t, command.Execute())
	require.Contains(t, stdout.String(), "--config")
	require.NotContains(t, stdout.String(), "--http-address")
}

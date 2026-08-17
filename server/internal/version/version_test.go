package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestString(t *testing.T) {
	require.Equal(t, "dev (commit=unknown, built=unknown)", String())
}

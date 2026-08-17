//go:build deps

package version

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestPlannedTestDependencies(t *testing.T) {
	_, _, err := sqlmock.New()
	require.NoError(t, err)

	server := miniredis.RunT(t)
	require.NotEmpty(t, server.Addr())
}

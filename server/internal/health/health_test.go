package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLiveness(t *testing.T) {
	response := httptest.NewRecorder()

	liveness(response, httptest.NewRequest(http.MethodGet, livenessPath, nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "ok\n", response.Body.String())
}

func TestReadiness(t *testing.T) {
	probe := NewProbe()

	notReady := httptest.NewRecorder()
	probe.readiness(notReady, httptest.NewRequest(http.MethodGet, readinessPath, nil))
	require.Equal(t, http.StatusServiceUnavailable, notReady.Code)

	probe.SetReady(true)
	ready := httptest.NewRecorder()
	probe.readiness(ready, httptest.NewRequest(http.MethodGet, readinessPath, nil))
	require.Equal(t, http.StatusOK, ready.Code)
	require.Equal(t, "ready\n", ready.Body.String())
}

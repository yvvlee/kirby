package health

import (
	"net/http"
	"sync/atomic"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
)

const (
	livenessPath  = "/healthz"
	readinessPath = "/readyz"
)

// Probe exposes the process lifecycle state without probing business dependencies.
type Probe struct {
	ready atomic.Bool
}

// NewProbe creates a process probe in the not-ready state.
// Parameters: none.
// Returns: a probe that starts refusing traffic until server startup succeeds.
// Throws: none.
func NewProbe() *Probe {
	return &Probe{}
}

// SetReady changes whether the process is ready to receive traffic.
// Parameters: ready is the lifecycle state shared with readiness requests.
// Returns: none.
// Throws: none.
func (p *Probe) SetReady(ready bool) {
	p.ready.Store(ready)
}

// Register installs liveness and readiness endpoints on the HTTP server.
// Parameters: server owns the routes; probe supplies the current lifecycle state.
// Returns: none.
// Throws: none.
func Register(server *kratoshttp.Server, probe *Probe) {
	server.Handle(livenessPath, http.HandlerFunc(liveness))
	server.Handle(readinessPath, http.HandlerFunc(probe.readiness))
}

func liveness(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("ok\n"))
}

func (p *Probe) readiness(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if !p.ready.Load() {
		response.WriteHeader(http.StatusServiceUnavailable)
		_, _ = response.Write([]byte("not ready\n"))
		return
	}

	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("ready\n"))
}

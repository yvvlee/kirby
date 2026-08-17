package middleware

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/yvvlee/kirby/server/internal/config"
)

// MaxForwardedForHops bounds work and memory used to parse one proxy chain.
const MaxForwardedForHops = 32

// ClientIPResolver resolves a client address without trusting forwarding
// headers sent by untrusted peers.
type ClientIPResolver struct {
	trusted []netip.Prefix
}

// NewClientIPResolver constructs a resolver from strict trusted proxy CIDRs.
// An empty list means every request is treated as a direct connection.
func NewClientIPResolver(trustedCIDRs []string) (*ClientIPResolver, error) {
	prefixes, err := config.ParseTrustedProxyCIDRs(trustedCIDRs)
	if err != nil {
		return nil, err
	}
	return &ClientIPResolver{trusted: prefixes}, nil
}

// Resolve returns the canonical client IP. A trusted peer must supply a valid
// X-Forwarded-For chain; an untrusted peer's header is ignored completely.
func (resolver *ClientIPResolver) Resolve(request *http.Request) (netip.Addr, error) {
	if resolver == nil {
		return netip.Addr{}, fmt.Errorf("client IP resolver is nil")
	}
	if request == nil {
		return netip.Addr{}, fmt.Errorf("HTTP request is nil")
	}
	remote, err := parseRemoteAddr(request.RemoteAddr)
	if err != nil {
		return netip.Addr{}, err
	}
	if !resolver.isTrusted(remote) {
		return remote, nil
	}

	values := request.Header.Values("X-Forwarded-For")
	if len(values) == 0 {
		return netip.Addr{}, fmt.Errorf("trusted proxy did not supply X-Forwarded-For")
	}
	parts := strings.Split(strings.Join(values, ","), ",")
	if len(parts) == 0 || len(parts) > MaxForwardedForHops {
		return netip.Addr{}, fmt.Errorf("X-Forwarded-For chain must contain between 1 and %d addresses", MaxForwardedForHops)
	}
	chain := make([]netip.Addr, len(parts))
	for index, part := range parts {
		value := strings.TrimSpace(part)
		address, parseErr := netip.ParseAddr(value)
		if value == "" || parseErr != nil || address.Zone() != "" {
			return netip.Addr{}, fmt.Errorf("X-Forwarded-For contains invalid IP at position %d", index+1)
		}
		chain[index] = address.Unmap()
	}

	for index := len(chain) - 1; index >= 0; index-- {
		if !resolver.isTrusted(chain[index]) {
			return chain[index], nil
		}
	}
	return chain[0], nil
}

func (resolver *ClientIPResolver) isTrusted(address netip.Addr) bool {
	for _, prefix := range resolver.trusted {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func parseRemoteAddr(value string) (netip.Addr, error) {
	if addressPort, err := netip.ParseAddrPort(value); err == nil {
		address := addressPort.Addr()
		if address.Zone() == "" {
			return address.Unmap(), nil
		}
	}
	address, err := netip.ParseAddr(value)
	if err != nil || address.Zone() != "" {
		return netip.Addr{}, fmt.Errorf("RemoteAddr does not contain a valid IP address")
	}
	return address.Unmap(), nil
}

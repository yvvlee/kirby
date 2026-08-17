package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resolverRequest(remote, forwarded string) *http.Request {
	request, _ := http.NewRequest(http.MethodPost, "https://kirby.example.com/auth/login", nil)
	request.RemoteAddr = remote
	if forwarded != "" {
		request.Header.Set("X-Forwarded-For", forwarded)
	}
	return request
}

func TestClientIPResolverIgnoresForgedHeaderFromUntrustedPeer(t *testing.T) {
	resolver, err := NewClientIPResolver([]string{"10.0.0.0/8"})
	require.NoError(t, err)

	request := resolverRequest("192.0.2.10:1234", "not-an-ip, 198.51.100.8")
	address, err := resolver.Resolve(request)
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.10", address.String())
}

func TestClientIPResolverUsesSingleTrustedProxy(t *testing.T) {
	resolver, err := NewClientIPResolver([]string{"10.0.0.0/8"})
	require.NoError(t, err)

	address, err := resolver.Resolve(
		resolverRequest("10.0.0.2:443", "203.0.113.7"),
	)
	require.NoError(t, err)
	assert.Equal(t, "203.0.113.7", address.String())
}

func TestClientIPResolverPeelsMultipleTrustedProxiesRightToLeft(t *testing.T) {
	resolver, err := NewClientIPResolver([]string{"10.0.0.0/8", "192.168.0.0/16"})
	require.NoError(t, err)

	address, err := resolver.Resolve(
		resolverRequest("10.0.0.2:443", "198.51.100.9, 192.168.1.4"),
	)
	require.NoError(t, err)
	assert.Equal(t, "198.51.100.9", address.String())
}

func TestClientIPResolverStopsAtNearestUntrustedProxy(t *testing.T) {
	resolver, err := NewClientIPResolver([]string{"10.0.0.0/8"})
	require.NoError(t, err)

	address, err := resolver.Resolve(
		resolverRequest("10.0.0.2:443", "198.51.100.200, 203.0.113.8"),
	)
	require.NoError(t, err)
	assert.Equal(t, "203.0.113.8", address.String())
}

func TestClientIPResolverRejectsMissingOrInvalidTrustedHeader(t *testing.T) {
	resolver, err := NewClientIPResolver([]string{"10.0.0.0/8"})
	require.NoError(t, err)

	for _, forwarded := range []string{"", "unknown", "203.0.113.8:1234", "203.0.113.8,,10.0.0.3"} {
		t.Run(forwarded, func(t *testing.T) {
			_, err := resolver.Resolve(resolverRequest("10.0.0.2:443", forwarded))
			require.Error(t, err)
		})
	}
}

func TestClientIPResolverLimitsForwardedChain(t *testing.T) {
	resolver, err := NewClientIPResolver([]string{"10.0.0.0/8"})
	require.NoError(t, err)
	addresses := make([]string, MaxForwardedForHops+1)
	for index := range addresses {
		addresses[index] = fmt.Sprintf("192.0.2.%d", index+1)
	}

	_, err = resolver.Resolve(
		resolverRequest("10.0.0.2:443", strings.Join(addresses, ", ")),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprint(MaxForwardedForHops))
}

func TestClientIPResolverRejectsInvalidRemoteAddress(t *testing.T) {
	resolver, err := NewClientIPResolver(nil)
	require.NoError(t, err)
	_, err = resolver.Resolve(resolverRequest("proxy.internal:443", "203.0.113.8"))
	require.Error(t, err)
}

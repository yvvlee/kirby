package service

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoginReplyRejectsLifetimeOverflow(t *testing.T) {
	for _, ttl := range []time.Duration{-time.Second, (time.Duration(math.MaxUint32) + 1) * time.Second} {
		reply, err := loginReply("token", ttl, nil)
		require.Error(t, err)
		require.Nil(t, reply)
	}
	reply, err := loginReply("token", time.Duration(math.MaxUint32)*time.Second, nil)
	require.NoError(t, err)
	require.Equal(t, uint32(math.MaxUint32), reply.ExpiresIn)
}

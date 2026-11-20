package passwordpolicy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordPolicy(t *testing.T) {
	for _, value := range []string{"Abcdefg1", "abcdefg!", "1234567!", strings.Repeat("a", 31) + "1"} {
		require.NoError(t, Validate(value))
	}
	for _, value := range []string{"Abcdefg", "Abcdefgh", "12345678", "!!!!!!!!", "abcdefg ", "abcdef\t1", "abcdef\n1", "abcdef\x7f1", "abc密码1!", strings.Repeat("a", 32) + "1"} {
		require.Error(t, Validate(value))
	}
}

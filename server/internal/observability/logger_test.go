package observability

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/config"
)

func TestLoggerRedactsSensitiveFieldsAndMessageText(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewLogger(&output, config.LogConfig{Level: "info", Format: "json"})
	require.NoError(t, err)

	logger.Info(
		`request failed: {"password":"message-password","name":"alice"} authorization=message-token api_key_pepper=message-pepper X-Amz-Signature=signed-value`,
		"password", "attribute-password",
		"authorization", "Bearer attribute-token",
		"cookie", "kirby_session=cookie-secret",
		"api_key_pepper", "shared-pepper-secret",
		"request_body", []byte(`{"secret":"body-secret"}`),
		"safe", "Bearer embedded-token",
		slog.Group("credentials", "api_key", "key-value", "name", "visible"),
	)

	logged := output.String()
	for _, secret := range []string{
		"message-password", "message-token", "message-pepper", "signed-value", "attribute-password",
		"attribute-token", "cookie-secret", "shared-pepper-secret", "body-secret", "embedded-token", "key-value",
	} {
		assert.NotContains(t, logged, secret)
	}
	assert.Contains(t, logged, redacted)
	assert.Contains(t, logged, "alice")
	assert.Contains(t, logged, "visible")
}

func TestLoggerRedactsJWTAndConfiguredSecret(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewLogger(&output, config.LogConfig{Level: "debug", Format: "text"})
	require.NoError(t, err)
	secret := config.NewSecret("configured-secret")
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature"

	logger.Debug("received "+jwt, "config_value", secret, "dsn", "user:password@tcp/db")
	logged := output.String()
	assert.NotContains(t, logged, jwt)
	assert.NotContains(t, logged, "configured-secret")
	assert.NotContains(t, logged, "user:password")
	assert.Contains(t, logged, redacted)
}

func TestNewLoggerRejectsUnknownSettings(t *testing.T) {
	_, err := NewLogger(&bytes.Buffer{}, config.LogConfig{Level: "trace", Format: "json"})
	require.Error(t, err)
	_, err = NewLogger(&bytes.Buffer{}, config.LogConfig{Level: "info", Format: "xml"})
	require.Error(t, err)
}

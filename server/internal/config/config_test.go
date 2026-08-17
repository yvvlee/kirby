package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFileStrictAndComplete(t *testing.T) {
	path := writeConfig(t, validConfig("single", "memory", "local"))
	cfg, err := LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, ModeSingle, cfg.Mode)
	assert.Equal(t, 15*time.Minute, cfg.JWT.AccessTTL.Duration)
	assert.Equal(t, "mysql-secret", cfg.MySQL.DSN.Value())
}

func TestExampleConfigurationRejectsPlaceholderSecrets(t *testing.T) {
	path := filepath.Join("..", "..", "..", "deploy", "config.example.yaml")
	_, err := LoadFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "active JWT signing key must contain at least 32 bytes")

	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	withValidJWT := strings.Replace(string(content), "CHANGE_ME_JWT_KEY", "01234567890123456789012345678901", 1)
	_, err = LoadFile(writeConfig(t, withValidJWT))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "security.api_key_pepper must contain at least 32 bytes")
}

func TestLoadFileRejectsUnknownField(t *testing.T) {
	path := writeConfig(t, validConfig("single", "memory", "local")+"unknown: true\n")
	_, err := LoadFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field unknown not found")
}

func TestLoadFileRejectsMissingRequiredField(t *testing.T) {
	content := strings.Replace(validConfig("single", "memory", "local"), "  dsn: mysql-secret\n", "", 1)
	_, err := LoadFile(writeConfig(t, content))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mysql.dsn is required")
}

func TestSecuritySettingsAreRequired(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "missing pepper",
			content: strings.Replace(validConfig("single", "memory", "local"), "  api_key_pepper: \"01234567890123456789012345678901\"\n", "", 1),
			want:    "security.api_key_pepper",
		},
		{
			name:    "short pepper",
			content: strings.Replace(validConfig("single", "memory", "local"), "api_key_pepper: \"01234567890123456789012345678901\"", "api_key_pepper: \"too-short\"", 1),
			want:    "at least 32 bytes",
		},
		{
			name:    "missing origins",
			content: strings.Replace(validConfig("single", "memory", "local"), "  allowed_origins:\n    - \"https://kirby.example.com\"\n", "", 1),
			want:    "security.allowed_origins",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadFile(writeConfig(t, test.content))
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestAllowedOriginsMustBeExactHTTPOrigins(t *testing.T) {
	tests := []string{
		"*",
		"https://*.example.com",
		"ftp://kirby.example.com",
		"https://kirby.example.com/",
		"https://kirby.example.com/path",
		"https://kirby.example.com?query=value",
		"https://kirby.example.com#fragment",
		"https://kirby.example.com#",
		"https://kirby.example.com:70000",
		"https://user@kirby.example.com",
	}
	for _, origin := range tests {
		t.Run(origin, func(t *testing.T) {
			content := strings.Replace(
				validConfig("single", "memory", "local"),
				"https://kirby.example.com",
				origin,
				1,
			)
			_, err := LoadFile(writeConfig(t, content))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "security.allowed_origins")
		})
	}
}

func TestAllowedOriginsAcceptHTTPAndExplicitPort(t *testing.T) {
	content := strings.Replace(
		validConfig("single", "memory", "local"),
		"https://kirby.example.com",
		"http://localhost:8080",
		1,
	)
	_, err := LoadFile(writeConfig(t, content))
	require.NoError(t, err)
}

func TestLoadFileRejectsMultipleDocuments(t *testing.T) {
	path := writeConfig(t, validConfig("single", "memory", "local")+"---\nmode: single\n")
	_, err := LoadFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple YAML documents")
}

func TestMultiModeRequiresSharedDependencies(t *testing.T) {
	tests := []struct {
		name    string
		cache   string
		storage string
		want    string
	}{
		{name: "memory cache", cache: "memory", storage: "s3", want: "cache.driver=redis"},
		{name: "local storage", cache: "redis", storage: "local", want: "object_storage.driver=s3"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadFile(writeConfig(t, validConfig("multi", test.cache, test.storage)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestResolvePathPrefersFlag(t *testing.T) {
	path, err := ResolvePath("./flag.yaml", func(string) (string, bool) {
		return "/environment.yaml", true
	})
	require.NoError(t, err)
	assert.Equal(t, "flag.yaml", path)
}

func TestResolvePathUsesOnlyNamedEnvironmentVariable(t *testing.T) {
	path, err := ResolvePath("", func(key string) (string, bool) {
		assert.Equal(t, ConfigFileEnvironment, key)
		return "/environment.yaml", true
	})
	require.NoError(t, err)
	assert.Equal(t, "/environment.yaml", path)

	_, err = ResolvePath("", func(string) (string, bool) { return "", false })
	require.Error(t, err)
}

func TestSecretCannotBeFormattedOrLogged(t *testing.T) {
	secret := NewSecret("do-not-print")
	assert.Equal(t, redactedValue, fmt.Sprint(secret))
	assert.Equal(t, redactedValue, fmt.Sprintf("%#v", secret))
	encoded, err := secret.MarshalYAML()
	require.NoError(t, err)
	assert.Equal(t, redactedValue, encoded)
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kirby.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func validConfig(mode, cacheDriver, storageDriver string) string {
	return fmt.Sprintf(`mode: %s
http:
  address: ":8000"
  timeout: 10s
grpc:
  address: ":9000"
  timeout: 10s
mysql:
  dsn: mysql-secret
  max_open_conns: 20
  max_idle_conns: 10
  conn_max_lifetime: 5m
cache:
  driver: %s
  redis:
    address: "redis:6379"
    username: ""
    password: redis-secret
    db: 0
    key_prefix: "kirby:"
jwt:
  issuer: kirby
  active_kid: primary
  access_ttl: 15m
  refresh_ttl: 168h
  keys:
    primary: "01234567890123456789012345678901"
security:
  api_key_pepper: "01234567890123456789012345678901"
  allowed_origins:
    - "https://kirby.example.com"
object_storage:
  driver: %s
  local:
    directory: ./data/assets
  s3:
    endpoint: minio:9000
    region: us-east-1
    bucket: kirby
    access_key: access-secret
    secret_key: object-secret
    use_ssl: false
log:
  level: info
  format: json
`, mode, cacheDriver, storageDriver)
}

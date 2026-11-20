package conf

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigLoadsFileSource(t *testing.T) {
	t.Setenv(ConfigFileEnvironment, writeConfig(t, validConfig("single", "memory", "local")))
	cfg, cleanup, err := NewConfig("")
	require.NoError(t, err)
	t.Cleanup(cleanup)
	assert.Equal(t, ModeSingle, cfg.Mode)
	assert.Equal(t, 15*60*1000, cfg.JWT.AccessTTL)
	assert.Equal(t, "mysql-secret", cfg.Database.DSN)
}

func TestExampleConfigurationNeedsOnlyDependencyCredentials(t *testing.T) {
	t.Setenv(ConfigFileEnvironment, filepath.Join("..", "..", "..", "deploy", "config.example.yaml"))
	cfg, cleanup, err := NewConfig("")
	require.NoError(t, err)
	t.Cleanup(cleanup)
	assert.Equal(t, ModeMulti, cfg.Mode)
	assert.Equal(t, "kirby", cfg.JWT.Issuer)
	require.NoError(t, cfg.Validate())
}

func TestNewConfigRejectsMissingRequiredField(t *testing.T) {
	content := strings.Replace(validConfig("single", "memory", "local"), "  dsn: mysql-secret\n", "", 1)
	t.Setenv(ConfigFileEnvironment, writeConfig(t, content))
	_, _, err := NewConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database.dsn is required")
}

func TestNewConfigRejectsLegacyMySQLSection(t *testing.T) {
	content := strings.Replace(validConfig("single", "memory", "local"), "\ndatabase:\n", "\nmysql:\n", 1)
	t.Setenv(ConfigFileEnvironment, writeConfig(t, content))
	_, _, err := NewConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database.driver")
}

func TestListenerAddressesMustNotConflict(t *testing.T) {
	for _, grpcAddress := range []string{":8000", "0.0.0.0:8000", "[::]:8000"} {
		content := strings.Replace(
			validConfig("single", "memory", "local"),
			`address: ":9000"`,
			`address: "`+grpcAddress+`"`,
			1,
		)
		t.Setenv(ConfigFileEnvironment, writeConfig(t, content))
		_, _, err := NewConfig("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "different listeners")
	}
}

func TestTrustedProxiesDefaultToReverseProxySupport(t *testing.T) {
	base := validConfig("single", "memory", "local")
	configs := []string{
		strings.Replace(base, "security:\n  trusted_proxies: []\n", "", 1),
		strings.Replace(base, "  trusted_proxies: []\n", "", 1),
		strings.Replace(base, "trusted_proxies: []", "trusted_proxies: null", 1),
	}
	want := []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/0"),
		netip.MustParsePrefix("::/0"),
	}
	for _, content := range configs {
		t.Setenv(ConfigFileEnvironment, writeConfig(t, content))
		cfg, cleanup, err := NewConfig("")
		require.NoError(t, err)
		t.Cleanup(cleanup)
		prefixes, err := ParseTrustedProxyCIDRs(cfg.Security.TrustedProxies)
		require.NoError(t, err)
		assert.Equal(t, want, prefixes)
	}
}

func TestTrustedProxiesAreStrictCanonicalCIDRs(t *testing.T) {
	base := validConfig("single", "memory", "local")
	for _, value := range []string{
		`"10.0.0.1"`,
		`" 10.0.0.0/8"`,
		`"10.0.0.1/8"`,
		`"010.0.0.0/8"`,
		`"::ffff:10.0.0.0/120"`,
	} {
		t.Run(value, func(t *testing.T) {
			content := strings.Replace(base, "  trusted_proxies: []", "  trusted_proxies:\n    - "+value, 1)
			t.Setenv(ConfigFileEnvironment, writeConfig(t, content))
			_, _, err := NewConfig("")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "security.trusted_proxies")
		})
	}

	duplicate := strings.Replace(
		base,
		"  trusted_proxies: []",
		"  trusted_proxies:\n    - \"10.0.0.0/8\"\n    - \"10.0.0.0/8\"",
		1,
	)
	t.Setenv(ConfigFileEnvironment, writeConfig(t, duplicate))
	_, _, err := NewConfig("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate CIDR")
}

func TestTrustedProxiesAllowEmptyAndExactNetworks(t *testing.T) {
	content := strings.Replace(
		validConfig("single", "memory", "local"),
		"  trusted_proxies: []",
		"  trusted_proxies:\n    - \"10.0.0.0/8\"\n    - \"2001:db8::/32\"",
		1,
	)
	t.Setenv(ConfigFileEnvironment, writeConfig(t, content))
	cfg, cleanup, err := NewConfig("")
	require.NoError(t, err)
	t.Cleanup(cleanup)
	assert.Equal(t, []string{"10.0.0.0/8", "2001:db8::/32"}, cfg.Security.TrustedProxies)

	t.Setenv(ConfigFileEnvironment, writeConfig(t, validConfig("single", "memory", "local")))
	cfg, cleanup, err = NewConfig("")
	require.NoError(t, err)
	t.Cleanup(cleanup)
	assert.NotNil(t, cfg.Security.TrustedProxies)
	assert.Empty(t, cfg.Security.TrustedProxies)
	prefixes, err := ParseTrustedProxyCIDRs(cfg.Security.TrustedProxies)
	require.NoError(t, err)
	assert.Empty(t, prefixes)
}

func TestMultiModeRequiresSharedDependencies(t *testing.T) {
	tests := []struct {
		name    string
		cache   string
		storage string
		want    string
	}{
		{
			name:    "memory cache",
			cache:   "memory",
			storage: "s3",
			want:    "cache.driver=redis",
		},
		{
			name:    "local storage",
			cache:   "redis",
			storage: "local",
			want:    "object_storage.driver=s3",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(ConfigFileEnvironment, writeConfig(t, validConfig("multi", test.cache, test.storage)))
			_, _, err := NewConfig("")
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestS3ConfigurationRequiresInternalConnection(t *testing.T) {
	base := validConfig("single", "memory", "s3")
	tests := []struct {
		name     string
		previous string
		next     string
		want     string
	}{
		{
			name:     "missing internal endpoint",
			previous: "    endpoint: minio:9000\n",
			want:     "object_storage.s3.endpoint",
		},
		{
			name:     "internal endpoint has scheme",
			previous: "endpoint: minio:9000",
			next:     "endpoint: http://minio:9000",
			want:     "object_storage.s3.endpoint",
		},
		{
			name:     "internal endpoint has path",
			previous: "endpoint: minio:9000",
			next:     "endpoint: minio:9000/upload",
			want:     "object_storage.s3.endpoint",
		},
		{
			name:     "internal endpoint has credentials",
			previous: "endpoint: minio:9000",
			next:     "endpoint: user@minio:9000",
			want:     "object_storage.s3.endpoint",
		},
		{
			name:     "internal endpoint has invalid port",
			previous: "endpoint: minio:9000",
			next:     "endpoint: minio:70000",
			want:     "object_storage.s3.endpoint",
		},
		{
			name:     "missing bucket",
			previous: "    bucket: kirby\n",
			want:     "object_storage.s3.bucket is required",
		},
		{
			name:     "missing access key",
			previous: "    access_key: access-secret\n",
			want:     "object_storage.s3 access_key and secret_key are required",
		},
		{
			name:     "missing secret key",
			previous: "    secret_key: object-secret\n",
			want:     "object_storage.s3 access_key and secret_key are required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := strings.Replace(base, test.previous, test.next, 1)
			t.Setenv(ConfigFileEnvironment, writeConfig(t, content))
			_, _, err := NewConfig("")
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

// TestS3ConfigurationLoadsWithoutPublicDomain verifies that internal S3 settings
// suffice when the application serves browser uploads and downloads itself.
// Parameters: t controls the configuration-loading regression test.
// Returns: none.
// Throws: none; invalid configurations fail the test.
func TestS3ConfigurationLoadsWithoutPublicDomain(t *testing.T) {
	base := validConfig("multi", "redis", "s3")
	for _, replacement := range []string{"", "    public_base_url: \"\"\n"} {
		content := strings.Replace(base, "    public_base_url: \"https://cdn.example.com/kirby/assets\"\n", replacement, 1)
		t.Setenv(ConfigFileEnvironment, writeConfig(t, content))
		cfg, cleanup, err := NewConfig("")
		require.NoError(t, err)
		t.Cleanup(cleanup)
		assert.Equal(t, "minio:9000", cfg.ObjectStorage.S3.Endpoint)
		assert.Empty(t, cfg.ObjectStorage.S3.PublicBaseURL)
	}
}

func TestS3PublicBaseURLIsStrictAndAllowsFixedPath(t *testing.T) {
	base := validConfig("single", "memory", "s3")
	invalidValues := []string{
		" ",
		" https://cdn.example.com/kirby",
		"https://cdn.example.com/kirby ",
		"/kirby/assets",
		"ftp://cdn.example.com/kirby",
		"https://user@cdn.example.com/kirby",
		"https://cdn.example.com/kirby?token=value",
		"https://cdn.example.com/kirby#fragment",
		"https://cdn.example.com/kirby/../private",
		"https://cdn.example.com/kirby%2Fprivate",
		"https://*.example.com/kirby",
	}
	for _, value := range invalidValues {
		t.Run(value, func(t *testing.T) {
			content := strings.Replace(
				base,
				"https://cdn.example.com/kirby/assets",
				value,
				1,
			)
			t.Setenv(ConfigFileEnvironment, writeConfig(t, content))
			_, _, err := NewConfig("")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "object_storage.s3.public_base_url")
		})
	}

	for _, value := range []string{
		"http://localhost:9000/kirby",
		"https://cdn.example.com/kirby/assets/",
	} {
		t.Run("valid "+value, func(t *testing.T) {
			content := strings.Replace(
				base,
				"https://cdn.example.com/kirby/assets",
				value,
				1,
			)
			t.Setenv(ConfigFileEnvironment, writeConfig(t, content))
			_, cleanup, err := NewConfig("")
			require.NoError(t, err)
			t.Cleanup(cleanup)
		})
	}
}

func TestResolvePathPrefersFlag(t *testing.T) {
	path := ResolvePath("./flag.yaml", func(string) (string, bool) {
		return "/environment.yaml", true
	})
	assert.Equal(t, "flag.yaml", path)
}

func TestResolvePathUsesOnlyNamedEnvironmentVariable(t *testing.T) {
	path := ResolvePath("", func(key string) (string, bool) {
		assert.Equal(t, ConfigFileEnvironment, key)
		return "/environment.yaml", true
	})
	assert.Equal(t, "/environment.yaml", path)

	path = ResolvePath("", func(string) (string, bool) { return "", false })
	assert.Equal(t, "", path)
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
  timeout: 10000
grpc:
  address: ":9000"
  timeout: 10000
database:
  driver: mysql
  dsn: mysql-secret
  max_open_conns: 20
  max_idle_conns: 10
  conn_max_lifetime: 300000
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
  access_ttl: 900000
  refresh_ttl: 604800000
security:
  trusted_proxies: []
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
    public_base_url: "https://cdn.example.com/kirby/assets"
log:
  level: info
`, mode, cacheDriver, storageDriver)
}

// Package conf loads and validates Kirby's configuration sources.
package conf

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// App 全局应用配置，由 NewConfig 在加载完成后赋值。
var App *AppConfig

// DeploymentMode controls whether process-local dependencies are permitted.
type DeploymentMode string

const (
	ModeSingle DeploymentMode = "single"
	ModeMulti  DeploymentMode = "multi"
)

// Env 部署环境
type Env string

const (
	EnvProduction Env = "prod"
	EnvTest       Env = "test"
	EnvDev        Env = "dev"
	EnvLocal      Env = "local"
)

// AppConfig 应用基本配置
type AppConfig struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Env     Env    `json:"env"`
}

// IsProduction 是否生产环境
//
// Parameters: none.
// Returns: whether the deployment environment is production.
// Throws: none.
func (a *AppConfig) IsProduction() bool {
	return a.Env == EnvProduction
}

// Config 全局Config，时长字段一律使用毫秒
type Config struct {
	Mode          DeploymentMode      `json:"mode"`
	App           *AppConfig          `json:"app"`
	HTTP          ListenerConfig      `json:"http"`
	GRPC          ListenerConfig      `json:"grpc"`
	Database      DatabaseConfig      `json:"database"`
	Cache         CacheConfig         `json:"cache"`
	JWT           JWTConfig           `json:"jwt"`
	Security      SecurityConfig      `json:"security"`
	ObjectStorage ObjectStorageConfig `json:"object_storage"`
	Log           LogConfig           `json:"log"`
}

// ListenerConfig configures one network listener.
type ListenerConfig struct {
	Address string `json:"address"`
	Timeout int    `json:"timeout"`
}

// DatabaseDriver selects the relational database dialect.
type DatabaseDriver string

const (
	DatabaseDriverMySQL DatabaseDriver = "mysql"
	DatabaseDriverPGSQL DatabaseDriver = "pgsql"
)

// DatabaseConfig configures the shared relational database.
type DatabaseConfig struct {
	Driver          DatabaseDriver `json:"driver"`
	DSN             string         `json:"dsn"`
	MaxOpenConns    int            `json:"max_open_conns"`
	MaxIdleConns    int            `json:"max_idle_conns"`
	ConnMaxLifetime int            `json:"conn_max_lifetime"`
}

// CacheConfig selects a process-local or shared cache.
type CacheConfig struct {
	Driver string      `json:"driver"`
	Redis  RedisConfig `json:"redis"`
}

// RedisConfig configures a shared Redis cache.
type RedisConfig struct {
	Address   string `json:"address"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	DB        int    `json:"db"`
	KeyPrefix string `json:"key_prefix"`
}

// JWTConfig configures token identity and lifetimes; signing keys are stored in the database.
type JWTConfig struct {
	Issuer     string `json:"issuer"`
	AccessTTL  int    `json:"access_ttl"`
	RefreshTTL int    `json:"refresh_ttl"`
}

// SecurityConfig identifies proxies trusted to forward client request metadata.
type SecurityConfig struct {
	TrustedProxies []string `json:"trusted_proxies"`
}

// ObjectStorageConfig selects local or S3-compatible object storage.
type ObjectStorageConfig struct {
	Driver string      `json:"driver"`
	Local  LocalConfig `json:"local"`
	S3     S3Config    `json:"s3"`
}

// LocalConfig configures single-instance local object storage.
type LocalConfig struct {
	Directory string `json:"directory"`
}

// S3Config configures an S3-compatible shared object store.
type S3Config struct {
	Endpoint      string `json:"endpoint"`
	Region        string `json:"region"`
	Bucket        string `json:"bucket"`
	AccessKey     string `json:"access_key"`
	SecretKey     string `json:"secret_key"`
	UseSSL        bool   `json:"use_ssl"`
	PublicBaseURL string `json:"public_base_url"`
}

// LogConfig controls structured application logging.
type LogConfig struct {
	Level string `json:"level"`
}

// Validate rejects incomplete, ambiguous, or unsafe deployment combinations.
//
// Parameters: none.
// Returns: nil for a valid configuration.
// Throws: an error naming the first invalid setting.
func (c Config) Validate() error {
	if c.Mode != ModeSingle && c.Mode != ModeMulti {
		return fmt.Errorf("mode must be %q or %q", ModeSingle, ModeMulti)
	}
	if err := validateListener("http", c.HTTP); err != nil {
		return err
	}
	if err := validateListener("grpc", c.GRPC); err != nil {
		return err
	}
	if listenerAddressesConflict(c.HTTP.Address, c.GRPC.Address) {
		return fmt.Errorf("http.address and grpc.address must use different listeners")
	}
	if err := c.Database.validate(); err != nil {
		return err
	}
	if err := c.Cache.validate(); err != nil {
		return err
	}
	if err := c.JWT.validate(); err != nil {
		return err
	}
	if err := c.Security.validate(); err != nil {
		return err
	}
	if err := c.ObjectStorage.validate(); err != nil {
		return err
	}
	if c.Mode == ModeMulti && c.Cache.Driver != "redis" {
		return fmt.Errorf("mode=multi requires cache.driver=redis")
	}
	if c.Mode == ModeMulti && c.ObjectStorage.Driver != "s3" {
		return fmt.Errorf("mode=multi requires object_storage.driver=s3")
	}
	return nil
}

func (database DatabaseConfig) validate() error {
	if database.Driver != DatabaseDriverMySQL && database.Driver != DatabaseDriverPGSQL {
		return fmt.Errorf("database.driver must be %q or %q", DatabaseDriverMySQL, DatabaseDriverPGSQL)
	}
	if database.DSN == "" {
		return fmt.Errorf("database.dsn is required")
	}
	if database.MaxOpenConns <= 0 {
		return fmt.Errorf("database.max_open_conns must be greater than zero")
	}
	if database.MaxIdleConns < 0 || database.MaxIdleConns > database.MaxOpenConns {
		return fmt.Errorf("database.max_idle_conns must be between zero and max_open_conns")
	}
	if database.ConnMaxLifetime <= 0 {
		return fmt.Errorf("database.conn_max_lifetime must be greater than zero")
	}
	return nil
}

func listenerAddressesConflict(first, second string) bool {
	firstHost, firstPort, firstErr := net.SplitHostPort(strings.TrimSpace(first))
	secondHost, secondPort, secondErr := net.SplitHostPort(strings.TrimSpace(second))
	if firstErr != nil || secondErr != nil || firstPort != secondPort {
		return strings.TrimSpace(first) == strings.TrimSpace(second)
	}
	normalizeHost := func(host string) string {
		switch strings.TrimSpace(strings.Trim(host, "[]")) {
		case "", "0.0.0.0", "::":
			return "*"
		default:
			return strings.ToLower(host)
		}
	}
	return normalizeHost(firstHost) == normalizeHost(secondHost)
}

func validateListener(name string, listener ListenerConfig) error {
	if strings.TrimSpace(listener.Address) == "" {
		return fmt.Errorf("%s.address is required", name)
	}
	if listener.Timeout <= 0 {
		return fmt.Errorf("%s.timeout must be greater than zero", name)
	}
	return nil
}

func (c CacheConfig) validate() error {
	switch c.Driver {
	case "memory":
		return nil
	case "redis":
		if strings.TrimSpace(c.Redis.Address) == "" {
			return fmt.Errorf("cache.redis.address is required")
		}
		if c.Redis.DB < 0 {
			return fmt.Errorf("cache.redis.db cannot be negative")
		}
		if strings.TrimSpace(c.Redis.KeyPrefix) == "" {
			return fmt.Errorf("cache.redis.key_prefix is required")
		}
		return nil
	default:
		return fmt.Errorf("cache.driver must be %q or %q", "memory", "redis")
	}
}

func (c JWTConfig) validate() error {
	if strings.TrimSpace(c.Issuer) == "" {
		return fmt.Errorf("jwt.issuer is required")
	}
	if c.AccessTTL <= 0 {
		return fmt.Errorf("jwt.access_ttl must be greater than zero")
	}
	if c.RefreshTTL <= c.AccessTTL {
		return fmt.Errorf("jwt.refresh_ttl must be greater than access_ttl")
	}
	return nil
}

func (c SecurityConfig) validate() error {
	if _, err := ParseTrustedProxyCIDRs(c.TrustedProxies); err != nil {
		return err
	}
	return nil
}

// ParseTrustedProxyCIDRs parses proxy networks while allowing reverse proxies
// without additional deployment configuration.
//
// Parameters:
//   - values: optional canonical CIDRs; nil trusts all proxies, while [] trusts none.
//
// Returns: parsed networks or a configuration error.
// Throws: none.
func ParseTrustedProxyCIDRs(values []string) ([]netip.Prefix, error) {
	if values == nil {
		return []netip.Prefix{
			netip.MustParsePrefix("0.0.0.0/0"),
			netip.MustParsePrefix("::/0"),
		}, nil
	}
	prefixes := make([]netip.Prefix, 0, len(values))
	seen := make(map[netip.Prefix]struct{}, len(values))
	for _, value := range values {
		if value == "" || strings.TrimSpace(value) != value {
			return nil, fmt.Errorf("security.trusted_proxies contains an empty or whitespace-padded CIDR")
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil || !prefix.IsValid() || prefix.Addr().Zone() != "" || prefix.Addr().Is4In6() {
			return nil, fmt.Errorf("security.trusted_proxies contains invalid CIDR %q", value)
		}
		masked := prefix.Masked()
		if prefix != masked || value != masked.String() {
			return nil, fmt.Errorf("security.trusted_proxies contains non-canonical CIDR %q", value)
		}
		if _, exists := seen[masked]; exists {
			return nil, fmt.Errorf("security.trusted_proxies contains duplicate CIDR %q", value)
		}
		seen[masked] = struct{}{}
		prefixes = append(prefixes, masked)
	}
	return prefixes, nil
}

func (c ObjectStorageConfig) validate() error {
	switch c.Driver {
	case "local":
		if strings.TrimSpace(c.Local.Directory) == "" {
			return fmt.Errorf("object_storage.local.directory is required")
		}
		return nil
	case "s3":
		return c.S3.Validate()
	default:
		return fmt.Errorf("object_storage.driver must be %q or %q", "local", "s3")
	}
}

// Validate requires server-side S3 connectivity without requiring a public
// storage domain. An explicit CDN URL must remain safe to use in browser links.
//
// Parameters: none; the receiver contains the S3 connection and optional CDN URL.
// Returns: an error for incomplete connection settings or an invalid CDN URL.
// Throws: none.
func (c S3Config) Validate() error {
	if err := validateEndpoint("object_storage.s3.endpoint", c.Endpoint); err != nil {
		return err
	}
	if strings.TrimSpace(c.Bucket) == "" {
		return fmt.Errorf("object_storage.s3.bucket is required")
	}
	if c.AccessKey == "" || c.SecretKey == "" {
		return fmt.Errorf("object_storage.s3 access_key and secret_key are required")
	}
	if c.PublicBaseURL == "" {
		return nil
	}
	return validatePublicBaseURL(c.PublicBaseURL)
}

func validateEndpoint(name, endpoint string) error {
	if strings.TrimSpace(endpoint) != endpoint || endpoint == "" || strings.Contains(endpoint, "://") || strings.ContainsAny(endpoint, "/?#@\\*") {
		return fmt.Errorf("%s must be a host with optional port and no scheme or path", name)
	}
	parsed, err := url.Parse("http://" + endpoint)
	if err != nil || parsed.Host != endpoint || parsed.Hostname() == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil || strings.HasSuffix(parsed.Host, ":") {
		return fmt.Errorf("%s must be a host with optional port and no scheme or path", name)
	}
	if port := parsed.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return fmt.Errorf("%s port must be between 1 and 65535", name)
		}
	}
	return nil
}

func validatePublicBaseURL(value string) error {
	const name = "object_storage.s3.public_base_url"
	if strings.TrimSpace(value) != value || value == "" || strings.ContainsAny(value, "\\*\r\n\t") {
		return fmt.Errorf("%s is required", name)
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Hostname() == "" {
		return fmt.Errorf("%s must be an absolute HTTP or HTTPS URL", name)
	}
	if parsed.User != nil {
		return fmt.Errorf("%s cannot contain user information", name)
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return fmt.Errorf("%s cannot contain a query or fragment", name)
	}
	if parsed.RawPath != "" || strings.Contains(parsed.Path, "//") {
		return fmt.Errorf("%s path must be canonical", name)
	}
	trimmedPath := strings.TrimSuffix(parsed.Path, "/")
	if trimmedPath != "" && path.Clean(parsed.Path) != trimmedPath {
		return fmt.Errorf("%s path must be canonical", name)
	}
	if strings.HasSuffix(parsed.Host, ":") {
		return fmt.Errorf("%s port cannot be empty", name)
	}
	if port := parsed.Port(); port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return fmt.Errorf("%s port must be between 1 and 65535", name)
		}
	}
	return nil
}

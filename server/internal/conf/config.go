// Package conf loads and validates Kirby's single YAML configuration source.
package conf

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DeploymentMode controls whether process-local dependencies are permitted.
type DeploymentMode string

const (
	ModeSingle DeploymentMode = "single"
	ModeMulti  DeploymentMode = "multi"
)

// Duration is a YAML duration such as 15s or 24h.
type Duration struct {
	time.Duration
}

// UnmarshalYAML parses a duration from a YAML string.
//
// Parameters: node is the YAML value being decoded.
// Returns: nil after the receiver has been updated.
// Throws: an error for an unsupported YAML type or malformed value.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("duration must be a string such as 15s")
	}
	value, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", node.Value, err)
	}
	d.Duration = value
	return nil
}

// MarshalYAML writes the duration in Go duration syntax.
//
// Parameters: none.
// Returns: the scalar value and a nil error.
// Throws: none.
func (d Duration) MarshalYAML() (any, error) {
	return d.String(), nil
}

// Secret holds a configured credential.
type Secret struct {
	value string
}

// NewSecret wraps a credential for programmatic configuration.
//
// Parameters: value is the configured credential.
// Returns: the credential wrapper.
// Throws: none.
func NewSecret(value string) Secret {
	return Secret{value: value}
}

// Value returns the credential to the component that must use it.
//
// Parameters: none.
// Returns: the original credential.
// Throws: none.
func (s Secret) Value() string {
	return s.value
}

// Empty reports whether the credential is absent.
//
// Parameters: none.
// Returns: whether the value is blank after trimming whitespace.
// Throws: none.
func (s Secret) Empty() bool {
	return strings.TrimSpace(s.value) == ""
}

// String returns the configured value.
//
// Parameters: none.
// Returns: the original value.
// Throws: none.
func (s Secret) String() string {
	return s.value
}

// GoString returns the quoted configured value.
//
// Parameters: none.
// Returns: the quoted original value.
// Throws: none.
func (s Secret) GoString() string {
	return strconv.Quote(s.value)
}

// MarshalJSON serializes the configured value.
//
// Parameters: none.
// Returns: the JSON string representation.
// Throws: an error if encoding fails.
func (s Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}

// UnmarshalYAML reads a scalar credential.
//
// Parameters: node is the YAML value being decoded.
// Returns: nil after the receiver has been updated.
// Throws: an error for an unsupported YAML type or malformed value.
func (s *Secret) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("secret must be a string")
	}
	s.value = node.Value
	return nil
}

// MarshalYAML serializes the configured value.
//
// Parameters: none.
// Returns: the scalar value and a nil error.
// Throws: none.
func (s Secret) MarshalYAML() (any, error) {
	return s.value, nil
}

// Config is the complete server configuration.
type Config struct {
	Mode          DeploymentMode      `yaml:"mode"`
	HTTP          ListenerConfig      `yaml:"http"`
	GRPC          ListenerConfig      `yaml:"grpc"`
	Database      DatabaseConfig      `yaml:"database"`
	MySQL         DatabaseConfig      `yaml:"mysql"` // Deprecated: use database.
	Cache         CacheConfig         `yaml:"cache"`
	JWT           JWTConfig           `yaml:"jwt"`
	Security      SecurityConfig      `yaml:"security"`
	ObjectStorage ObjectStorageConfig `yaml:"object_storage"`
	Log           LogConfig           `yaml:"log"`
}

// ListenerConfig configures one network listener.
type ListenerConfig struct {
	Address string   `yaml:"address"`
	Timeout Duration `yaml:"timeout"`
}

// DatabaseDriver selects the relational database dialect.
type DatabaseDriver string

const (
	DatabaseDriverMySQL DatabaseDriver = "mysql"
	DatabaseDriverPGSQL DatabaseDriver = "pgsql"
)

// DatabaseConfig configures the shared relational database.
type DatabaseConfig struct {
	Driver          DatabaseDriver `yaml:"driver"`
	DSN             Secret         `yaml:"dsn"`
	MaxOpenConns    int            `yaml:"max_open_conns"`
	MaxIdleConns    int            `yaml:"max_idle_conns"`
	ConnMaxLifetime Duration       `yaml:"conn_max_lifetime"`
}

// MySQLConfig is retained as a source-compatible alias for integrations that
// still construct the legacy configuration type directly.
type MySQLConfig = DatabaseConfig

// CacheConfig selects a process-local or shared cache.
type CacheConfig struct {
	Driver string      `yaml:"driver"`
	Redis  RedisConfig `yaml:"redis"`
}

// RedisConfig configures a shared Redis cache.
type RedisConfig struct {
	Address   string `yaml:"address"`
	Username  string `yaml:"username"`
	Password  Secret `yaml:"password"`
	DB        int    `yaml:"db"`
	KeyPrefix string `yaml:"key_prefix"`
}

// JWTConfig configures token identity and lifetimes; signing keys are stored in the database.
type JWTConfig struct {
	Issuer     string   `yaml:"issuer"`
	AccessTTL  Duration `yaml:"access_ttl"`
	RefreshTTL Duration `yaml:"refresh_ttl"`
}

// SecurityConfig identifies proxies trusted to forward client request metadata.
type SecurityConfig struct {
	TrustedProxies []string `yaml:"trusted_proxies"`
}

// ObjectStorageConfig selects local or S3-compatible object storage.
type ObjectStorageConfig struct {
	Driver string      `yaml:"driver"`
	Local  LocalConfig `yaml:"local"`
	S3     S3Config    `yaml:"s3"`
}

// LocalConfig configures single-instance local object storage.
type LocalConfig struct {
	Directory string `yaml:"directory"`
}

// S3Config configures an S3-compatible shared object store.
type S3Config struct {
	Endpoint      string `yaml:"endpoint"`
	Region        string `yaml:"region"`
	Bucket        string `yaml:"bucket"`
	AccessKey     Secret `yaml:"access_key"`
	SecretKey     Secret `yaml:"secret_key"`
	UseSSL        bool   `yaml:"use_ssl"`
	PublicBaseURL string `yaml:"public_base_url"`
}

// LogConfig controls structured application logging.
type LogConfig struct {
	Level string `yaml:"level"`
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
	database := c.Database
	legacyDatabase := false
	if database.Driver == "" && database.DSN.Empty() && c.MySQL.Driver == "" && c.MySQL.DSN.Empty() {
		return fmt.Errorf("mysql.dsn is required")
	}
	if database.Driver == "" && !c.MySQL.DSN.Empty() {
		database = c.MySQL
		legacyDatabase = true
		if database.Driver == "" {
			database.Driver = DatabaseDriverMySQL
		}
	}
	if database.Driver != DatabaseDriverMySQL && database.Driver != DatabaseDriverPGSQL {
		return fmt.Errorf("database.driver must be %q or %q", DatabaseDriverMySQL, DatabaseDriverPGSQL)
	}
	if database.DSN.Empty() {
		if legacyDatabase {
			return fmt.Errorf("mysql.dsn is required")
		}
		return fmt.Errorf("database.dsn is required")
	}
	if database.MaxOpenConns <= 0 {
		return fmt.Errorf("database.max_open_conns must be greater than zero")
	}
	if database.MaxIdleConns < 0 || database.MaxIdleConns > database.MaxOpenConns {
		return fmt.Errorf("database.max_idle_conns must be between zero and max_open_conns")
	}
	if database.ConnMaxLifetime.Duration <= 0 {
		return fmt.Errorf("database.conn_max_lifetime must be greater than zero")
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
	if listener.Timeout.Duration <= 0 {
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
	if c.AccessTTL.Duration <= 0 {
		return fmt.Errorf("jwt.access_ttl must be greater than zero")
	}
	if c.RefreshTTL.Duration <= c.AccessTTL.Duration {
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
	if c.AccessKey.Empty() || c.SecretKey.Empty() {
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

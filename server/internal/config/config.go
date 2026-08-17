// Package config loads and validates Kirby's single YAML configuration source.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const redactedValue = "[REDACTED]"

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
func (d Duration) MarshalYAML() (any, error) {
	return d.String(), nil
}

// Secret keeps credentials from being exposed by common formatting and logging.
type Secret struct {
	value string
}

// NewSecret wraps a credential for programmatic configuration.
func NewSecret(value string) Secret {
	return Secret{value: value}
}

// Value returns the credential to the component that must use it.
func (s Secret) Value() string {
	return s.value
}

// Empty reports whether the credential is absent.
func (s Secret) Empty() bool {
	return strings.TrimSpace(s.value) == ""
}

// String prevents credentials from leaking through fmt.Stringer.
func (s Secret) String() string {
	return redactedValue
}

// GoString prevents credentials from leaking through %#v formatting.
func (s Secret) GoString() string {
	return redactedValue
}

// LogValue prevents credentials from leaking through slog.
func (s Secret) LogValue() slog.Value {
	return slog.StringValue(redactedValue)
}

// UnmarshalYAML reads a scalar credential.
func (s *Secret) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("secret must be a string")
	}
	s.value = node.Value
	return nil
}

// MarshalYAML never serializes the credential.
func (s Secret) MarshalYAML() (any, error) {
	return redactedValue, nil
}

// Config is the complete server configuration.
type Config struct {
	Mode          DeploymentMode      `yaml:"mode"`
	HTTP          ListenerConfig      `yaml:"http"`
	GRPC          ListenerConfig      `yaml:"grpc"`
	MySQL         MySQLConfig         `yaml:"mysql"`
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

// MySQLConfig configures the shared relational database.
type MySQLConfig struct {
	DSN             Secret   `yaml:"dsn"`
	MaxOpenConns    int      `yaml:"max_open_conns"`
	MaxIdleConns    int      `yaml:"max_idle_conns"`
	ConnMaxLifetime Duration `yaml:"conn_max_lifetime"`
}

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

// JWTConfig configures the signing key ring and token lifetimes.
type JWTConfig struct {
	Issuer     string            `yaml:"issuer"`
	ActiveKID  string            `yaml:"active_kid"`
	AccessTTL  Duration          `yaml:"access_ttl"`
	RefreshTTL Duration          `yaml:"refresh_ttl"`
	Keys       map[string]Secret `yaml:"keys"`
}

// SecurityConfig contains secrets and browser origins shared by every instance.
type SecurityConfig struct {
	APIKeyPepper   Secret   `yaml:"api_key_pepper"`
	AllowedOrigins []string `yaml:"allowed_origins"`
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
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
	Bucket    string `yaml:"bucket"`
	AccessKey Secret `yaml:"access_key"`
	SecretKey Secret `yaml:"secret_key"`
	UseSSL    bool   `yaml:"use_ssl"`
}

// LogConfig controls structured application logging.
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Validate rejects incomplete, ambiguous, or unsafe deployment combinations.
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
	if c.MySQL.DSN.Empty() {
		return fmt.Errorf("mysql.dsn is required")
	}
	if c.MySQL.MaxOpenConns <= 0 {
		return fmt.Errorf("mysql.max_open_conns must be greater than zero")
	}
	if c.MySQL.MaxIdleConns < 0 || c.MySQL.MaxIdleConns > c.MySQL.MaxOpenConns {
		return fmt.Errorf("mysql.max_idle_conns must be between zero and max_open_conns")
	}
	if c.MySQL.ConnMaxLifetime.Duration <= 0 {
		return fmt.Errorf("mysql.conn_max_lifetime must be greater than zero")
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
	if err := c.Log.validate(); err != nil {
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
	if strings.TrimSpace(c.ActiveKID) == "" {
		return fmt.Errorf("jwt.active_kid is required")
	}
	if c.AccessTTL.Duration <= 0 {
		return fmt.Errorf("jwt.access_ttl must be greater than zero")
	}
	if c.RefreshTTL.Duration <= c.AccessTTL.Duration {
		return fmt.Errorf("jwt.refresh_ttl must be greater than access_ttl")
	}
	if len(c.Keys) == 0 {
		return fmt.Errorf("jwt.keys is required")
	}
	key, ok := c.Keys[c.ActiveKID]
	if !ok {
		return fmt.Errorf("jwt.active_kid %q does not exist in jwt.keys", c.ActiveKID)
	}
	if len(key.Value()) < 32 {
		return fmt.Errorf("active JWT signing key must contain at least 32 bytes")
	}
	for kid, value := range c.Keys {
		if strings.TrimSpace(kid) == "" || len(value.Value()) < 32 {
			return fmt.Errorf("each JWT key must have a non-empty id and at least 32 bytes")
		}
	}
	return nil
}

func (c SecurityConfig) validate() error {
	if len(c.APIKeyPepper.Value()) < 32 {
		return fmt.Errorf("security.api_key_pepper must contain at least 32 bytes")
	}
	if len(c.AllowedOrigins) == 0 {
		return fmt.Errorf("security.allowed_origins must contain at least one origin")
	}
	seen := make(map[string]struct{}, len(c.AllowedOrigins))
	for _, origin := range c.AllowedOrigins {
		if err := validateOrigin(origin); err != nil {
			return fmt.Errorf("security.allowed_origins contains invalid origin %q: %w", origin, err)
		}
		if _, exists := seen[origin]; exists {
			return fmt.Errorf("security.allowed_origins contains duplicate origin %q", origin)
		}
		seen[origin] = struct{}{}
	}
	return nil
}

func validateOrigin(origin string) error {
	if strings.TrimSpace(origin) != origin || origin == "" {
		return fmt.Errorf("origin cannot be empty or contain surrounding whitespace")
	}
	if strings.Contains(origin, "*") {
		return fmt.Errorf("wildcards are not allowed")
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("parse origin: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return fmt.Errorf("host is required")
	}
	if parsed.User != nil {
		return fmt.Errorf("user information is not allowed")
	}
	if origin != parsed.Scheme+"://"+parsed.Host || parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return fmt.Errorf("path, query, and fragment are not allowed")
	}
	if strings.HasSuffix(parsed.Host, ":") {
		return fmt.Errorf("port cannot be empty")
	}
	if port := parsed.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}
	}
	return nil
}

func (c ObjectStorageConfig) validate() error {
	switch c.Driver {
	case "local":
		if strings.TrimSpace(c.Local.Directory) == "" {
			return fmt.Errorf("object_storage.local.directory is required")
		}
		return nil
	case "s3":
		if strings.TrimSpace(c.S3.Endpoint) == "" {
			return fmt.Errorf("object_storage.s3.endpoint is required")
		}
		if strings.TrimSpace(c.S3.Bucket) == "" {
			return fmt.Errorf("object_storage.s3.bucket is required")
		}
		if c.S3.AccessKey.Empty() || c.S3.SecretKey.Empty() {
			return fmt.Errorf("object_storage.s3 access_key and secret_key are required")
		}
		return nil
	default:
		return fmt.Errorf("object_storage.driver must be %q or %q", "local", "s3")
	}
}

func (c LogConfig) validate() error {
	switch c.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level must be debug, info, warn, or error")
	}
	switch c.Format {
	case "json", "text":
		return nil
	default:
		return fmt.Errorf("log.format must be %q or %q", "json", "text")
	}
}

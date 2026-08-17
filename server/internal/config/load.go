package config

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/yvvlee/kirby/server/internal/safefile"
)

const ConfigFileEnvironment = "KIRBY_CONFIG_FILE"

// LookupEnv matches os.LookupEnv and allows deterministic source selection tests.
type LookupEnv func(string) (string, bool)

// ResolvePath selects the explicit flag value before KIRBY_CONFIG_FILE.
func ResolvePath(flagPath string, lookupEnv LookupEnv) (string, error) {
	if strings.TrimSpace(flagPath) != "" {
		return filepath.Clean(flagPath), nil
	}
	if lookupEnv == nil {
		return "", fmt.Errorf("configuration path is required via --config or %s", ConfigFileEnvironment)
	}
	value, ok := lookupEnv(ConfigFileEnvironment)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("configuration path is required via --config or %s", ConfigFileEnvironment)
	}
	return filepath.Clean(value), nil
}

// Load resolves and loads the single YAML configuration file.
func Load(flagPath string, lookupEnv LookupEnv) (*Config, error) {
	path, err := ResolvePath(flagPath, lookupEnv)
	if err != nil {
		return nil, err
	}
	return LoadFile(path)
}

// LoadFile strictly decodes one YAML document and validates it.
func LoadFile(path string) (*Config, error) {
	file, err := safefile.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config %q: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("decode config %q: multiple YAML documents are not allowed", path)
		}
		return nil, fmt.Errorf("decode config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config %q: %w", path, err)
	}
	return &cfg, nil
}

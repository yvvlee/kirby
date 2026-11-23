package conf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
)

const ConfigFileEnvironment = "KIRBY_CONFIG_FILE"

// LookupEnv matches os.LookupEnv and allows deterministic source selection tests.
type LookupEnv func(string) (string, bool)

// ResolvePath selects the explicit flag value before KIRBY_CONFIG_FILE.
//
// Parameters: flagPath is the explicit path; lookupEnv reads configuration environment variables.
// Returns: the selected configuration path, or an empty string when no file source is configured.
// Throws: none.
func ResolvePath(flagPath string, lookupEnv LookupEnv) string {
	if strings.TrimSpace(flagPath) != "" {
		return filepath.Clean(flagPath)
	}
	if lookupEnv == nil {
		return ""
	}
	value, ok := lookupEnv(ConfigFileEnvironment)
	if !ok || strings.TrimSpace(value) == "" {
		return ""
	}
	return filepath.Clean(value)
}

// NewConfig loads the explicitly selected local configuration file.
//
// Parameters: flagPath 是 --config 传入的显式路径，为空时回退 KIRBY_CONFIG_FILE 环境变量。
// Returns: 解析并校验后的全局配置，以及用于关闭配置源的 cleanup 函数。
// Throws: 配置源创建、加载、解析或校验失败时返回错误。
func NewConfig(flagPath string) (*Config, func(), error) {
	configPath := ResolvePath(flagPath, os.LookupEnv)
	if configPath == "" {
		return nil, nil, errors.New("configuration file required: set --config or KIRBY_CONFIG_FILE")
	}
	fmt.Printf("Loading config file: %s\n", configPath)
	c := config.New(
		config.WithSource(
			env.NewSource("KIRBY_"),
			file.NewSource(configPath),
		),
	)
	var err error

	if err = c.Load(); err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		if closeErr := c.Close(); closeErr != nil {
			fmt.Printf("config cleanup error: %+v", closeErr)
		}
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()

	var bc Config
	if err = c.Scan(&bc); err != nil {
		return nil, nil, err
	}
	if err = bc.Validate(); err != nil {
		return nil, nil, err
	}

	App = bc.App
	return &bc, cleanup, nil
}

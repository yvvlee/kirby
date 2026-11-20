package conf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	knacos "github.com/go-kratos/kratos/contrib/config/nacos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/spf13/cast"
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

// NewConfig config 初始化：优先使用本地配置文件，否则从 Nacos 配置中心加载。
//
// Parameters: flagPath 是 --config 传入的显式路径，为空时回退 KIRBY_CONFIG_FILE 环境变量。
// Returns: 解析并校验后的全局配置，以及用于关闭配置源的 cleanup 函数。
// Throws: 配置源创建、加载、解析或校验失败时返回错误。
func NewConfig(flagPath string) (*Config, func(), error) {
	var (
		c   config.Config
		err error
	)
	if configPath := ResolvePath(flagPath, os.LookupEnv); configPath != "" {
		fmt.Printf("Loading config file: %s\n", configPath)
		c = config.New(
			config.WithSource(
				env.NewSource("KIRBY_"),
				file.NewSource(configPath),
			),
		)
	} else {
		c, err = createConfigFromNacos()
		if err != nil {
			return nil, nil, err
		}
	}

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

func createConfigFromNacos() (config.Config, error) {
	nacosServerHost := os.Getenv("NACOS_SERVER_HOST")
	nacosServerPort := cast.ToUint64(os.Getenv("NACOS_SERVER_PORT"))
	nacosNamespaceID := os.Getenv("NACOS_NAMESPACE_ID")
	nacosDataID := os.Getenv("NACOS_DATA_ID")
	nacosGroup := os.Getenv("NACOS_GROUP")
	if nacosServerHost == "" {
		return nil, errors.New("请设置配置文件路径(KIRBY_CONFIG_FILE)或配置中心地址(NACOS_SERVER_HOST)")
	}
	configClient, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig: &constant.ClientConfig{
				NamespaceId:         nacosNamespaceID,
				TimeoutMs:           5000,
				NotLoadCacheAtStart: true,
				LogLevel:            "error",
			},
			ServerConfigs: []constant.ServerConfig{
				*constant.NewServerConfig(nacosServerHost, nacosServerPort),
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return config.New(
		config.WithSource(
			env.NewSource("KIRBY_"),
			knacos.NewConfigSource(
				configClient,
				knacos.WithDataID(nacosDataID),
				knacos.WithGroup(nacosGroup),
			),
		),
	), nil
}

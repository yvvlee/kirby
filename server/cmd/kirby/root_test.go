package kirby

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionFlag(t *testing.T) {
	stdout := new(bytes.Buffer)
	command := newRootCommand(stdout, new(bytes.Buffer))
	command.SetArgs([]string{"--version"})

	require.NoError(t, command.Execute())
	require.True(t, strings.HasPrefix(stdout.String(), "kirby "))
}

func TestServeUsesConfigFlag(t *testing.T) {
	stdout := new(bytes.Buffer)
	command := newRootCommand(stdout, new(bytes.Buffer))
	command.SetArgs([]string{"serve", "--help"})
	require.NoError(t, command.Execute())
	require.Contains(t, stdout.String(), "--config")
	require.Contains(t, stdout.String(), "--web-root")
	require.NotContains(t, stdout.String(), "--http-address")
}

func TestCLIHelpUsesLANG(t *testing.T) {
	tests := []struct {
		name     string
		language string
		args     []string
		want     []string
		unwanted []string
	}{
		{
			name:     "root Chinese",
			language: "zh_CN.UTF-8",
			args:     []string{"--help"},
			want: []string{
				"用法：",
				"可用命令：",
				"启动 Kirby 服务端",
				"示例：",
				"kirby create-admin --config",
				"显示版本信息",
			},
			unwanted: []string{"Available Commands:", "Flags:"},
		},
		{
			name:     "serve Chinese",
			language: "zh_CN.UTF-8",
			args:     []string{"serve", "--help"},
			want: []string{
				"启动 Kirby 服务端并提供浏览器应用。",
				"YAML 配置文件路径",
				"--config",
				"--web-root",
			},
		},
		{
			name:     "healthcheck Chinese",
			language: "zh_CN.UTF-8",
			args:     []string{"healthcheck", "--help"},
			want: []string{
				"通过 HTTP 就绪端点和 gRPC 健康服务检查运行状态。",
				"查看 kirby healthcheck 命令的帮助",
				"HTTP 就绪检查端点",
				"--http-endpoint",
				"--grpc-address",
				"--timeout",
			},
		},
		{
			name:     "create admin Chinese",
			language: "zh_CN.UTF-8",
			args:     []string{"create-admin", "--help"},
			want: []string{
				"管理员用户名",
				"管理员昵称",
				"从受保护的文件读取密码",
				"--password-file",
			},
		},
		{
			name:     "English",
			language: "en_US.UTF-8",
			args:     []string{"healthcheck", "--help"},
			want: []string{
				"Usage:",
				"Check readiness through the HTTP endpoint and gRPC health service.",
				"HTTP readiness endpoint",
				"--http-endpoint",
			},
			unwanted: []string{"用法：", "HTTP 就绪检查端点"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LANG", test.language)
			stdout := new(bytes.Buffer)
			command := newRootCommand(stdout, new(bytes.Buffer))
			command.SetArgs(test.args)
			require.NoError(t, command.Execute())
			for _, text := range test.want {
				require.Contains(t, stdout.String(), text)
			}
			for _, text := range test.unwanted {
				require.NotContains(t, stdout.String(), text)
			}
		})
	}
}

func TestHelpSubcommandShowsRequestedTarget(t *testing.T) {
	tests := []struct {
		name     string
		language string
		target   string
		want     string
	}{
		{
			name:     "serve Chinese",
			language: "zh_CN.UTF-8",
			target:   "serve",
			want:     "启动 Kirby 服务端并提供浏览器应用。",
		},
		{
			name:     "create admin Chinese",
			language: "zh_CN.UTF-8",
			target:   "create-admin",
			want:     "在已初始化的数据库中创建管理员或提升现有用户。",
		},
		{
			name:     "serve English",
			language: "en_US.UTF-8",
			target:   "serve",
			want:     "Start Kirby and serve the browser application.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LANG", test.language)
			stdout := new(bytes.Buffer)
			command := newRootCommand(stdout, new(bytes.Buffer))
			command.SetArgs([]string{"help", test.target})
			require.NoError(t, command.Execute())
			require.Contains(t, stdout.String(), test.want)
		})
	}
}

func TestHelpSubcommandUnknownTargetKeepsUsageBehavior(t *testing.T) {
	t.Setenv("LANG", "zh_CN.UTF-8")
	stdout := new(bytes.Buffer)
	command := newRootCommand(stdout, new(bytes.Buffer))
	command.SetArgs([]string{"help", "missing-command"})
	require.NoError(t, command.Execute())
	require.Contains(t, stdout.String(), "Unknown help topic")
	require.Contains(t, stdout.String(), "missing-command")
	require.Contains(t, stdout.String(), "可用命令：")
}

func TestCompletionHelpIsLocalized(t *testing.T) {
	t.Setenv("LANG", "zh_CN.UTF-8")
	stdout := new(bytes.Buffer)
	command := newRootCommand(stdout, new(bytes.Buffer))
	command.SetArgs([]string{"completion", "bash", "--help"})
	require.NoError(t, command.Execute())
	require.Contains(t, stdout.String(), "生成 Kirby 的 bash shell 补全脚本")
	require.Contains(t, stdout.String(), "生成补全内容时省略命令和标志说明")
}

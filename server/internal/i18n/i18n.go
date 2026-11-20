// Package i18n provides request-scoped localization for public API errors.
package i18n

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
)

// Locale identifies one supported public API language.
type Locale string

const (
	LocaleChinese Locale = "zh-CN"
	LocaleEnglish Locale = "en"
)

// Negotiate selects a supported locale from an HTTP Accept-Language value or gRPC metadata.
// Parameters: header contains the client language preferences.
// Returns: the best supported locale, defaulting to Simplified Chinese.
// Throws: none.
func Negotiate(header string) Locale {
	if strings.TrimSpace(header) == "" {
		return LocaleChinese
	}
	preferences := make([]languagePreference, 0)
	for _, item := range strings.Split(header, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), ";", 2)
		language := strings.ToLower(strings.TrimSpace(parts[0]))
		quality := 1.0
		if len(parts) == 2 {
			quality = qualityValue(parts[1])
		}
		if quality == 0 {
			continue
		}
		preferences = append(preferences, languagePreference{
			language: language,
			quality:  quality,
		})
	}
	sort.SliceStable(preferences, func(left, right int) bool {
		return preferences[left].quality > preferences[right].quality
	})
	for _, preference := range preferences {
		switch preference.language {
		case "zh", "zh-cn", "zh-hans":
			return LocaleChinese
		case "en", "en-us", "en-gb":
			return LocaleEnglish
		}
	}
	return LocaleChinese
}

// FromLANG selects a CLI locale from the conventional LANG environment value.
// Parameters: language is the process locale, such as zh_CN.UTF-8 or en_US.UTF-8.
// Returns: Simplified Chinese or English, defaulting to Simplified Chinese.
// Throws: none.
func FromLANG(language string) Locale {
	language = strings.ToLower(strings.TrimSpace(language))
	language = strings.ReplaceAll(language, "_", "-")
	language = strings.SplitN(language, ".", 2)[0]
	if strings.HasPrefix(language, "en") {
		return LocaleEnglish
	}
	return LocaleChinese
}

type languagePreference struct {
	language string
	quality  float64
}

func qualityValue(value string) float64 {
	parts := strings.SplitN(strings.TrimSpace(value), "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) != "q" {
		return 0
	}
	quality, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil || quality < 0 || quality > 1 {
		return 0
	}
	return quality
}

// LocalizeError converts a public error at a transport boundary without exposing internal failures.
// Parameters: err is the service error; locale is the negotiated request language.
// Returns: a translated API error preserving its HTTP code and reason.
// Throws: none; errors are returned as API errors.
func LocalizeError(err error, locale Locale) error {
	if err == nil {
		return nil
	}
	apiErr := kratoserrors.FromError(err)
	if apiErr.Code >= 500 && apiErr.Reason == "" {
		return kratoserrors.New(int(apiErr.Code), apiErr.Reason, translated(locale, "operation failed"))
	}
	message := translateMessage(locale, apiErr.Message)
	localized := kratoserrors.New(int(apiErr.Code), apiErr.Reason, message)
	if apiErr.Metadata != nil {
		localized = localized.WithMetadata(apiErr.Metadata)
	}
	return localized
}

func translateMessage(locale Locale, message string) string {
	if locale == LocaleEnglish {
		return message
	}
	if translatedMessage, ok := chineseMessages[message]; ok {
		return translatedMessage
	}
	if strings.HasPrefix(message, "invalid ") {
		return translateValidation(message)
	}
	if strings.Contains(message, "password must contain 8 to 32 visible ASCII characters") {
		return "密码长度须为 8 到 32 个可见 ASCII 字符，且至少包含字母、数字、符号中的两类"
	}
	return genericMessage(locale, message)
}

// Translate returns a localized catalog message while leaving English diagnostics intact.
// Parameters: locale is the selected language; message is a stable catalog key.
// Returns: the localized message or the original text when no catalog entry exists.
// Throws: none.
func Translate(locale Locale, message string) string {
	return translateMessage(locale, message)
}

func translateValidation(message string) string {
	field, reason, ok := validationParts(message)
	if !ok {
		return "请求参数无效"
	}
	return fmt.Sprintf("请求参数无效：%s %s", field, translateValidationReason(reason))
}

func validationParts(message string) (string, string, bool) {
	colon := strings.Index(message, ": ")
	if colon < 0 {
		return "", "", false
	}
	fieldStart := strings.LastIndex(message[:colon], ".") + 1
	field := message[fieldStart:colon]
	reason := strings.SplitN(message[colon+2:], " | caused by:", 2)[0]
	if field == "" || reason == "" {
		return "", "", false
	}
	return field, reason, true
}

func translateValidationReason(reason string) string {
	switch {
	case strings.Contains(reason, "length must be between"):
		if match := validationLengthRange.FindStringSubmatch(reason); len(match) == 3 {
			return fmt.Sprintf("长度须在 %s 到 %s 个字符之间", match[1], match[2])
		}
		return "长度不符合要求"
	case strings.Contains(reason, "length must be at most"):
		if match := validationLengthMaximum.FindStringSubmatch(reason); len(match) == 2 {
			return fmt.Sprintf("长度不得超过 %s 个字符", match[1])
		}
		return "长度不符合要求"
	case strings.Contains(reason, "must be greater than 0"):
		return "必须大于 0"
	case strings.Contains(reason, "does not match regex pattern"):
		return "格式不正确"
	case strings.Contains(reason, "valid email address"):
		return "须为有效的电子邮箱地址"
	case strings.Contains(reason, "must contain at least"):
		if match := validationMinimumItems.FindStringSubmatch(reason); len(match) == 2 {
			return fmt.Sprintf("至少需要包含 %s 个项目", match[1])
		}
		return "至少需要包含一个项目"
	case strings.Contains(reason, "must be in list"):
		return "值不在允许的范围内"
	case strings.Contains(reason, "must not be in list"):
		return "不能使用未指定值"
	case strings.Contains(reason, "must be inside range"):
		return "数值超出允许范围"
	case strings.Contains(reason, "one of the defined enum values"):
		return "枚举值无效"
	case strings.Contains(reason, "embedded message failed validation"), strings.Contains(reason, "typed-nil"):
		return "嵌套字段校验未通过"
	default:
		return "格式不符合要求"
	}
}

var (
	validationLengthRange   = regexp.MustCompile(`value length must be between ([0-9]+) and ([0-9]+) (?:runes|bytes), inclusive`)
	validationLengthMaximum = regexp.MustCompile(`value length must be at most ([0-9]+) (?:runes|bytes)`)
	validationMinimumItems  = regexp.MustCompile(`value must contain at least ([0-9]+) item`)
)

func genericMessage(locale Locale, message string) string {
	if locale == LocaleEnglish {
		return message
	}
	return "操作失败，请稍后重试"
}

func translated(locale Locale, message string) string {
	if locale == LocaleEnglish {
		return message
	}
	return genericMessage(locale, message)
}

// #nosec G101 -- this catalog contains credential labels and validation messages, never credential values.
var chineseMessages = map[string]string{
	"invalid configuration data":                                         "配置数据无效",
	"operation conflicts with current configuration":                     "操作与当前配置冲突",
	"operation conflicts with current state":                             "操作与当前状态冲突",
	"permission denied":                                                  "没有执行此操作的权限",
	"resource not found":                                                 "请求的资源不存在",
	"invalid request":                                                    "请求参数无效",
	"operation failed":                                                   "操作失败，请稍后重试",
	"authentication failed":                                              "身份验证失败",
	"authentication is unavailable":                                      "身份验证服务暂不可用",
	"password change required":                                           "请先修改密码",
	"API key authentication failed":                                      "API 密钥验证失败",
	"runtime config read failed":                                         "读取运行时配置失败",
	"invalid runtime config request":                                     "运行时配置请求无效",
	"runtime config was not found":                                       "运行时配置不存在",
	"setup status is unavailable":                                        "无法获取初始化状态",
	"HTTP setup request required":                                        "此操作需要通过 HTTP 请求",
	"request origin is not allowed":                                      "请求来源不被允许",
	"administrator setup is already complete":                            "管理员初始化已完成",
	"administrator setup failed":                                         "管理员初始化失败",
	"administrator credentials are required":                             "必须提供管理员账号和密码",
	"administrator credentials are invalid":                              "管理员账号或密码无效",
	"administrator name is invalid":                                      "管理员名称无效",
	"client address is invalid":                                          "客户端地址无效",
	"HTTP authentication request required":                               "此操作需要通过 HTTP 请求",
	"username or email already exists":                                   "用户名或电子邮箱已存在",
	"self registration is disabled":                                      "当前未开放自主注册",
	"too many authentication attempts":                                   "身份验证尝试次数过多，请稍后重试",
	"HTTP request is unavailable":                                        "HTTP 请求不可用",
	"public request address is invalid":                                  "公开请求地址无效",
	"asset request is invalid":                                           "资源请求无效",
	"asset operation is not permitted":                                   "无权执行此资源操作",
	"asset scope was not found":                                          "资源范围不存在",
	"asset upload was already completed":                                 "资源上传已完成",
	"uploaded asset failed integrity validation":                         "上传的资源未通过完整性校验",
	"asset operation failed":                                             "资源操作失败，请稍后重试",
	"method not allowed":                                                 "不支持此请求方法",
	"request rate limit exceeded":                                        "请求频率超限，请稍后重试",
	"missing role":                                                       "缺少角色",
	"duplicate role":                                                     "角色重复",
	"unknown role":                                                       "角色无效",
	"duplicate permission":                                               "权限重复",
	"unknown permission":                                                 "权限无效",
	"owner permissions cannot be restricted":                             "所有者权限不能受限",
	"valid administrator email is required":                              "必须提供有效的管理员电子邮箱",
	"username must contain 1 to 128 printable characters":                "用户名须为 1 到 128 个可打印字符",
	"display name must contain 1 to 128 printable characters":            "显示名称须为 1 到 128 个可打印字符",
	"password input is not a TTY; use --password-file":                   "当前终端不支持密码输入，请使用 --password-file",
	"password confirmation does not match":                               "两次输入的密码不一致",
	"object storage is unavailable":                                      "对象存储暂不可用",
	"unable to read web content":                                         "无法读取网页内容",
	"Bad Request":                                                        "请求无效",
	"Kirby configuration platform server":                                "Kirby 配置平台服务端",
	"Run the Kirby server and its management commands.":                  "运行 Kirby 服务端及其管理命令。",
	"Start the Kirby server":                                             "启动 Kirby 服务端",
	"Start Kirby and serve the browser application.":                     "启动 Kirby 服务端并提供浏览器应用。",
	"path to the YAML configuration file":                                "YAML 配置文件路径",
	"path to the built web application":                                  "已构建的 Web 应用目录",
	"Check the local HTTP and gRPC listeners":                            "检查本机 HTTP 和 gRPC 服务是否就绪",
	"Check readiness through the HTTP endpoint and gRPC health service.": "通过 HTTP 就绪端点和 gRPC 健康服务检查运行状态。",
	"HTTP readiness endpoint":                                            "HTTP 就绪检查端点",
	"gRPC health service address":                                        "gRPC 健康服务地址",
	"total probe timeout":                                                "探测总超时时间",
	"Create or promote a system administrator in an existing database":   "在现有数据库中创建系统管理员或提升现有用户",
	"Create or promote an administrator in an initialized database.":     "在已初始化的数据库中创建管理员或提升现有用户。密码会通过终端交互输入；也可使用 --password-file 指定受保护的文件。",
	"Create or promote an administrator in an initialized database. The password is prompted securely unless --password-file is set.": "在已初始化的数据库中创建管理员或提升现有用户。密码默认通过终端安全输入；也可使用 --password-file 从受保护文件读取。",
	"path to the single Kirby YAML config (or use KIRBY_CONFIG_FILE)":                                                                 "单机模式的 Kirby YAML 配置文件路径（也可使用 KIRBY_CONFIG_FILE）",
	"required administrator email":                             "必填的管理员电子邮箱",
	"administrator username":                                   "管理员用户名",
	"administrator nickname (defaults to username)":            "管理员昵称（默认使用用户名）",
	"read the password from a protected file instead of a TTY": "从受保护的文件读取密码，不使用终端交互",
	"Usage:":                  "用法：",
	"Available Commands:":     "可用命令：",
	"Additional Commands:":    "其他命令：",
	"Flags:":                  "标志：",
	"Global Flags:":           "全局标志：",
	"Examples:":               "示例：",
	"Aliases:":                "别名：",
	"Additional help topics:": "其他帮助主题：",
	"Use \"%s [command] --help\" for more information about a command.": "使用 \"%s [command] --help\" 查看命令的详细帮助。",
	"help [command]":              "help [command]",
	"Help about any command":      "查看任意命令的帮助",
	"Display help for a command.": "显示命令帮助。",
	"Help provides details for any command. Use kirby help [command] to see its full help.": "查看任意命令的详细帮助。运行 kirby help [command] 查看完整帮助。",
	"Kirby commands":                                                        "Kirby 命令",
	"Run kirby serve to start the server.":                                  "运行 kirby serve 启动服务端。",
	"Check both listeners with kirby healthcheck.":                          "运行 kirby healthcheck 检查两个服务端口。",
	"Create an administrator with kirby create-admin.":                      "运行 kirby create-admin 创建系统管理员。",
	"Use --config to select the YAML configuration file.":                   "使用 --config 指定 YAML 配置文件。",
	"For deployments, pass --web-root to the built web application.":        "部署时使用 --web-root 指定已构建的 Web 应用目录。",
	"The default endpoints target the local server.":                        "默认端点指向本机服务。",
	"Create or promote an administrator in an existing database.":           "在现有数据库中创建管理员，或将现有用户提升为管理员。",
	"The password is prompted securely unless --password-file is set.":      "密码默认通过终端安全输入；也可使用 --password-file 从受保护文件读取。",
	"Check the configured HTTP and gRPC listeners.":                         "检查配置的 HTTP 和 gRPC 服务端口。",
	"The command returns a nonzero status if either listener is unhealthy.": "任一服务未就绪时，命令将以非零状态退出。",
	"Generate shell completion scripts for Kirby":                           "生成 Kirby 的 shell 命令补全脚本",
	"Generate a shell completion script for Kirby. Select bash, zsh, fish, or powershell and follow your shell's standard completion setup.": "生成 Kirby 的 shell 命令补全脚本。选择 bash、zsh、fish 或 powershell，并按对应 shell 的常规方式安装或加载。",
	"Omit command and flag descriptions from completion output.":                                                                             "生成补全内容时省略命令和标志说明",
	"Generate the %s shell completion script for Kirby":                                                                                      "生成 Kirby 的 %s shell 补全脚本",
	"Generate a completion script for Kirby's %s shell. Install or source it using your shell's standard completion setup.":                  "生成 Kirby 的 %s shell 补全脚本。请按该 shell 的常规方式安装或加载脚本。",
	"Internal Server Error":           "服务器内部错误",
	"Unauthorized":                    "未授权",
	"Not Found":                       "请求的资源不存在",
	"Method Not Allowed":              "不支持此请求方法",
	"Requested Range Not Satisfiable": "请求范围无效或无法满足",
	"management credentials are not accepted by object storage": "对象存储请求不接受管理凭据",
	"invalid or expired upload ticket":                          "上传凭证无效或已过期",
	"content type does not match upload ticket":                 "请求内容类型与上传凭证不匹配",
	"content length does not match upload ticket":               "请求内容长度与上传凭证不匹配",
}

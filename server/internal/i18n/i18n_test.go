package i18n

import (
	"errors"
	"sync"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/stretchr/testify/require"
)

func TestNegotiateAcceptLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   Locale
	}{
		{
			name:   "default",
			header: "",
			want:   LocaleChinese,
		},
		{
			name:   "Chinese region",
			header: "zh-CN, en-US;q=0.8",
			want:   LocaleChinese,
		},
		{
			name:   "English region",
			header: "en-US,en;q=0.9",
			want:   LocaleEnglish,
		},
		{
			name:   "quality preference",
			header: "zh-CN;q=0.4,en-US;q=0.9",
			want:   LocaleEnglish,
		},
		{
			name:   "unsupported falls back",
			header: "fr-FR",
			want:   LocaleChinese,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, Negotiate(test.header))
		})
	}
}

func TestFromLANG(t *testing.T) {
	require.Equal(t, LocaleChinese, FromLANG("zh_CN.UTF-8"))
	require.Equal(t, LocaleEnglish, FromLANG("en_US.UTF-8"))
	require.Equal(t, LocaleChinese, FromLANG("C.UTF-8"))
}

func TestLocalizeErrorPreservesCodeAndReason(t *testing.T) {
	err := kratoserrors.New(403, "FORBIDDEN", "permission denied")
	localized := kratoserrors.FromError(LocalizeError(err, LocaleChinese))
	require.Equal(t, int32(403), localized.Code)
	require.Equal(t, "FORBIDDEN", localized.Reason)
	require.Equal(t, "没有执行此操作的权限", localized.Message)
}

func TestLocalizeErrorRemovesValidationCause(t *testing.T) {
	err := kratoserrors.New(400, "VALIDATOR", "invalid RegisterRequest.Email: value must be a valid email address | caused by: private@example.com")
	localized := kratoserrors.FromError(LocalizeError(err, LocaleChinese))
	require.Equal(t, "请求参数无效：Email 须为有效的电子邮箱地址", localized.Message)
}

func TestLocalizeValidationRules(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{
			message: "invalid CreateProjectRequest.Id: value must be inside range (0, 100]",
			want:    "请求参数无效：Id 数值超出允许范围",
		},
		{
			message: "invalid Project.Role: value must not be in list [UNSPECIFIED]",
			want:    "请求参数无效：Role 不能使用未指定值",
		},
		{
			message: "invalid UpdateEnumRequest.Values: value must contain at least 1 item(s)",
			want:    "请求参数无效：Values 至少需要包含 1 个项目",
		},
	}
	for _, test := range tests {
		require.Equal(t, test.want, Translate(LocaleChinese, test.message))
	}
}

func TestLocalizeErrorDoesNotExposeUnknownInternalFailure(t *testing.T) {
	localized := kratoserrors.FromError(LocalizeError(errors.New("database password leaked"), LocaleChinese))
	require.Equal(t, int32(500), localized.Code)
	require.NotContains(t, localized.Message, "database password")
}

func TestLocaleTranslationIsConcurrentAndRequestScoped(t *testing.T) {
	const workers = 40
	wait := sync.WaitGroup{}
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			chinese := kratoserrors.FromError(LocalizeError(kratoserrors.New(401, "UNAUTHORIZED", "authentication failed"), LocaleChinese))
			english := kratoserrors.FromError(LocalizeError(kratoserrors.New(401, "UNAUTHORIZED", "authentication failed"), LocaleEnglish))
			require.Equal(t, "身份验证失败", chinese.Message)
			require.Equal(t, "authentication failed", english.Message)
		}()
	}
	wait.Wait()
}

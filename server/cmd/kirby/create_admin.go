package kirby

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yvvlee/kirby/server/internal/auth/password"
	"github.com/yvvlee/kirby/server/internal/auth/passwordpolicy"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/i18n"
	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/util/safefile"
)

const maxPasswordBytes = 32

type createAdminOptions struct {
	email        string
	configPath   string
	username     string
	nickname     string
	passwordFile string
}

func newCreateAdminCommand() *cobra.Command {
	locale := i18n.FromLANG(os.Getenv("LANG"))
	options := new(createAdminOptions)
	command := &cobra.Command{
		Use:     "create-admin",
		Short:   cliText(locale, "Create or promote a system administrator in an existing database"),
		Long:    cliText(locale, "Create or promote an administrator in an initialized database. The password is prompted securely unless --password-file is set."),
		Example: "  kirby create-admin --config ./config.yaml --username admin --email admin@example.com",
		Args:    cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runCreateAdmin(command.Context(), command.OutOrStdout(), command.ErrOrStderr(), options)
		},
	}
	command.Flags().StringVar(&options.configPath, "config", "", cliText(locale, "path to the single Kirby YAML config (or use KIRBY_CONFIG_FILE)"))
	command.Flags().StringVar(&options.email, "email", "", cliText(locale, "required administrator email"))
	command.Flags().StringVar(&options.username, "username", "", cliText(locale, "administrator username"))
	command.Flags().StringVar(&options.nickname, "nickname", "", cliText(locale, "administrator nickname (defaults to username)"))
	command.Flags().StringVar(&options.passwordFile, "password-file", "", cliText(locale, "read the password from a protected file instead of a TTY"))
	return command
}

func runCreateAdmin(ctx context.Context, stdout, stderr io.Writer, options *createAdminOptions) error {
	locale := i18n.FromLANG(os.Getenv("LANG"))
	username := strings.ToLower(strings.TrimSpace(options.username))
	email := strings.ToLower(strings.TrimSpace(options.email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return fmt.Errorf("%s", i18n.Translate(locale, "valid administrator email is required"))
	}
	nickname := strings.TrimSpace(options.nickname)
	if nickname == "" {
		nickname = username
	}
	if err := validateAdminIdentity(username, nickname); err != nil {
		return fmt.Errorf("%s", i18n.Translate(locale, err.Error()))
	}
	cfg, cleanupConfig, err := conf.NewConfig(options.configPath)
	if err != nil {
		return err
	}
	defer cleanupConfig()
	store, closeStore, err := openAdminStorage(ctx, cfg, stderr)
	if err != nil {
		return err
	}
	defer closeStore()
	encoded, err := hashAdminPassword(options.passwordFile, stderr)
	if err != nil {
		localized := i18n.Translate(locale, err.Error())
		if localized != err.Error() {
			return fmt.Errorf("%s", localized)
		}
		return err
	}
	user, created, err := saveAdministrator(ctx, store, &model.User{
		Username:     username,
		Email:        email,
		Nickname:     nickname,
		PasswordHash: encoded,
	})
	if err != nil {
		return err
	}
	action := "updated"
	if created {
		action = "created"
	}
	if locale == i18n.LocaleChinese {
		if created {
			action = "已创建"
		} else {
			action = "已更新"
		}
		_, err = fmt.Fprintf(stdout, "%s系统管理员 %q（编号 %d）\n", action, user.Username, user.ID)
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s system administrator %q (id %d)\n", action, user.Username, user.ID)
	return err
}

func openAdminStorage(ctx context.Context, cfg *conf.Config, stderr io.Writer) (*infra.Storage, func(), error) {
	logger, err := infra.NewLoggerWithWriter(stderr, cfg.Log)
	if err != nil {
		return nil, nil, err
	}
	engine, cleanupEngine, err := infra.NewCoreEngine(ctx, cfg, infra.NewDBLogger(logger))
	if err != nil {
		_ = logger.Close()
		return nil, nil, err
	}
	return &infra.Storage{CoreDB: engine}, func() {
		cleanupEngine()
		if err := logger.Close(); err != nil {
			_, _ = fmt.Fprintf(stderr, "close logger: %v\n", err)
		}
	}, nil
}

func hashAdminPassword(path string, stderr io.Writer) (string, error) {
	plain, err := readAdminPassword(path, stderr)
	if err != nil {
		return "", err
	}
	defer clearBytes(plain)
	if err := validateAdminPassword(plain); err != nil {
		return "", err
	}
	encoded, err := password.NewDefault().Hash(string(plain))
	if err != nil {
		return "", fmt.Errorf("hash administrator password: %w", err)
	}
	return encoded, nil
}

func saveAdministrator(ctx context.Context, store *infra.Storage, input *model.User) (*model.User, bool, error) {
	users := repository.NewUserRepository(store)
	setup := repository.NewSetupRepository(store)
	var user *model.User
	var created bool
	err := store.CoreDB.TX(ctx, func(txctx context.Context) error {
		if _, err := setup.MarkInitialized(txctx); err != nil {
			return err
		}
		var err error
		user, created, err = users.CreateOrPromoteSystemAdmin(txctx, input.Username, input.Nickname, input.PasswordHash, input.Email)
		return err
	})
	return user, created, err
}

func validateAdminIdentity(username, nickname string) error {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`).MatchString(username) || !utf8.ValidString(username) || utf8.RuneCountInString(username) > 128 || strings.IndexFunc(username, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return fmt.Errorf("username must contain 1 to 128 printable characters")
	}
	if nickname == "" || !utf8.ValidString(nickname) || utf8.RuneCountInString(nickname) > 128 || strings.IndexFunc(nickname, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return fmt.Errorf("display name must contain 1 to 128 printable characters")
	}
	return nil
}

func validateAdminPassword(plain []byte) error { return passwordpolicy.Validate(string(plain)) }

func readAdminPassword(path string, stderr io.Writer) ([]byte, error) {
	if path != "" {
		return readPasswordFile(path)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("password input is not a TTY; use --password-file")
	}
	locale := i18n.FromLANG(os.Getenv("LANG"))
	passwordPrompt := "Password: "
	confirmPrompt := "Confirm password: "
	if locale == i18n.LocaleChinese {
		// #nosec G101 -- these are terminal prompt labels, not credential values.
		passwordPrompt = "密码："
		confirmPrompt = "确认密码："
	}
	if _, err := fmt.Fprint(stderr, passwordPrompt); err != nil {
		return nil, fmt.Errorf("write password prompt: %w", err)
	}
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	if _, writeErr := fmt.Fprintln(stderr); writeErr != nil && err == nil {
		err = writeErr
	}
	if err != nil {
		return nil, fmt.Errorf("read password from TTY: %w", err)
	}
	if _, err := fmt.Fprint(stderr, confirmPrompt); err != nil {
		clearBytes(first)
		return nil, fmt.Errorf("write password prompt: %w", err)
	}
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	if _, writeErr := fmt.Fprintln(stderr); writeErr != nil && err == nil {
		err = writeErr
	}
	if err != nil {
		clearBytes(first)
		return nil, fmt.Errorf("read password confirmation from TTY: %w", err)
	}
	defer clearBytes(second)
	if !bytes.Equal(first, second) {
		clearBytes(first)
		return nil, fmt.Errorf("password confirmation does not match")
	}
	return first, nil
}

func readPasswordFile(path string) ([]byte, error) {
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect password file path: %w", err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("password file cannot be a symbolic link")
	}
	file, err := safefile.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open password file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect password file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("password file must be a regular file")
	}
	if !os.SameFile(linkInfo, info) {
		return nil, fmt.Errorf("password file changed while it was being opened")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("password file permissions must not allow group or other access")
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxPasswordBytes+3))
	if err != nil {
		return nil, fmt.Errorf("read password file: %w", err)
	}
	contents = trimOneLineEnding(contents)
	if len(contents) > maxPasswordBytes {
		clearBytes(contents)
		return nil, fmt.Errorf("password file exceeds %d password bytes", maxPasswordBytes)
	}
	return contents, nil
}

func trimOneLineEnding(value []byte) []byte {
	if bytes.HasSuffix(value, []byte("\r\n")) {
		return value[:len(value)-2]
	}
	if bytes.HasSuffix(value, []byte("\n")) {
		return value[:len(value)-1]
	}
	return value
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

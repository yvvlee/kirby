package kirby

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yvvlee/kirby/server/internal/auth/password"
	"github.com/yvvlee/kirby/server/internal/config"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

const maxPasswordBytes = 1024

type createAdminOptions struct {
	configPath   string
	username     string
	displayName  string
	passwordFile string
}

func newCreateAdminCommand() *cobra.Command {
	options := new(createAdminOptions)
	command := &cobra.Command{
		Use:   "create-admin",
		Short: "Create or promote a system administrator in an existing database",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runCreateAdmin(command.Context(), command.OutOrStdout(), command.ErrOrStderr(), options)
		},
	}
	command.Flags().StringVar(&options.configPath, "config", "", "path to the single Kirby YAML config (or use KIRBY_CONFIG_FILE)")
	command.Flags().StringVar(&options.username, "username", "", "administrator username")
	command.Flags().StringVar(&options.displayName, "display-name", "", "administrator display name (defaults to username)")
	command.Flags().StringVar(&options.passwordFile, "password-file", "", "read the password from a protected file instead of a TTY")
	return command
}

func runCreateAdmin(ctx context.Context, stdout, stderr io.Writer, options *createAdminOptions) error {
	if options == nil {
		return fmt.Errorf("create-admin options are nil")
	}
	username := strings.TrimSpace(options.username)
	displayName := strings.TrimSpace(options.displayName)
	if displayName == "" {
		displayName = username
	}
	if err := validateAdminIdentity(username, displayName); err != nil {
		return err
	}

	cfg, err := config.Load(options.configPath, os.LookupEnv)
	if err != nil {
		return err
	}
	engine, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		return err
	}
	defer engine.Close()
	if err := model.ValidateSchema(ctx, engine); err != nil {
		return err
	}
	users, err := repository.NewUserRepository(engine)
	if err != nil {
		return err
	}

	plain, err := readAdminPassword(options.passwordFile, stderr)
	if err != nil {
		return err
	}
	defer clearBytes(plain)
	if err := validateAdminPassword(plain); err != nil {
		return err
	}
	hasher, err := password.NewDefault()
	if err != nil {
		return err
	}
	encoded, err := hasher.Hash(string(plain))
	if err != nil {
		return fmt.Errorf("hash administrator password: %w", err)
	}
	user, created, err := users.CreateOrPromoteSystemAdmin(ctx, username, displayName, encoded)
	if err != nil {
		return err
	}
	action := "updated"
	if created {
		action = "created"
	}
	_, err = fmt.Fprintf(stdout, "%s system administrator %q (id %d)\n", action, user.Username, user.ID)
	return err
}

func validateAdminIdentity(username, displayName string) error {
	if username == "" || !utf8.ValidString(username) || utf8.RuneCountInString(username) > 128 || strings.IndexFunc(username, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return fmt.Errorf("username must contain 1 to 128 printable characters")
	}
	if displayName == "" || !utf8.ValidString(displayName) || utf8.RuneCountInString(displayName) > 128 || strings.IndexFunc(displayName, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return fmt.Errorf("display name must contain 1 to 128 printable characters")
	}
	return nil
}

func validateAdminPassword(plain []byte) error {
	if len(plain) < 12 || len(plain) > maxPasswordBytes {
		return fmt.Errorf("password must contain 12 to %d bytes", maxPasswordBytes)
	}
	if bytes.IndexByte(plain, 0) >= 0 || bytes.IndexByte(plain, '\n') >= 0 || bytes.IndexByte(plain, '\r') >= 0 {
		return fmt.Errorf("password cannot contain NUL or newline characters")
	}
	return nil
}

func readAdminPassword(path string, stderr io.Writer) ([]byte, error) {
	if path != "" {
		return readPasswordFile(path)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("password input is not a TTY; use --password-file")
	}
	if _, err := fmt.Fprint(stderr, "Password: "); err != nil {
		return nil, fmt.Errorf("write password prompt: %w", err)
	}
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	if _, writeErr := fmt.Fprintln(stderr); writeErr != nil && err == nil {
		err = writeErr
	}
	if err != nil {
		return nil, fmt.Errorf("read password from TTY: %w", err)
	}
	if _, err := fmt.Fprint(stderr, "Confirm password: "); err != nil {
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
	file, err := os.Open(path)
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

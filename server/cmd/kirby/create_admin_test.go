package kirby

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateAdminHelpExposesNoPasswordFlag(t *testing.T) {
	stdout := new(bytes.Buffer)
	command := newRootCommand(stdout, new(bytes.Buffer))
	command.SetArgs([]string{"create-admin", "--help"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	help := stdout.String()
	if !strings.Contains(help, "--password-file") {
		t.Fatal("create-admin help omits --password-file")
	}
	for _, line := range strings.Split(help, "\n") {
		if strings.TrimSpace(line) == "--password string" || strings.Contains(line, "--password ") {
			t.Fatalf("plaintext password flag exposed: %q", line)
		}
	}
}

func TestReadPasswordFileAcceptsOneTrailingLineEnding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("a secure password\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plain, err := readPasswordFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "a secure password" {
		t.Fatalf("password = %q", plain)
	}
}

func TestReadPasswordFileRejectsBroadPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("a secure password"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPasswordFile(path); err == nil {
		t.Fatal("broad password-file permissions were accepted")
	}
}

func TestValidateAdminPassword(t *testing.T) {
	if err := validateAdminPassword([]byte("a secure password")); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{[]byte("short"), []byte("line one\nline two"), append(bytes.Repeat([]byte("x"), maxPasswordBytes), 'x')} {
		if err := validateAdminPassword(invalid); err == nil {
			t.Fatalf("invalid password accepted: length=%d", len(invalid))
		}
	}
}

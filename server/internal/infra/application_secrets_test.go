package infra

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestApplicationSecretsLoadWithoutRegeneration(t *testing.T) {
	tests := []struct {
		name    string
		rows    *sqlmock.Rows
		readErr error
		wantErr string
	}{
		{
			name: "persisted identity",
			rows: secretRows().AddRow(1, strings.Repeat("ab", 16), strings.Repeat("cd", 32), strings.Repeat("ef", 32), false),
		},
		{
			name:    "missing identity",
			rows:    secretRows(),
			wantErr: "application secrets are missing",
		},
		{
			name:    "corrupt signing key",
			rows:    secretRows().AddRow(1, strings.Repeat("ab", 16), "broken", strings.Repeat("ef", 32), false),
			wantErr: "stored application secret jwt_signing_key is invalid",
		},
		{
			name:    "corrupt pepper",
			rows:    secretRows().AddRow(1, strings.Repeat("ab", 16), strings.Repeat("cd", 32), "broken", false),
			wantErr: "stored application secret api_key_pepper is invalid",
		},
		{
			name:    "database read failure",
			readErr: errors.New("connection unavailable"),
			wantErr: "load application secrets",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertApplicationSecretsLoad(t, test.rows, test.readErr, test.wantErr)
		})
	}
}

func secretRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "jwt_key_id", "jwt_signing_key", "api_key_pepper", "admin_initialized"})
}

func assertApplicationSecretsLoad(t *testing.T, rows *sqlmock.Rows, readErr error, wantErr string) {
	t.Helper()
	engine, mock := newMockEngine(t)
	query := mock.ExpectQuery(regexp.QuoteMeta("SELECT `id`, `jwt_key_id`, `jwt_signing_key`, `api_key_pepper`, `admin_initialized` FROM `application_secrets` WHERE `id` = ? LIMIT 1")).WithArgs(int64(1))
	expectSecretsRead(query, rows, readErr)
	secrets, err := ProvideApplicationSecrets(context.Background(), &Storage{
		CoreDB: engine,
	})
	if wantErr != "" {
		require.ErrorContains(t, err, wantErr)
		require.Nil(t, secrets)
		require.NoError(t, mock.ExpectationsWereMet())
		return
	}
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("ab", 16), secrets.JWTKeyID)
	require.Equal(t, strings.Repeat("cd", 32), secrets.JWTSigningKey)
	require.Equal(t, strings.Repeat("ef", 32), secrets.APIKeyPepper)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectSecretsRead(query *sqlmock.ExpectedQuery, rows *sqlmock.Rows, readErr error) {
	if readErr != nil {
		query.WillReturnError(readErr)
		return
	}
	query.WillReturnRows(rows)
}

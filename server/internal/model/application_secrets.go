package model

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/yvvlee/lorm"
)

// ApplicationSecrets stores the installation identity separately from business
// configuration so every process uses the same credentials after a restart.
type ApplicationSecrets struct {
	lorm.UnimplementedTable `lorm:"application_secrets"`
	ID                      int64  `lorm:"id,primary_key"`
	JWTKeyID                string `lorm:"jwt_key_id"`
	JWTSigningKey           string `lorm:"jwt_signing_key"`
	APIKeyPepper            string `lorm:"api_key_pepper"`
	AdminInitialized        bool   `lorm:"admin_initialized"`
}

func seedApplicationSecrets(ctx context.Context, connection *sql.Conn, postgres bool) error {
	secrets, err := generateApplicationSecrets()
	if err != nil {
		return err
	}
	statement := "INSERT INTO application_secrets (id, jwt_key_id, jwt_signing_key, api_key_pepper) VALUES (1, ?, ?, ?)"
	if postgres {
		statement = "INSERT INTO application_secrets (id, jwt_key_id, jwt_signing_key, api_key_pepper) VALUES (1, $1, $2, $3)"
	}
	if _, err := connection.ExecContext(ctx, statement, secrets.JWTKeyID, secrets.JWTSigningKey, secrets.APIKeyPepper); err != nil {
		return fmt.Errorf("initialize application secrets: %w", err)
	}
	return nil
}

func generateApplicationSecrets() (*ApplicationSecrets, error) {
	var entropy [80]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return nil, fmt.Errorf("generate application secrets: %w", err)
	}
	return &ApplicationSecrets{
		ID:            1,
		JWTKeyID:      hex.EncodeToString(entropy[:16]),
		JWTSigningKey: hex.EncodeToString(entropy[16:48]),
		APIKeyPepper:  hex.EncodeToString(entropy[48:]),
	}, nil
}

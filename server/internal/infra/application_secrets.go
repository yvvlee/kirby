package infra

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/yvvlee/kirby/server/internal/model"
)

// ProvideApplicationSecrets loads the shared installation identity once before
// the authentication managers are constructed. Missing keys are never replaced
// at startup because doing so would invalidate existing credentials.
//
// Parameters: ctx controls the database read; store has initialized its schema.
// Returns: the persisted installation secrets, or a missing/corrupt data error.
// Throws: none.
func ProvideApplicationSecrets(ctx context.Context, store *Storage) (*model.ApplicationSecrets, error) {
	secrets, err := store.CoreDB.Repository[*model.ApplicationSecrets]().Get(ctx, int64(1))
	if err != nil {
		return nil, fmt.Errorf("load application secrets: %w", err)
	}
	if secrets == nil {
		return nil, fmt.Errorf("application secrets are missing; restore the installation secrets from backup")
	}
	if err := validateApplicationSecrets(secrets); err != nil {
		return nil, err
	}
	return secrets, nil
}

func validateApplicationSecrets(secrets *model.ApplicationSecrets) error {
	fields := []struct {
		name  string
		value string
		size  int
	}{
		{
			name:  "jwt_key_id",
			value: secrets.JWTKeyID,
			size:  16,
		},
		{
			name:  "jwt_signing_key",
			value: secrets.JWTSigningKey,
			size:  32,
		},
		{
			name:  "api_key_pepper",
			value: secrets.APIKeyPepper,
			size:  32,
		},
	}
	for _, field := range fields {
		decoded, err := hex.DecodeString(field.value)
		if err != nil || len(decoded) != field.size {
			return fmt.Errorf("stored application secret %s is invalid", field.name)
		}
	}
	return nil
}

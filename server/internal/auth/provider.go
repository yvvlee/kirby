package auth

import (
	"github.com/google/wire"

	credential "github.com/yvvlee/kirby/server/internal/auth/api_key"
	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/auth/password"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/model"
)

// ProviderSet is authentication providers.
var ProviderSet = wire.NewSet(
	ProvideJWTManager,
	ProvideCredentialManager,
	ProvidePasswordHasher,
)

// ProvideJWTManager uses the persisted signing key shared by every instance.
//
// Parameters: cfg supplies token lifetimes; secrets supplies the installation key.
// Returns: the access-token manager, or an invalid configuration error.
// Throws: none.
func ProvideJWTManager(cfg *conf.Config, secrets *model.ApplicationSecrets) (*authjwt.Manager, error) {
	return authjwt.New(authjwt.Config{
		Issuer:    cfg.JWT.Issuer,
		ActiveKID: secrets.JWTKeyID,
		AccessTTL: cfg.JWT.AccessTTL.Duration,
		Keys: map[string]conf.Secret{
			secrets.JWTKeyID: conf.NewSecret(secrets.JWTSigningKey),
		},
	})
}

// ProvideCredentialManager preserves API key verification across restarts.
//
// Parameters: secrets supplies the persisted installation pepper.
// Returns: the credential manager, or an invalid pepper error.
// Throws: none.
func ProvideCredentialManager(secrets *model.ApplicationSecrets) (*credential.Manager, error) {
	return credential.New(conf.NewSecret(secrets.APIKeyPepper))
}

// ProvidePasswordHasher shares the reviewed password policy with authentication services.
//
// Parameters: none.
// Returns: a default password hasher.
// Throws: none.
func ProvidePasswordHasher() *password.Hasher {
	return password.NewDefault()
}

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	// Make sure no env overrides leak in.
	t.Setenv("APP_ENV", "")
	t.Setenv("SERVER_PORT", "")
	t.Setenv("DATABASE_DRIVER", "")
	t.Setenv("JWT_SECRET", "")
	_ = os.Unsetenv("APP_ENV")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "dev", cfg.App.Env)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "sqlite", cfg.Database.Driver)
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("DATABASE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("JWT_SECRET", "a-very-long-secret-string-here")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "postgres", cfg.Database.Driver)
	assert.Equal(t, "postgres://localhost/x", cfg.Database.URL)
}

func TestInvalidDriver(t *testing.T) {
	t.Setenv("DATABASE_DRIVER", "oracle")
	_, err := Load("")
	assert.Error(t, err)
}

func prodEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "prod")
	t.Setenv("GITHUB_CLIENT_ID", "id")
	t.Setenv("GITHUB_CLIENT_SECRET", "secret")
	t.Setenv("GITHUB_REDIRECT_URI", "https://api.example.com/api/v1/auth/github/callback")
	t.Setenv("QQ_APP_ID", "100123456")
}

func TestProdRejectsDefaultSecrets(t *testing.T) {
	prodEnv(t)
	// Secrets left at their change-me defaults.
	_, err := Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a strong random value")
}

func TestProdRejectsShortSecrets(t *testing.T) {
	prodEnv(t)
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("CREDENTIAL_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("UPDATE_URL_SECRET", "0123456789abcdef0123456789abcdef")
	_, err := Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jwt.secret")
}

func TestProdAcceptsStrongSecrets(t *testing.T) {
	prodEnv(t)
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("CREDENTIAL_SECRET", "fedcba9876543210fedcba9876543210")
	t.Setenv("UPDATE_URL_SECRET", "00112233445566778899aabbccddeeff")
	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "prod", cfg.App.Env)
}

func TestUpdateSecretEnvAliases(t *testing.T) {
	prodEnv(t)
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("CREDENTIAL_SECRET", "fedcba9876543210fedcba9876543210")
	// Legacy fallback name still works.
	t.Setenv("UPDATE_SECRET", "00112233445566778899aabbccddeeff")
	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "00112233445566778899aabbccddeeff", cfg.Update.Secret)
}

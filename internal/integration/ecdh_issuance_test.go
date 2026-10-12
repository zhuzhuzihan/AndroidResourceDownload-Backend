package integration

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"resdownload.com/backend/internal/dto"
	"resdownload.com/backend/internal/pkg/crypto"
)

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func unb64url(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	require.NoError(t, err)
	return b
}

// loginEmailUser runs the console-OTP email flow and returns an access token.
func loginEmailUser(t *testing.T, env *testEnv, email string) string {
	t.Helper()
	_, _ = env.doJSON(t, http.MethodPost, "/api/v1/auth/email/code", map[string]string{"email": email}, "")
	otp := readLatestOTPCode(t, env, email)
	_, body := env.doJSON(t, http.MethodPost, "/api/v1/auth/email/login", map[string]any{
		"email": email, "code": otp,
	}, "")
	require.EqualValues(t, 0, body["code"])
	return body["data"].(map[string]any)["accessToken"].(string)
}

func openEnvelope(t *testing.T, env map[string]any, priv []byte, scope crypto.ECDHScope) []byte {
	t.Helper()
	sPub := unb64url(t, env["serverPublicKey"].(string))
	nonce := unb64url(t, env["nonce"].(string))
	ct := unb64url(t, env["ciphertext"].(string))
	require.Equal(t, crypto.KeyTypeV1, env["keyType"])
	pt, err := crypto.OpenFromServer(priv, sPub, nonce, ct, scope)
	require.NoError(t, err)
	return pt
}

func TestCredentialEncryptedRoundtrip(t *testing.T) {
	env := newTestEnv(t)
	access := loginEmailUser(t, env, "u@qq.com")

	cPriv, cPub, err := crypto.GenerateECDHKeypair()
	require.NoError(t, err)

	code, body := env.doJSON(t, http.MethodPost, "/api/v1/webdav/credential", map[string]any{
		"clientPublicKey": b64url(cPub),
	}, access)
	require.Equal(t, http.StatusOK, code)
	require.EqualValues(t, 0, body["code"])
	data := body["data"].(map[string]any)

	// No plaintext credential fields on the encrypted branch.
	assert.NotContains(t, data, "username")
	assert.NotContains(t, data, "password")
	assert.NotContains(t, data, "baseUrl")
	require.Contains(t, data, "encryptedCredential")

	pt := openEnvelope(t, data["encryptedCredential"].(map[string]any), cPriv, crypto.ECDHScopeCredential)
	var payload dto.CredentialPayload
	require.NoError(t, json.Unmarshal(pt, &payload))
	assert.NotEmpty(t, payload.Username)
	assert.NotEmpty(t, payload.Password)
	assert.NotEmpty(t, payload.BaseURL)
	assert.Equal(t, "READ_ONLY", payload.Permission)
}

func TestCredentialEncryptedRejects(t *testing.T) {
	env := newTestEnv(t)
	access := loginEmailUser(t, env, "u@qq.com")
	_, cPub, err := crypto.GenerateECDHKeypair()
	require.NoError(t, err)

	// Unknown key type.
	code, _ := env.doJSON(t, http.MethodPost, "/api/v1/webdav/credential", map[string]any{
		"clientPublicKey": b64url(cPub), "keyType": "bogus-v9",
	}, access)
	assert.NotEqual(t, http.StatusOK, code)

	// Malformed public key.
	code, body := env.doJSON(t, http.MethodPost, "/api/v1/webdav/credential", map[string]any{
		"clientPublicKey": "!!!not-base64!!!",
	}, access)
	assert.NotEqual(t, http.StatusOK, code)
	require.EqualValues(t, 20003, body["code"])
}

func TestUpdateResolveEncrypted(t *testing.T) {
	env := newTestEnv(t)
	env.seedAppVersion(t, 42, "1.2.0", false)
	access := loginEmailUser(t, env, "u@qq.com")

	_, body := env.doJSON(t, http.MethodGet, "/api/v1/update/info", nil, access)
	enc := body["data"].(map[string]any)["encryptedUrl"].(string)

	cPriv, cPub, err := crypto.GenerateECDHKeypair()
	require.NoError(t, err)

	code, body := env.doJSON(t, http.MethodPost, "/api/v1/update/resolve", map[string]any{
		"encryptedUrl": enc, "clientPublicKey": b64url(cPub),
	}, access)
	require.Equal(t, http.StatusOK, code)
	require.EqualValues(t, 0, body["code"])
	data := body["data"].(map[string]any)
	assert.NotContains(t, data, "url")
	require.Contains(t, data, "encryptedUrl")

	pt := openEnvelope(t, data["encryptedUrl"].(map[string]any), cPriv, crypto.ECDHScopeUpdate)
	var payload dto.UpdateURLPayload
	require.NoError(t, json.Unmarshal(pt, &payload))
	assert.Contains(t, payload.TargetURL, "cdn.example.com")
	assert.EqualValues(t, 42, payload.VersionCode)
}

func TestUpdateResolveReplayRejected(t *testing.T) {
	env := newTestEnv(t)
	env.seedAppVersion(t, 43, "1.2.1", false)
	access := loginEmailUser(t, env, "u@qq.com")

	_, body := env.doJSON(t, http.MethodGet, "/api/v1/update/info", nil, access)
	enc := body["data"].(map[string]any)["encryptedUrl"].(string)

	// Legacy branch: first resolve succeeds, replay is rejected as expired.
	code, _ := env.doJSON(t, http.MethodPost, "/api/v1/update/resolve", map[string]any{
		"encryptedUrl": enc,
	}, access)
	require.Equal(t, http.StatusOK, code)
	code, body = env.doJSON(t, http.MethodPost, "/api/v1/update/resolve", map[string]any{
		"encryptedUrl": enc,
	}, access)
	assert.NotEqual(t, http.StatusOK, code)
	require.EqualValues(t, 30001, body["code"])

	// Encrypted branch: same single-use rule.
	_, body = env.doJSON(t, http.MethodGet, "/api/v1/update/info", nil, access)
	enc2 := body["data"].(map[string]any)["encryptedUrl"].(string)
	_, cPub, err := crypto.GenerateECDHKeypair()
	require.NoError(t, err)
	code, _ = env.doJSON(t, http.MethodPost, "/api/v1/update/resolve", map[string]any{
		"encryptedUrl": enc2, "clientPublicKey": b64url(cPub),
	}, access)
	require.Equal(t, http.StatusOK, code)
	code, body = env.doJSON(t, http.MethodPost, "/api/v1/update/resolve", map[string]any{
		"encryptedUrl": enc2, "clientPublicKey": b64url(cPub),
	}, access)
	assert.NotEqual(t, http.StatusOK, code)
	require.EqualValues(t, 30001, body["code"])
}

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"resdownload.com/backend/internal/model"
)

// loginAdminUser returns an access token for an ADMIN-role user.
func loginAdminUser(t *testing.T, env *testEnv) string {
	t.Helper()
	return loginEmailUser(t, env, "admin@admin.example.com")
}

func TestAdminRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	userAccess := loginEmailUser(t, env, "u@qq.com")

	// No token → 401.
	code, _ := env.doJSON(t, http.MethodGet, "/api/v1/admin/versions", nil, "")
	assert.Equal(t, http.StatusUnauthorized, code)

	// Non-admin → 403.
	code, body := env.doJSON(t, http.MethodGet, "/api/v1/admin/versions", nil, userAccess)
	assert.Equal(t, http.StatusForbidden, code)
	require.EqualValues(t, 10003, body["code"])
}

func TestAdminVersionsFlow(t *testing.T) {
	env := newTestEnv(t)
	admin := loginAdminUser(t, env)

	publish := func(code int64, name string) (int, map[string]any) {
		return env.doJSON(t, http.MethodPost, "/api/v1/admin/versions", map[string]any{
			"versionCode": code, "versionName": name, "targetUrl": "https://cdn.example.com/" + name + ".apk",
		}, admin)
	}

	_, _ = publish(10, "1.0.0")
	code, body := publish(11, "1.1.0")
	require.Equal(t, http.StatusOK, code)
	require.EqualValues(t, 0, body["code"])

	// Non-HTTPS target → 400.
	code, _ = env.doJSON(t, http.MethodPost, "/api/v1/admin/versions", map[string]any{
		"versionCode": 12, "versionName": "1.2.0", "targetUrl": "http://cdn.example.com/1.2.0.apk",
	}, admin)
	assert.Equal(t, http.StatusBadRequest, code)

	// Duplicate code → 409.
	code, _ = publish(11, "1.1.0")
	assert.Equal(t, http.StatusConflict, code)

	// Older code → 400.
	code, _ = publish(5, "0.5.0")
	assert.Equal(t, http.StatusBadRequest, code)

	// List newest-first.
	code, body = env.doJSON(t, http.MethodGet, "/api/v1/admin/versions?limit=10", nil, admin)
	require.Equal(t, http.StatusOK, code)
	items := body["data"].(map[string]any)["items"].([]any)
	require.Len(t, items, 2)
	assert.EqualValues(t, 11, items[0].(map[string]any)["version_code"])
	assert.EqualValues(t, 2, body["data"].(map[string]any)["total"])

	// Delete non-latest → 409.
	code, _ = env.doJSON(t, http.MethodDelete, "/api/v1/admin/versions/10", nil, admin)
	assert.Equal(t, http.StatusConflict, code)

	// Delete missing → 404.
	code, _ = env.doJSON(t, http.MethodDelete, "/api/v1/admin/versions/99", nil, admin)
	assert.Equal(t, http.StatusNotFound, code)

	// Delete latest → 200, previous becomes latest.
	code, _ = env.doJSON(t, http.MethodDelete, "/api/v1/admin/versions/11", nil, admin)
	require.Equal(t, http.StatusOK, code)
	code, body = env.doJSON(t, http.MethodGet, "/api/v1/admin/versions", nil, admin)
	require.Equal(t, http.StatusOK, code)
	items = body["data"].(map[string]any)["items"].([]any)
	require.Len(t, items, 1)
	assert.EqualValues(t, 10, items[0].(map[string]any)["version_code"])
}

func TestAdminStatsAndLogs(t *testing.T) {
	env := newTestEnv(t)
	admin := loginAdminUser(t, env)
	userAccess := loginEmailUser(t, env, "u@qq.com")
	env.seedAppVersion(t, 42, "1.2.0", false)

	// Generate traffic: credential + successful + failed resolves.
	_, _ = env.doJSON(t, http.MethodPost, "/api/v1/webdav/credential", map[string]any{}, userAccess)
	_, body := env.doJSON(t, http.MethodGet, "/api/v1/update/info", nil, userAccess)
	enc := body["data"].(map[string]any)["encryptedUrl"].(string)
	_, _ = env.doJSON(t, http.MethodPost, "/api/v1/update/resolve", map[string]any{"encryptedUrl": enc}, userAccess)
	_, _ = env.doJSON(t, http.MethodPost, "/api/v1/update/resolve", map[string]any{"encryptedUrl": "garbage"}, userAccess)

	// Publish a version so audit logs have an admin action.
	_, _ = env.doJSON(t, http.MethodPost, "/api/v1/admin/versions", map[string]any{
		"versionCode": 50, "versionName": "5.0.0", "targetUrl": "https://cdn.example.com/5.0.0.apk",
	}, admin)

	// Issuance stats.
	code, body := env.doJSON(t, http.MethodGet, "/api/v1/admin/stats/issuance?days=7", nil, admin)
	require.Equal(t, http.StatusOK, code)
	days := body["data"].(map[string]any)["days"].([]any)
	require.NotEmpty(t, days)
	today := days[len(days)-1].(map[string]any)
	assert.EqualValues(t, 1, today["credentialTotal"])
	assert.EqualValues(t, 1, today["resolveTotal"])
	assert.EqualValues(t, 1, today["resolveFailed"])

	// Log browsers.
	code, body = env.doJSON(t, http.MethodGet, "/api/v1/admin/logs/credentials", nil, admin)
	require.Equal(t, http.StatusOK, code)
	assert.EqualValues(t, 1, body["data"].(map[string]any)["total"])

	code, body = env.doJSON(t, http.MethodGet, "/api/v1/admin/logs/updates?resolved=false", nil, admin)
	require.Equal(t, http.StatusOK, code)
	assert.EqualValues(t, 1, body["data"].(map[string]any)["total"])

	code, body = env.doJSON(t, http.MethodGet, "/api/v1/admin/logs/audit?action=admin.version.publish", nil, admin)
	require.Equal(t, http.StatusOK, code)
	assert.EqualValues(t, 1, body["data"].(map[string]any)["total"])
}

func TestAdminSessionAlerts(t *testing.T) {
	env := newTestEnv(t)
	admin := loginAdminUser(t, env)
	userAccess := loginEmailUser(t, env, "u@qq.com")

	// Resolve the user id behind the token.
	_, body := env.doJSON(t, http.MethodGet, "/api/v1/auth/me", nil, userAccess)
	uid := body["data"].(map[string]any)["id"].(string)

	// Same user seen from 3 IPs via audit login rows.
	for _, ip := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		ip := ip
		require.NoError(t, env.repos.AuditLogs.Create(&model.AuditLog{
			ID:        uuid.NewString(),
			UserID:    &uid,
			Action:    "login_success",
			IP:        &ip,
			CreatedAt: time.Now(),
		}))
	}

	code, body := env.doJSON(t, http.MethodGet, "/api/v1/admin/alerts/sessions?windowH=24&minIps=3", nil, admin)
	require.Equal(t, http.StatusOK, code)
	alerts := body["data"].(map[string]any)["alerts"].([]any)
	require.Len(t, alerts, 1)
	assert.Equal(t, uid, alerts[0].(map[string]any)["userId"])
	assert.EqualValues(t, 3, alerts[0].(map[string]any)["ipCount"])

	// Higher threshold → no alert.
	code, body = env.doJSON(t, http.MethodGet, "/api/v1/admin/alerts/sessions?windowH=24&minIps=4", nil, admin)
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, body["data"].(map[string]any)["alerts"])
}

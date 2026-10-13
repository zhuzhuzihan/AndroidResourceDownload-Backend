// In-memory session token. Deliberately not persisted: closing the page
// logs the admin out (see docs/ADMIN_API_V1.md).
String? sessionToken;

/// Refresh token for the current session (revoked on logout).
String? sessionRefreshToken;

/// Called when the backend reports an invalid/expired access token so the
/// UI can return to the login page. Registered by main.dart.
void Function()? onAuthLost;

package dto

// UpdateInfoResponse is the payload returned by GET /api/v1/update/info.
type UpdateInfoResponse struct {
	VersionCode  int64  `json:"versionCode"`
	VersionName  string `json:"versionName"`
	ForceUpdate  bool   `json:"forceUpdate"`
	Changelog    string `json:"changelog"`
	EncryptedURL string `json:"encryptedUrl"`
	ExpiresAt    int64  `json:"expiresAt"`
	Signature    string `json:"signature"`
}

// UpdateResolveRequest is the body for POST /api/v1/update/resolve.
// Clients requesting the ECDH envelope set ClientPublicKey (+ KeyType);
// legacy clients omit both and receive the plaintext URL.
type UpdateResolveRequest struct {
	EncryptedURL    string `json:"encryptedUrl" binding:"required"`
	ClientPublicKey string `json:"clientPublicKey,omitempty"`
	KeyType         string `json:"keyType,omitempty"`
}

// UpdateResolveResponse is the payload returned by POST /api/v1/update/resolve.
type UpdateResolveResponse struct {
	URL       string `json:"url,omitempty"`
	ExpiresIn int    `json:"expiresIn"`

	EncryptedURL *EncryptedEnvelope `json:"encryptedUrl,omitempty"`
}

// UpdateURLPayload is the plaintext sealed inside encryptedUrl.
type UpdateURLPayload struct {
	VersionCode int64  `json:"versionCode"`
	TargetURL   string `json:"targetUrl"`
	ExpiresAt   int64  `json:"exp"`
	JTI         string `json:"jti"`
}

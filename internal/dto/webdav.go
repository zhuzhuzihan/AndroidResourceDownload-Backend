package dto

// CredentialRequest is the body for POST /api/v1/webdav/credential.
// An empty body selects the legacy plaintext response; setting
// ClientPublicKey (+ KeyType) selects the ECDH envelope branch.
type CredentialRequest struct {
	ClientPublicKey string `json:"clientPublicKey,omitempty"`
	KeyType         string `json:"keyType,omitempty"`
}

// CredentialPayload is the plaintext sealed inside encryptedCredential.
type CredentialPayload struct {
	BaseURL    string `json:"baseUrl"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	RootPath   string `json:"rootPath"`
	Permission string `json:"permission"`
	ExpiresAt  int64  `json:"expiresAt"`
	JTI        string `json:"jti"`
}

// CredentialResponse is the /webdav/credential response. MVP clients read the
// plaintext fields over HTTPS; ECDH clients read EncryptedCredential and
// ignore the (omitted) plaintext.
type CredentialResponse struct {
	BaseURL    string `json:"baseUrl,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	RootPath   string `json:"rootPath,omitempty"`
	Permission string `json:"permission"`
	ExpiresAt  int64  `json:"expiresAt"`

	EncryptedCredential *EncryptedEnvelope `json:"encryptedCredential,omitempty"`
}

// EncryptedEnvelope is the wire form of an ECDH-sealed payload. All fields
// are base64url (no padding); see crypto.SealForClient for construction.
type EncryptedEnvelope struct {
	ServerPublicKey string `json:"serverPublicKey"`
	Nonce           string `json:"nonce"`
	Ciphertext      string `json:"ciphertext"`
	KeyType         string `json:"keyType"`
}

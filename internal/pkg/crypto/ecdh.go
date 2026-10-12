// Ephemeral ECDH envelope for client-facing issuance (WebDAV credentials,
// update URLs): X25519 + HKDF-SHA256 + XChaCha20-Poly1305.
//
// Construction (mirrored by the Android client, which implements the same
// steps in Kotlin — keep this comment in sync with it):
//
//	shared = X25519(serverPriv, clientPub) == X25519(clientPriv, serverPub)
//	salt   = serverPub || clientPub            // fixed order, binds transcript
//	info   = "webdavbox-ecdh-v1|" + scope      // scope is "credential"|"update"
//	key    = HKDF-SHA256(salt, shared, info, 32)
//	ct     = XChaCha20-Poly1305(key, random 24-byte nonce, plaintext, aad=keyType)
//
// Both keypairs are ephemeral (per request/response); nothing here is
// persisted. keyType "x25519-xchacha20poly1305-v1" is also the AAD, so an
// envelope cannot be replayed under a future algorithm version.
//
// Reference vectors (for the Android implementation to cross-check):
//   - X25519: BoringSSL vector In=668fb9f7…4014, Base=db5f32b7…a7dd
//     gives 090d85e5…0b4074 (see TestECDHSharedVector).
//   - XChaCha20-Poly1305: draft-irtf-cfrg-xchacha A.3.1, key 8081…9f,
//     nonce 4041…5757, aad 5051…c7, sunscreen plaintext.
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// KeyTypeV1 identifies the v1 ECDH envelope and doubles as its AAD.
const KeyTypeV1 = "x25519-xchacha20poly1305-v1"

// ECDHScope isolates key derivation per issuance concern.
type ECDHScope string

const (
	// ECDHScopeCredential seals WebDAV credential payloads.
	ECDHScopeCredential ECDHScope = "credential"
	// ECDHScopeUpdate seals update-URL payloads.
	ECDHScopeUpdate ECDHScope = "update"
)

// ecdhInfoPrefix prefixes the HKDF info string.
const ecdhInfoPrefix = "webdavbox-ecdh-v1|"

// ErrECDHInvalidKey is returned for malformed (non-32-byte) keys.
var ErrECDHInvalidKey = errors.New("crypto: invalid ECDH key length")

// ErrECDHWeakPeer is returned when the peer public key yields an all-zero
// shared secret (low-order / all-zero point).
var ErrECDHWeakPeer = errors.New("crypto: weak ECDH peer key")

// GenerateECDHKeypair returns a fresh ephemeral X25519 keypair.
func GenerateECDHKeypair() (priv, pub []byte, err error) {
	priv = make([]byte, curve25519.ScalarSize)
	if _, err := io.ReadFull(rand.Reader, priv); err != nil {
		return nil, nil, err
	}
	pub, err = curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}
	return priv, pub, nil
}

// ECDHPublic derives the public key for a private key.
func ECDHPublic(priv []byte) ([]byte, error) {
	if len(priv) != curve25519.ScalarSize {
		return nil, ErrECDHInvalidKey
	}
	return curve25519.X25519(priv, curve25519.Basepoint)
}

// ECDHShared computes the X25519 shared secret, rejecting weak peers.
func ECDHShared(priv, peerPub []byte) ([]byte, error) {
	if len(priv) != curve25519.ScalarSize || len(peerPub) != curve25519.PointSize {
		return nil, ErrECDHInvalidKey
	}
	shared, err := curve25519.X25519(priv, peerPub)
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare(shared, make([]byte, curve25519.PointSize)) == 1 {
		return nil, ErrECDHWeakPeer
	}
	return shared, nil
}

// DeriveECDHKey runs HKDF-SHA256 over the shared secret. Salt order is fixed
// (serverPub || clientPub) so both sides derive the same key.
func DeriveECDHKey(shared, serverPub, clientPub []byte, scope ECDHScope) ([]byte, error) {
	if len(shared) != 32 || len(serverPub) != curve25519.PointSize || len(clientPub) != curve25519.PointSize {
		return nil, ErrECDHInvalidKey
	}
	if scope != ECDHScopeCredential && scope != ECDHScopeUpdate {
		return nil, errors.New("crypto: unknown ECDH scope")
	}
	salt := make([]byte, 0, 64)
	salt = append(salt, serverPub...)
	salt = append(salt, clientPub...)
	r := hkdf.New(sha256.New, shared, salt, []byte(ecdhInfoPrefix+string(scope)))
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, err
	}
	return key, nil
}

// SealForClient encrypts plaintext for the holder of clientPub. It returns
// the server's ephemeral public key plus the nonce and ciphertext, all raw
// bytes for the caller to base64url-encode into the DTO envelope.
func SealForClient(serverPriv, clientPub, plaintext []byte, scope ECDHScope) (serverPub, nonce, ciphertext []byte, err error) {
	shared, err := ECDHShared(serverPriv, clientPub)
	if err != nil {
		return nil, nil, nil, err
	}
	serverPub, err = ECDHPublic(serverPriv)
	if err != nil {
		return nil, nil, nil, err
	}
	key, err := DeriveECDHKey(shared, serverPub, clientPub, scope)
	if err != nil {
		return nil, nil, nil, err
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, nil, nil, err
	}
	nonce = make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, nil, err
	}
	ciphertext = aead.Seal(nil, nonce, plaintext, []byte(KeyTypeV1))
	return serverPub, nonce, ciphertext, nil
}

// OpenFromServer decrypts an envelope sealed by SealForClient.
func OpenFromServer(clientPriv, serverPub, nonce, ciphertext []byte, scope ECDHScope) ([]byte, error) {
	if len(nonce) != chacha20poly1305.NonceSizeX {
		return nil, ErrECDHInvalidKey
	}
	shared, err := ECDHShared(clientPriv, serverPub)
	if err != nil {
		return nil, err
	}
	clientPub, err := ECDHPublic(clientPriv)
	if err != nil {
		return nil, err
	}
	key, err := DeriveECDHKey(shared, serverPub, clientPub, scope)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	pt, err := aead.Open(nil, nonce, ciphertext, []byte(KeyTypeV1))
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return pt, nil
}

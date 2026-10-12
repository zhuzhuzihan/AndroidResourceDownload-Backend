package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"resdownload.com/backend/internal/dto"
	"resdownload.com/backend/internal/model"
	"resdownload.com/backend/internal/pkg/crypto"
	"resdownload.com/backend/internal/repository"
)

// UpdateService prepares and resolves encrypted update URLs.
type UpdateService struct {
	suite  *crypto.Suite
	repo   *repository.AppVersionRepo
	logs   *repository.UpdateLogRepo
	ttl    time.Duration
}

// NewUpdateService constructs an UpdateService.
func NewUpdateService(suite *crypto.Suite, repo *repository.AppVersionRepo, logs *repository.UpdateLogRepo, ttlSeconds int) *UpdateService {
	return &UpdateService{
		suite: suite,
		repo:  repo,
		logs:  logs,
		ttl:   time.Duration(ttlSeconds) * time.Second,
	}
}

// Info returns the latest version info with an encrypted URL.
func (s *UpdateService) Info() (*dto.UpdateInfoResponse, error) {
	v, err := s.repo.Latest()
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, errors.New("update_service: no app version configured")
	}
	now := time.Now()
	expiresAt := now.Add(s.ttl).Unix()
	payload := &dto.UpdateURLPayload{
		VersionCode: v.VersionCode,
		TargetURL:   v.TargetURL,
		ExpiresAt:   expiresAt,
		JTI:         uuid.NewString(),
	}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("update_service: marshal: %w", err)
	}
	enc, err := s.suite.Encrypt(plaintext, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("update_service: encrypt: %w", err)
	}
	changelog := ""
	if v.Changelog != nil {
		changelog = *v.Changelog
	}
	return &dto.UpdateInfoResponse{
		VersionCode:  v.VersionCode,
		VersionName:  v.VersionName,
		ForceUpdate:  v.ForceUpdate,
		Changelog:    changelog,
		EncryptedURL: composite(enc),
		ExpiresAt:    expiresAt,
		Signature:    enc.Signature,
	}, nil
}

// Resolve verifies the encrypted URL, returns the underlying target URL,
// and persists a log entry. The log records metadata only — never the
// plaintext payload.
//
// Each issuance is single-use: resolving the same encrypted URL twice
// reports it as expired on the second attempt.
func (s *UpdateService) Resolve(encStr string, userID, ip, userAgent string) (*dto.UpdateResolveResponse, error) {
	payload, err := s.resolvePayload(encStr)
	if err != nil {
		return nil, err
	}

	resolved := true
	log := &model.UpdateURLLog{
		ID:           uuid.NewString(),
		UserID:       nullableString(userID),
		VersionCode:  &payload.VersionCode,
		EncryptedURL: encStr,
		Resolved:     resolved,
		JTI:          nullableString(payload.JTI),
		IP:           nullableString(ip),
		UserAgent:    nullableString(userAgent),
	}
	if err := recordResolve(s, log, payload.JTI); err != nil {
		return nil, err
	}

	return &dto.UpdateResolveResponse{
		URL:       payload.TargetURL,
		ExpiresIn: int(s.ttl.Seconds()),
	}, nil
}

// ResolveEncrypted verifies the encrypted URL like Resolve, but instead of
// the plaintext target URL it returns a fresh ECDH v1 envelope sealed for
// the holder of the base64url-encoded client X25519 public key. The client
// decrypts it locally; the fresh envelope is therefore not tracked for
// replay (only the redeemed issuance JTI is).
func (s *UpdateService) ResolveEncrypted(encStr, clientPubB64, userID, ip, userAgent string) (*dto.UpdateResolveResponse, error) {
	clientPub, err := base64.RawURLEncoding.DecodeString(clientPubB64)
	if err != nil {
		return nil, fmt.Errorf("update_service: decode client key: %w", err)
	}
	payload, err := s.resolvePayload(encStr)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	fresh := &dto.UpdateURLPayload{
		VersionCode: payload.VersionCode,
		TargetURL:   payload.TargetURL,
		ExpiresAt:   now.Add(s.ttl).Unix(),
		JTI:         uuid.NewString(),
	}
	plaintext, err := json.Marshal(fresh)
	if err != nil {
		return nil, fmt.Errorf("update_service: marshal: %w", err)
	}
	serverPriv, _, err := crypto.GenerateECDHKeypair()
	if err != nil {
		return nil, fmt.Errorf("update_service: keypair: %w", err)
	}
	serverPub, nonce, ct, err := crypto.SealForClient(serverPriv, clientPub, plaintext, crypto.ECDHScopeUpdate)
	if err != nil {
		return nil, err
	}

	resolved := true
	log := &model.UpdateURLLog{
		ID:           uuid.NewString(),
		UserID:       nullableString(userID),
		VersionCode:  &payload.VersionCode,
		EncryptedURL: encStr,
		Resolved:     resolved,
		JTI:          nullableString(payload.JTI),
		KeyType:      ptrStr(crypto.KeyTypeV1),
		IP:           nullableString(ip),
		UserAgent:    nullableString(userAgent),
	}
	if err := recordResolve(s, log, payload.JTI); err != nil {
		return nil, err
	}

	return &dto.UpdateResolveResponse{
		ExpiresIn: int(s.ttl.Seconds()),
		EncryptedURL: &dto.EncryptedEnvelope{
			ServerPublicKey: base64.RawURLEncoding.EncodeToString(serverPub),
			Nonce:           base64.RawURLEncoding.EncodeToString(nonce),
			Ciphertext:      base64.RawURLEncoding.EncodeToString(ct),
			KeyType:         crypto.KeyTypeV1,
		},
	}, nil
}

// recordResolve persists the resolution log. The unique index on jti (not
// just the pre-insert ExistsByJTI check) arbitrates concurrent resolves: if
// the insert loses the race, the issuance was consumed and the caller is
// told it expired. Genuine logging failures stay non-fatal, as before.
func recordResolve(s *UpdateService, log *model.UpdateURLLog, jti string) error {
	if err := s.logs.Create(log); err != nil && jti != "" {
		if used, qerr := s.logs.ExistsByJTI(jti); qerr == nil && used {
			return ErrUpdateURLExpired
		}
	}
	return nil
}

// resolvePayload parses, verifies and decrypts an issuance envelope, then
// enforces single-use via the issuance JTI.
func (s *UpdateService) resolvePayload(encStr string) (*dto.UpdateURLPayload, error) {
	enc, err := parseComposite(encStr)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	plaintext, err := s.suite.Decrypt(enc, now)
	if err != nil {
		if errors.Is(err, crypto.ErrSignatureMismatch) {
			return nil, ErrUpdateURLInvalid
		}
		if errors.Is(err, crypto.ErrDecryptionFailed) {
			return nil, ErrUpdateURLExpired
		}
		return nil, err
	}
	payload := &dto.UpdateURLPayload{}
	if err := json.Unmarshal(plaintext, payload); err != nil {
		return nil, ErrUpdateURLInvalid
	}
	if payload.JTI != "" {
		used, err := s.logs.ExistsByJTI(payload.JTI)
		if err != nil {
			return nil, err
		}
		if used {
			return nil, ErrUpdateURLExpired
		}
	}
	return payload, nil
}

// ErrUpdateURLExpired is exported for handler mapping.
var ErrUpdateURLExpired = errors.New("update_service: encrypted url expired")

// ErrUpdateURLInvalid is exported for handler mapping.
var ErrUpdateURLInvalid = errors.New("update_service: encrypted url invalid")

// composite serialises Encrypted to a single URL-safe string: ct.nonce.exp.sig
// (all base64-url-encoded so the dot is the only separator).
func composite(e *crypto.Encrypted) string {
	return e.Ciphertext + "." + e.Nonce + "." + intToString(e.ExpiresAt) + "." + e.Signature
}

func parseComposite(s string) (*crypto.Encrypted, error) {
	parts := splitDots(s)
	if len(parts) != 4 {
		return nil, ErrUpdateURLInvalid
	}
	exp, err := stringToInt(parts[2])
	if err != nil {
		return nil, ErrUpdateURLInvalid
	}
	return &crypto.Encrypted{
		Ciphertext: parts[0],
		Nonce:      parts[1],
		ExpiresAt:  exp,
		Signature:  parts[3],
	}, nil
}

func splitDots(s string) []string {
	out := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func intToString(v int64) string {
	return fmt.Sprintf("%d", v)
}

func stringToInt(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

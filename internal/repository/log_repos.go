package repository

import (
	"time"

	"gorm.io/gorm"

	"resdownload.com/backend/internal/model"
)

// UpdateLogRepo persists update_url_logs.
type UpdateLogRepo struct{ db *gorm.DB }

func (r *UpdateLogRepo) Create(l *model.UpdateURLLog) error { return r.db.Create(l).Error }

// ExistsByJTI reports whether the given issuance ID was already resolved.
func (r *UpdateLogRepo) ExistsByJTI(jti string) (bool, error) {
	var n int64
	if err := r.db.Model(&model.UpdateURLLog{}).Where("jti = ?", jti).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpdateLogFilter scopes List; nil fields are ignored.
type UpdateLogFilter struct {
	UserID   *string
	Resolved *bool
	Since    *time.Time
	Until    *time.Time
}

// List returns update URL logs newest-first with pagination.
func (r *UpdateLogRepo) List(f UpdateLogFilter, limit, offset int) ([]model.UpdateURLLog, int64, error) {
	q := r.db.Model(&model.UpdateURLLog{})
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Resolved != nil {
		q = q.Where("resolved = ?", *f.Resolved)
	}
	if f.Since != nil {
		q = q.Where("created_at >= ?", *f.Since)
	}
	if f.Until != nil {
		q = q.Where("created_at <= ?", *f.Until)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []model.UpdateURLLog
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// FetchSince returns (created_at, key_type, resolved) rows for aggregation.
// Callers aggregate in Go so the query stays portable across SQLite/PG.
func (r *UpdateLogRepo) FetchSince(since time.Time) ([]model.UpdateURLLog, error) {
	var out []model.UpdateURLLog
	err := r.db.Model(&model.UpdateURLLog{}).
		Select("created_at", "key_type", "resolved").
		Where("created_at >= ?", since).Find(&out).Error
	return out, err
}

// CredentialLogRepo persists webdav_credential_logs.
type CredentialLogRepo struct{ db *gorm.DB }

func (r *CredentialLogRepo) Create(l *model.WebDAVCredentialLog) error { return r.db.Create(l).Error }

// CredentialLogFilter scopes List; nil fields are ignored.
type CredentialLogFilter struct {
	UserID *string
	Since  *time.Time
	Until  *time.Time
}

// List returns credential logs newest-first with pagination.
func (r *CredentialLogRepo) List(f CredentialLogFilter, limit, offset int) ([]model.WebDAVCredentialLog, int64, error) {
	q := r.db.Model(&model.WebDAVCredentialLog{})
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Since != nil {
		q = q.Where("created_at >= ?", *f.Since)
	}
	if f.Until != nil {
		q = q.Where("created_at <= ?", *f.Until)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []model.WebDAVCredentialLog
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// FetchSince returns minimal rows for session/IP aggregation (see LoginHit
// pattern below): callers aggregate in Go for SQLite/PG portability.
func (r *CredentialLogRepo) FetchSince(since time.Time) ([]model.WebDAVCredentialLog, error) {
	var out []model.WebDAVCredentialLog
	err := r.db.Model(&model.WebDAVCredentialLog{}).
		Select("user_id", "ip", "created_at", "key_type").
		Where("created_at >= ?", since).Find(&out).Error
	return out, err
}

// AuditLogRepo persists audit_logs.
type AuditLogRepo struct{ db *gorm.DB }

func (r *AuditLogRepo) Create(l *model.AuditLog) error { return r.db.Create(l).Error }

// AuditLogFilter scopes List; nil/empty fields are ignored.
type AuditLogFilter struct {
	UserID *string
	Action string
	Since  *time.Time
	Until  *time.Time
}

// List returns audit logs newest-first with pagination.
func (r *AuditLogRepo) List(f AuditLogFilter, limit, offset int) ([]model.AuditLog, int64, error) {
	q := r.db.Model(&model.AuditLog{})
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.Since != nil {
		q = q.Where("created_at >= ?", *f.Since)
	}
	if f.Until != nil {
		q = q.Where("created_at <= ?", *f.Until)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []model.AuditLog
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// LoginHit is the minimal projection for session/IP aggregation.
type LoginHit struct {
	UserID    *string
	IP        *string
	CreatedAt time.Time
}

// LoginHitsSince returns successful-login (user_id, ip) pairs in window.
func (r *AuditLogRepo) LoginHitsSince(since time.Time) ([]LoginHit, error) {
	var out []LoginHit
	err := r.db.Model(&model.AuditLog{}).
		Select("user_id", "ip", "created_at").
		Where("action = ? AND created_at >= ?", "login_success", since).Find(&out).Error
	return out, err
}

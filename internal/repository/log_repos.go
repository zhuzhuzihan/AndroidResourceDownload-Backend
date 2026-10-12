package repository

import (
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

// CredentialLogRepo persists webdav_credential_logs.
type CredentialLogRepo struct{ db *gorm.DB }

func (r *CredentialLogRepo) Create(l *model.WebDAVCredentialLog) error { return r.db.Create(l).Error }

// AuditLogRepo persists audit_logs.
type AuditLogRepo struct{ db *gorm.DB }

func (r *AuditLogRepo) Create(l *model.AuditLog) error { return r.db.Create(l).Error }

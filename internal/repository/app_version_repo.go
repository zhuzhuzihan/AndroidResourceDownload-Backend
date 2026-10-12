package repository

import (
	"errors"

	"gorm.io/gorm"

	"resdownload.com/backend/internal/model"
)

// AppVersionRepo persists app_versions.
type AppVersionRepo struct{ db *gorm.DB }

func (r *AppVersionRepo) Create(v *model.AppVersion) error { return r.db.Create(v).Error }

// Latest returns the highest-version-code entry.
func (r *AppVersionRepo) Latest() (*model.AppVersion, error) {
	var v model.AppVersion
	err := r.db.Order("version_code DESC").First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *AppVersionRepo) GetByCode(code int64) (*model.AppVersion, error) {
	var v model.AppVersion
	err := r.db.First(&v, "version_code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// List returns versions newest-first with pagination.
func (r *AppVersionRepo) List(limit, offset int) ([]model.AppVersion, int64, error) {
	var total int64
	if err := r.db.Model(&model.AppVersion{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []model.AppVersion
	if err := r.db.Order("version_code DESC").Limit(limit).Offset(offset).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// DeleteByCode removes the version with the given code.
func (r *AppVersionRepo) DeleteByCode(code int64) error {
	return r.db.Delete(&model.AppVersion{}, "version_code = ?", code).Error
}

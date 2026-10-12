package service

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"resdownload.com/backend/internal/dto"
	"resdownload.com/backend/internal/model"
	"resdownload.com/backend/internal/pkg/crypto"
	"resdownload.com/backend/internal/repository"
)

// AdminService backs the /admin/* endpoints: version publishing, issuance
// stats, log browsing and session alerts. Every mutation writes an audit row
// attributing the acting admin.
type AdminService struct {
	versions *repository.AppVersionRepo
	updates  *repository.UpdateLogRepo
	creds    *repository.CredentialLogRepo
	audits   *repository.AuditLogRepo
}

// NewAdminService constructs an AdminService.
func NewAdminService(
	versions *repository.AppVersionRepo,
	updates *repository.UpdateLogRepo,
	creds *repository.CredentialLogRepo,
	audits *repository.AuditLogRepo,
) *AdminService {
	return &AdminService{versions: versions, updates: updates, creds: creds, audits: audits}
}

// ErrVersionExists is returned when publishing a duplicate version code.
var ErrVersionExists = errors.New("admin_service: version code already exists")

// ErrVersionNotNewer is returned when the code is not newer than latest.
var ErrVersionNotNewer = errors.New("admin_service: version code must be newer than latest")

// ErrVersionNotFound is returned when the code does not exist.
var ErrVersionNotFound = errors.New("admin_service: version not found")

// ErrVersionBadRequest marks input validation failures (400).
var ErrVersionBadRequest = errors.New("admin_service: invalid version input")

// ErrNotLatestVersion is returned when deleting anything but the latest.
var ErrNotLatestVersion = errors.New("admin_service: only the latest version can be deleted")

// ListVersions returns versions newest-first.
func (s *AdminService) ListVersions(limit, offset int) (dto.Page[model.AppVersion], error) {
	items, total, err := s.versions.List(limit, offset)
	if err != nil {
		return dto.Page[model.AppVersion]{}, err
	}
	if items == nil {
		items = []model.AppVersion{}
	}
	return dto.Page[model.AppVersion]{Items: items, Total: total}, nil
}

// PublishVersion validates and inserts a new latest version.
func (s *AdminService) PublishVersion(actorID, ip, userAgent string, req dto.CreateVersionRequest) (*model.AppVersion, error) {
	if req.VersionCode <= 0 {
		return nil, fmt.Errorf("%w: version code must be positive", ErrVersionBadRequest)
	}
	if strings.TrimSpace(req.VersionName) == "" {
		return nil, fmt.Errorf("%w: version name is required", ErrVersionBadRequest)
	}
	target, err := url.Parse(strings.TrimSpace(req.TargetURL))
	if err != nil || target.Scheme != "https" || target.Host == "" {
		return nil, fmt.Errorf("%w: target url must be an absolute HTTPS URL", ErrVersionBadRequest)
	}
	if existing, err := s.versions.GetByCode(req.VersionCode); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrVersionExists
	}
	if latest, err := s.versions.Latest(); err != nil {
		return nil, err
	} else if latest != nil && req.VersionCode <= latest.VersionCode {
		return nil, ErrVersionNotNewer
	}
	changelog := req.Changelog
	v := &model.AppVersion{
		ID:          uuid.NewString(),
		VersionCode: req.VersionCode,
		VersionName: strings.TrimSpace(req.VersionName),
		ForceUpdate: req.ForceUpdate,
		Changelog:   &changelog,
		TargetURL:   target.String(),
		CreatedAt:   time.Now(),
	}
	if err := s.versions.Create(v); err != nil {
		return nil, err
	}
	s.audit(actorID, "admin.version.publish", "app_version", ip, userAgent)
	return v, nil
}

// DeleteVersion removes a version; only the latest can go (rollback).
func (s *AdminService) DeleteVersion(actorID, ip, userAgent string, code int64) error {
	existing, err := s.versions.GetByCode(code)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrVersionNotFound
	}
	latest, err := s.versions.Latest()
	if err != nil {
		return err
	}
	if latest == nil || latest.VersionCode != code {
		return ErrNotLatestVersion
	}
	if err := s.versions.DeleteByCode(code); err != nil {
		return err
	}
	s.audit(actorID, "admin.version.delete", "app_version", ip, userAgent)
	return nil
}

// IssuanceStats aggregates per-UTC-day credential/resolve volume split by
// key type (empty = legacy plaintext, KeyTypeV1 = ECDH).
func (s *AdminService) IssuanceStats(days int) (dto.IssuanceStats, error) {
	if days <= 0 || days > 90 {
		return dto.IssuanceStats{}, errors.New("admin_service: days must be 1..90")
	}
	since := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -(days - 1))
	byDay := map[string]*dto.DayStat{}
	day := func(t time.Time) *dto.DayStat {
		k := t.UTC().Format("2006-01-02")
		d, ok := byDay[k]
		if !ok {
			d = &dto.DayStat{Date: k}
			byDay[k] = d
		}
		return d
	}
	creds, err := s.creds.FetchSince(since)
	if err != nil {
		return dto.IssuanceStats{}, err
	}
	for _, c := range creds {
		d := day(c.CreatedAt)
		d.CredentialTotal++
		if c.KeyType != nil && *c.KeyType == crypto.KeyTypeV1 {
			d.CredentialECDH++
		}
	}
	updates, err := s.updates.FetchSince(since)
	if err != nil {
		return dto.IssuanceStats{}, err
	}
	for _, u := range updates {
		d := day(u.CreatedAt)
		if u.Resolved {
			d.ResolveTotal++
			if u.KeyType != nil && *u.KeyType == crypto.KeyTypeV1 {
				d.ResolveECDH++
			}
		} else {
			d.ResolveFailed++
		}
	}
	out := dto.IssuanceStats{Days: make([]dto.DayStat, 0, len(byDay))}
	for _, d := range byDay {
		if d.CredentialTotal > 0 {
			d.CredentialECDHRate = float64(d.CredentialECDH) / float64(d.CredentialTotal)
		}
		if d.ResolveTotal > 0 {
			d.ResolveECDHRate = float64(d.ResolveECDH) / float64(d.ResolveTotal)
		}
		out.Days = append(out.Days, *d)
	}
	sort.Slice(out.Days, func(i, j int) bool { return out.Days[i].Date < out.Days[j].Date })
	return out, nil
}

// SessionAlerts flags users seen from at least minIPs distinct IPs in the
// window, using login successes and credential issuances as signals.
func (s *AdminService) SessionAlerts(window time.Duration, minIPs int) (dto.SessionAlerts, error) {
	if window <= 0 || window > 30*24*time.Hour {
		return dto.SessionAlerts{}, errors.New("admin_service: window must be within 30d")
	}
	if minIPs < 2 {
		return dto.SessionAlerts{}, errors.New("admin_service: minIPs must be >= 2")
	}
	since := time.Now().Add(-window)
	type hit struct {
		ip   string
		when time.Time
	}
	byUser := map[string][]hit{}
	add := func(userID, ip *string, when time.Time) {
		if userID == nil || *userID == "" || ip == nil || *ip == "" {
			return
		}
		byUser[*userID] = append(byUser[*userID], hit{ip: *ip, when: when})
	}
	logins, err := s.audits.LoginHitsSince(since)
	if err != nil {
		return dto.SessionAlerts{}, err
	}
	for _, l := range logins {
		add(l.UserID, l.IP, l.CreatedAt)
	}
	creds, err := s.creds.FetchSince(since)
	if err != nil {
		return dto.SessionAlerts{}, err
	}
	for _, c := range creds {
		add(c.UserID, c.IP, c.CreatedAt)
	}
	out := dto.SessionAlerts{Alerts: []dto.SessionAlert{}}
	for uid, hits := range byUser {
		ips := map[string]time.Time{}
		for _, h := range hits {
			if last, ok := ips[h.ip]; !ok || h.when.After(last) {
				ips[h.ip] = h.when
			}
		}
		if len(ips) < minIPs {
			continue
		}
		names := make([]string, 0, len(ips))
		var last time.Time
		for ip, when := range ips {
			names = append(names, ip)
			if when.After(last) {
				last = when
			}
		}
		sort.Strings(names)
		out.Alerts = append(out.Alerts, dto.SessionAlert{
			UserID:   uid,
			IPCount:  len(ips),
			IPs:      names,
			LastSeen: last.UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(out.Alerts, func(i, j int) bool { return out.Alerts[i].IPCount > out.Alerts[j].IPCount })
	return out, nil
}

// ListCredentialLogs / ListUpdateLogs / ListAuditLogs are thin paginated
// pass-throughs for the admin log browsers.
func (s *AdminService) ListCredentialLogs(f repository.CredentialLogFilter, limit, offset int) (dto.Page[model.WebDAVCredentialLog], error) {
	items, total, err := s.creds.List(f, limit, offset)
	if err != nil {
		return dto.Page[model.WebDAVCredentialLog]{}, err
	}
	if items == nil {
		items = []model.WebDAVCredentialLog{}
	}
	return dto.Page[model.WebDAVCredentialLog]{Items: items, Total: total}, nil
}

// ListUpdateLogs is a thin paginated pass-through for the admin log browser.
func (s *AdminService) ListUpdateLogs(f repository.UpdateLogFilter, limit, offset int) (dto.Page[model.UpdateURLLog], error) {
	items, total, err := s.updates.List(f, limit, offset)
	if err != nil {
		return dto.Page[model.UpdateURLLog]{}, err
	}
	if items == nil {
		items = []model.UpdateURLLog{}
	}
	return dto.Page[model.UpdateURLLog]{Items: items, Total: total}, nil
}

// ListAuditLogs is a thin paginated pass-through for the admin log browser.
func (s *AdminService) ListAuditLogs(f repository.AuditLogFilter, limit, offset int) (dto.Page[model.AuditLog], error) {
	items, total, err := s.audits.List(f, limit, offset)
	if err != nil {
		return dto.Page[model.AuditLog]{}, err
	}
	if items == nil {
		items = []model.AuditLog{}
	}
	return dto.Page[model.AuditLog]{Items: items, Total: total}, nil
}

// audit records an admin mutation. Failures are non-fatal (the mutation
// already succeeded); they surface via service logs.
func (s *AdminService) audit(actorID, action, resource, ip, userAgent string) {
	_ = s.audits.Create(&model.AuditLog{
		ID:           uuid.NewString(),
		UserID:       nullableString(actorID),
		Action:       action,
		ResourceType: &resource,
		IP:           nullableString(ip),
		UserAgent:    nullableString(userAgent),
		CreatedAt:    time.Now(),
	})
}

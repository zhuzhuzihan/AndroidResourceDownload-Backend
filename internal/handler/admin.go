package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"resdownload.com/backend/internal/dto"
	"resdownload.com/backend/internal/middleware"
	"resdownload.com/backend/internal/pkg/response"
	"resdownload.com/backend/internal/repository"
	"resdownload.com/backend/internal/service"
)

// AdminHandler exposes the /admin/* endpoints. Every route already sits
// behind auth + RequireRole(ADMIN); handlers only parse input and map
// service errors to envelopes.
type AdminHandler struct {
	svc *service.AdminService
	log *slog.Logger
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(svc *service.AdminService, log *slog.Logger) *AdminHandler {
	return &AdminHandler{svc: svc, log: log}
}

// ListVersions handles GET /api/v1/admin/versions.
func (h *AdminHandler) ListVersions(c *gin.Context) {
	limit, offset := pageParams(c)
	page, err := h.svc.ListVersions(limit, offset)
	if err != nil {
		logRequestError(h.log, c, "admin_versions_list", err)
		response.Fail(c, response.ErrInternal)
		return
	}
	response.OK(c, page)
}

// PublishVersion handles POST /api/v1/admin/versions.
func (h *AdminHandler) PublishVersion(c *gin.Context) {
	var req dto.CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, 0, "invalid_request")
		return
	}
	v, err := h.svc.PublishVersion(middleware.UserIDFromContext(c), clientIP(c), userAgent(c), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrVersionExists):
			response.Fail(c, response.New(http.StatusConflict, 0, "version_exists"))
		case errors.Is(err, service.ErrVersionNotNewer):
			writeBadRequest(c, 0, "version_not_newer")
		case errors.Is(err, service.ErrVersionBadRequest):
			writeBadRequest(c, 0, "invalid_version")
		default:
			logRequestError(h.log, c, "admin_version_publish", err)
			response.Fail(c, response.ErrInternal)
		}
		return
	}
	response.OK(c, v)
}

// DeleteVersion handles DELETE /api/v1/admin/versions/:code.
func (h *AdminHandler) DeleteVersion(c *gin.Context) {
	code, err := strconv.ParseInt(c.Param("code"), 10, 64)
	if err != nil || code <= 0 {
		writeBadRequest(c, 0, "invalid_version_code")
		return
	}
	err = h.svc.DeleteVersion(middleware.UserIDFromContext(c), clientIP(c), userAgent(c), code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrVersionNotFound):
			response.Fail(c, response.New(http.StatusNotFound, 0, "version_not_found"))
		case errors.Is(err, service.ErrNotLatestVersion):
			response.Fail(c, response.New(http.StatusConflict, 0, "not_latest_version"))
		default:
			logRequestError(h.log, c, "admin_version_delete", err)
			response.Fail(c, response.ErrInternal)
		}
		return
	}
	response.OK(c, gin.H{"deleted": code})
}

// IssuanceStats handles GET /api/v1/admin/stats/issuance.
func (h *AdminHandler) IssuanceStats(c *gin.Context) {
	days := intParam(c, "days", 7)
	stats, err := h.svc.IssuanceStats(days)
	if err != nil {
		writeBadRequest(c, 0, "invalid_days")
		return
	}
	response.OK(c, stats)
}

// SessionAlerts handles GET /api/v1/admin/alerts/sessions.
func (h *AdminHandler) SessionAlerts(c *gin.Context) {
	windowH := intParam(c, "windowH", 24)
	minIPs := intParam(c, "minIps", 3)
	alerts, err := h.svc.SessionAlerts(time.Duration(windowH)*time.Hour, minIPs)
	if err != nil {
		writeBadRequest(c, 0, "invalid_window")
		return
	}
	response.OK(c, alerts)
}

// CredentialLogs handles GET /api/v1/admin/logs/credentials.
func (h *AdminHandler) CredentialLogs(c *gin.Context) {
	limit, offset := pageParams(c)
	f := repository.CredentialLogFilter{
		UserID: strParam(c, "user_id"),
		Since:  timeParam(c, "from"),
		Until:  timeParam(c, "to"),
	}
	page, err := h.svc.ListCredentialLogs(f, limit, offset)
	if err != nil {
		logRequestError(h.log, c, "admin_logs_credentials", err)
		response.Fail(c, response.ErrInternal)
		return
	}
	response.OK(c, page)
}

// UpdateLogs handles GET /api/v1/admin/logs/updates.
func (h *AdminHandler) UpdateLogs(c *gin.Context) {
	limit, offset := pageParams(c)
	f := repository.UpdateLogFilter{
		UserID:   strParam(c, "user_id"),
		Resolved: boolParam(c, "resolved"),
		Since:    timeParam(c, "from"),
		Until:    timeParam(c, "to"),
	}
	page, err := h.svc.ListUpdateLogs(f, limit, offset)
	if err != nil {
		logRequestError(h.log, c, "admin_logs_updates", err)
		response.Fail(c, response.ErrInternal)
		return
	}
	response.OK(c, page)
}

// AuditLogs handles GET /api/v1/admin/logs/audit.
func (h *AdminHandler) AuditLogs(c *gin.Context) {
	limit, offset := pageParams(c)
	f := repository.AuditLogFilter{
		UserID: strParam(c, "user_id"),
		Action: c.Query("action"),
		Since:  timeParam(c, "from"),
		Until:  timeParam(c, "to"),
	}
	page, err := h.svc.ListAuditLogs(f, limit, offset)
	if err != nil {
		logRequestError(h.log, c, "admin_logs_audit", err)
		response.Fail(c, response.ErrInternal)
		return
	}
	response.OK(c, page)
}

// pageParams parses limit/offset with sane defaults and caps.
func pageParams(c *gin.Context) (limit, offset int) {
	limit = intParam(c, "limit", 50)
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset = intParam(c, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// intParam parses a query int or returns def on absence/error.
func intParam(c *gin.Context, key string, def int) int {
	raw := c.Query(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// strParam returns a pointer for non-empty query values, nil otherwise.
func strParam(c *gin.Context, key string) *string {
	raw := c.Query(key)
	if raw == "" {
		return nil
	}
	return &raw
}

// boolParam parses a query bool; nil when absent or unparsable.
func boolParam(c *gin.Context, key string) *bool {
	raw := c.Query(key)
	if raw == "" {
		return nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return nil
	}
	return &b
}

// timeParam parses an RFC3339 query time; nil when absent or unparsable.
func timeParam(c *gin.Context, key string) *time.Time {
	raw := c.Query(key)
	if raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &t
}

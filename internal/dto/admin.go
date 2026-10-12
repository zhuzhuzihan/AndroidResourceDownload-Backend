package dto

// CreateVersionRequest is the body for POST /api/v1/admin/versions.
type CreateVersionRequest struct {
	VersionCode int64  `json:"versionCode" binding:"required"`
	VersionName string `json:"versionName" binding:"required"`
	ForceUpdate bool   `json:"forceUpdate"`
	Changelog   string `json:"changelog"`
	TargetURL   string `json:"targetUrl" binding:"required"`
}

// Page is the paginated wrapper for admin list responses.
type Page[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

// DayStat aggregates one UTC day of issuance.
type DayStat struct {
	Date             string `json:"date"`
	CredentialTotal  int    `json:"credentialTotal"`
	CredentialECDH   int    `json:"credentialEcdh"`
	ResolveTotal     int    `json:"resolveTotal"`
	ResolveECDH      int    `json:"resolveEcdh"`
	ResolveFailed    int    `json:"resolveFailed"`
	CredentialECDHRate float64 `json:"credentialEcdhRate"`
	ResolveECDHRate    float64 `json:"resolveEcdhRate"`
}

// IssuanceStats is the response for GET /api/v1/admin/stats/issuance.
type IssuanceStats struct {
	Days []DayStat `json:"days"`
}

// SessionAlert flags a user seen from many IPs in the window.
type SessionAlert struct {
	UserID   string   `json:"userId"`
	IPCount  int      `json:"ipCount"`
	IPs      []string `json:"ips"`
	LastSeen string   `json:"lastSeen"`
}

// SessionAlerts is the response for GET /api/v1/admin/alerts/sessions.
type SessionAlerts struct {
	Alerts []SessionAlert `json:"alerts"`
}

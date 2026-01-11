package model

// UserQuotaResponse represents quota information for a single user
type UserQuotaResponse struct {
	Username    string         `json:"username"`
	UsedBytes   int64          `json:"usedBytes"`
	QuotaBytes  int64          `json:"quotaBytes"`  // 0 = unlimited
	QuotaSource string         `json:"quotaSource"` // "user", "domain", or "none"
	Mailboxes   []MailboxUsage `json:"mailboxes"`
}

// MailboxUsage represents storage usage for a single mailbox
type MailboxUsage struct {
	Name         string `json:"name"`
	MessageCount int64  `json:"messageCount"`
	UsedBytes    int64  `json:"usedBytes"`
}

// DomainQuotaResponse represents quota information for a domain
type DomainQuotaResponse struct {
	Domain     string      `json:"domain"`
	UsedBytes  int64       `json:"usedBytes"`
	QuotaBytes int64       `json:"quotaBytes"` // 0 = unlimited
	UserCount  int64       `json:"userCount"`
	Users      []UserUsage `json:"users"`
}

// UserUsage represents storage usage for a user within a domain
type UserUsage struct {
	Username      string `json:"username"`
	UsedBytes     int64  `json:"usedBytes"`
	QuotaOverride *int64 `json:"quotaOverride,omitempty"` // nil if using domain quota
}

// SetQuotaRequest is the request body for setting quota limits
type SetQuotaRequest struct {
	QuotaBytes int64 `json:"quotaBytes" validate:"gte=0"`
}

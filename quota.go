package maddy

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/foxcpp/maddy/internal/rest/model"
	"github.com/foxcpp/maddy/internal/storage/imapsql"
	echo "github.com/labstack/echo/v4"
)

// getUserQuota handles GET /v1/users/:id/quota
func getUserQuota(c echo.Context) error {
	username := c.Param("id")

	storage, ok := imapDb.(*imapsql.Storage)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "storage backend not accessible")
	}
	db := storage.Back.DB

	// Get mailbox breakdown
	mailboxQuery := `
		SELECT mb.name, COUNT(m.msgid), COALESCE(SUM(m.bodylen), 0)
		FROM mboxes mb
		LEFT JOIN msgs m ON m.mboxid = mb.id
		WHERE mb.uid = (SELECT id FROM users WHERE username = $1)
		GROUP BY mb.id, mb.name
		ORDER BY mb.name
	`

	rows, err := db.Query(mailboxQuery, username)
	if err != nil {
		return err
	}
	defer rows.Close()

	var mailboxes []model.MailboxUsage
	var totalUsed int64

	for rows.Next() {
		var mu model.MailboxUsage
		if err := rows.Scan(&mu.Name, &mu.MessageCount, &mu.UsedBytes); err != nil {
			return err
		}
		totalUsed += mu.UsedBytes
		mailboxes = append(mailboxes, mu)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// If no mailboxes found, check if user exists
	if len(mailboxes) == 0 {
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", username).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return c.NoContent(http.StatusNotFound)
		}
	}

	// Get quota info (user override or domain quota)
	domain := extractDomain(username)
	var userQuota, domainQuota sql.NullInt64

	err = db.QueryRow("SELECT quota_bytes FROM user_quotas WHERE username = $1", username).Scan(&userQuota)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	err = db.QueryRow("SELECT quota_bytes FROM domain_quotas WHERE domain = $1", domain).Scan(&domainQuota)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Determine effective quota and source
	var quotaBytes int64
	var quotaSource string

	if userQuota.Valid && userQuota.Int64 > 0 {
		quotaBytes = userQuota.Int64
		quotaSource = "user"
	} else if domainQuota.Valid && domainQuota.Int64 > 0 {
		quotaBytes = domainQuota.Int64
		quotaSource = "domain"
	} else {
		quotaBytes = 0
		quotaSource = "none"
	}

	response := model.UserQuotaResponse{
		Username:    username,
		UsedBytes:   totalUsed,
		QuotaBytes:  quotaBytes,
		QuotaSource: quotaSource,
		Mailboxes:   mailboxes,
	}

	return c.JSON(http.StatusOK, response)
}

// setUserQuota handles PUT /v1/users/:id/quota
func setUserQuota(c echo.Context) error {
	username := c.Param("id")

	var req model.SetQuotaRequest
	if err := c.Bind(&req); err != nil {
		return err
	}

	storage, ok := imapDb.(*imapsql.Storage)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "storage backend not accessible")
	}
	db := storage.Back.DB

	// Check if user exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", username).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return c.NoContent(http.StatusNotFound)
	}

	// Upsert user quota override
	_, err = db.Exec(`
		INSERT INTO user_quotas (username, quota_bytes, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (username) DO UPDATE SET
			quota_bytes = EXCLUDED.quota_bytes,
			updated_at = CURRENT_TIMESTAMP
	`, username, req.QuotaBytes)

	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

// getDomainQuota handles GET /v1/domains/:domain/quota
func getDomainQuota(c echo.Context) error {
	domain := c.Param("domain")

	storage, ok := imapDb.(*imapsql.Storage)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "storage backend not accessible")
	}
	db := storage.Back.DB

	// Get domain quota setting
	var quotaBytes int64
	err := db.QueryRow("SELECT quota_bytes FROM domain_quotas WHERE domain = $1", domain).Scan(&quotaBytes)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Get per-user breakdown
	userQuery := `
		SELECT u.username, COALESCE(SUM(m.bodylen), 0), uq.quota_bytes
		FROM users u
		LEFT JOIN mboxes mb ON mb.uid = u.id
		LEFT JOIN msgs m ON m.mboxid = mb.id
		LEFT JOIN user_quotas uq ON uq.username = u.username
		WHERE u.username LIKE '%@' || $1
		GROUP BY u.id, u.username, uq.quota_bytes
		ORDER BY u.username
	`

	rows, err := db.Query(userQuery, domain)
	if err != nil {
		return err
	}
	defer rows.Close()

	var users []model.UserUsage
	var totalUsed int64

	for rows.Next() {
		var username string
		var usedBytes int64
		var quotaOverride sql.NullInt64

		if err := rows.Scan(&username, &usedBytes, &quotaOverride); err != nil {
			return err
		}

		user := model.UserUsage{
			Username:  username,
			UsedBytes: usedBytes,
		}
		if quotaOverride.Valid && quotaOverride.Int64 > 0 {
			user.QuotaOverride = &quotaOverride.Int64
		}

		totalUsed += usedBytes
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	response := model.DomainQuotaResponse{
		Domain:     domain,
		UsedBytes:  totalUsed,
		QuotaBytes: quotaBytes,
		UserCount:  int64(len(users)),
		Users:      users,
	}

	return c.JSON(http.StatusOK, response)
}

// setDomainQuota handles PUT /v1/domains/:domain/quota
func setDomainQuota(c echo.Context) error {
	domain := c.Param("domain")

	var req model.SetQuotaRequest
	if err := c.Bind(&req); err != nil {
		return err
	}

	storage, ok := imapDb.(*imapsql.Storage)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "storage backend not accessible")
	}
	db := storage.Back.DB

	// Upsert domain quota
	_, err := db.Exec(`
		INSERT INTO domain_quotas (domain, quota_bytes, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (domain) DO UPDATE SET
			quota_bytes = EXCLUDED.quota_bytes,
			updated_at = CURRENT_TIMESTAMP
	`, domain, req.QuotaBytes)

	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

// extractDomain extracts the domain part from an email address
func extractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

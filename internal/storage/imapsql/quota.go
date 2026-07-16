/*
Maddy Mail Server - Composable all-in-one email server.
Copyright 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package imapsql

import (
	"database/sql"

	"github.com/foxcpp/maddy/framework/exterrors"
)

// CheckQuota verifies if the user has enough quota for a new message.
// additionalBytes is the size of the incoming message (0 for pre-check).
// Returns an SMTP error if quota is exceeded, nil otherwise.
func (store *Storage) CheckQuota(username string, additionalBytes int64) error {
	db := store.Back.DB

	// Query current usage and effective quota
	// Effective quota: user_quotas > domain_quotas > unlimited (0)
	var usedBytes int64
	var effectiveQuota int64

	err := db.QueryRow(`
		SELECT
			COALESCE(SUM(m.bodylen), 0),
			COALESCE(
				(SELECT quota_bytes FROM user_quotas WHERE username = u.username),
				(SELECT quota_bytes FROM domain_quotas
				 WHERE domain = SUBSTRING(u.username FROM POSITION('@' IN u.username) + 1)),
				0
			)
		FROM users u
		LEFT JOIN mboxes mb ON mb.uid = u.id
		LEFT JOIN msgs m ON m.mboxid = mb.id
		WHERE u.username = $1
		GROUP BY u.id, u.username
	`, username).Scan(&usedBytes, &effectiveQuota)

	if err == sql.ErrNoRows {
		// User doesn't exist yet, let other checks handle it
		return nil
	}
	if err != nil {
		store.Log.Error("quota check failed", err, "username", username)
		return err
	}

	// 0 means unlimited
	if effectiveQuota == 0 {
		return nil
	}

	if usedBytes+additionalBytes > effectiveQuota {
		return &exterrors.SMTPError{
			Code:         552,
			EnhancedCode: exterrors.EnhancedCode{5, 2, 2},
			Message:      "Mailbox quota exceeded",
			TargetName:   "imapsql",
		}
	}

	return nil
}

package accountstore

import (
	"context"
	"database/sql"
	"fmt"
)

const (
	SyncStateServer = "server"
	SyncStateMine   = "mine"
	SyncStateOther  = "other"
)

type SyncAccount struct {
	UserID int
	Name   string
	State  string
}

func (accounts *Accounts) SyncRoster(deviceID string) ([]SyncAccount, error) {
	database, err := accounts.storage.OpenRead()
	if err != nil {
		return nil, err
	}
	rows, err := database.QueryContext(context.Background(), `
		SELECT a.user_id,
		       COALESCE(json_extract(CAST(p.payload_json AS TEXT), '$.user.name'), ''),
		       h.holder_id,
		       h.frozen
		FROM cn_local_account a
		LEFT JOIN cn_account_projection p ON p.user_id = a.user_id
		LEFT JOIN cn_account_holder h ON h.user_id = a.user_id
		WHERE a.user_id >= ? AND a.user_id < ?
		ORDER BY a.user_id`, PrimaryUserID, SystemPartnerUserIDBase)
	if err != nil {
		return nil, fmt.Errorf("list CN sync accounts: %w", err)
	}
	defer rows.Close()
	var out []SyncAccount
	for rows.Next() {
		var row SyncAccount
		var holder sql.NullString
		var frozen sql.NullInt64
		if err := rows.Scan(&row.UserID, &row.Name, &holder, &frozen); err != nil {
			return nil, fmt.Errorf("list CN sync accounts: %w", err)
		}
		row.State = syncRosterState(deviceID, holder.String, holder.Valid, frozen.Int64 == 1)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list CN sync accounts: %w", err)
	}
	if out == nil {
		out = []SyncAccount{}
	}
	return out, nil
}

func syncRosterState(deviceID, holderID string, hasHolder, frozen bool) string {
	if !hasHolder || (holderID == "s" && !frozen) {
		return SyncStateServer
	}
	if holderID == deviceID && !frozen {
		return SyncStateMine
	}
	return SyncStateOther
}

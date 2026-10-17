package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (accounts *Accounts) OpenStorage() (*sql.DB, error) {
	return accounts.storage.Open()
}

func (accounts *Accounts) FindLogin(loginUUID string) (int, bool, error) {
	loginUUID = strings.ToLower(strings.TrimSpace(loginUUID))
	if !validLoginUUID(loginUUID) {
		return 0, false, nil
	}
	database, err := accounts.storage.OpenRead()
	if err != nil {
		return 0, false, err
	}
	var userID int
	err = database.QueryRowContext(context.Background(),
		`SELECT user_id FROM cn_local_account WHERE login_uuid = ?`, loginUUID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return userID, true, nil
}

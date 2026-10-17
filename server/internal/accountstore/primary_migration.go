package accountstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

func (storage *Database) MigratePrimarySnapshot() error {
	if err := storage.EnsureSchema(); err != nil {
		return err
	}
	database, err := storage.Open()
	if err != nil {
		return err
	}
	transaction, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var revision int
	var updated string
	var payload []byte
	var digestText string
	err = transaction.QueryRow(`SELECT revision, updated_utc, payload_json, payload_sha256
		FROM cn_save_snapshot WHERE singleton = 1`).Scan(&revision, &updated, &payload, &digestText)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read primary CN snapshot: %w", err)
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != digestText {
		return errors.New("primary CN account snapshot digest mismatch")
	}
	if _, err := transaction.Exec(`INSERT INTO cn_account_snapshot
		(user_id, schema_version, revision, updated_utc, payload_json, payload_sha256)
		VALUES (?, 3, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
		 schema_version = excluded.schema_version,
		 revision = excluded.revision,
		 updated_utc = excluded.updated_utc,
		 payload_json = excluded.payload_json,
		 payload_sha256 = excluded.payload_sha256`,
		PrimaryUserID, revision, updated, payload, digestText); err != nil {
		return fmt.Errorf("copy primary CN snapshot: %w", err)
	}
	return transaction.Commit()
}

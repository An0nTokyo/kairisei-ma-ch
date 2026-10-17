package accountstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"kairisei.local/server/internal/gamestate"
)

type AccountTransfer struct {
	UserID         int
	Revision       int
	Metadata       []byte
	MetadataDigest string
	MasterDigest   string
	Account        gamestate.State
}

func (accounts *Accounts) ExportAccount(userID int) (AccountTransfer, error) {
	state, err := accounts.LoadPersistentState(userID)
	if err != nil {
		return AccountTransfer{}, err
	}
	database, err := accounts.storage.OpenRead()
	if err != nil {
		return AccountTransfer{}, err
	}
	var revision int
	var metadata []byte
	var digestText string
	if err := database.QueryRowContext(context.Background(),
		`SELECT revision, payload_json, payload_sha256 FROM cn_account_snapshot WHERE user_id = ?`,
		userID,
	).Scan(&revision, &metadata, &digestText); err != nil {
		return AccountTransfer{}, fmt.Errorf("export CN account: %w", err)
	}
	sum := sha256.Sum256(metadata)
	if hex.EncodeToString(sum[:]) != digestText {
		return AccountTransfer{}, errors.New("CN account snapshot digest mismatch")
	}
	return AccountTransfer{
		UserID:         userID,
		Revision:       revision,
		Metadata:       metadata,
		MetadataDigest: digestText,
		Account:        state,
	}, nil
}

func (accounts *Accounts) ImportAccount(transfer AccountTransfer) error {
	if transfer.UserID < PrimaryUserID || transfer.Account.User.UserID != transfer.UserID {
		return errors.New("CN account transfer has a mismatched user")
	}
	sum := sha256.Sum256(transfer.Metadata)
	if hex.EncodeToString(sum[:]) != transfer.MetadataDigest {
		return errors.New("CN account transfer digest mismatch")
	}
	database, err := accounts.storage.Open()
	if err != nil {
		return err
	}
	transaction, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin CN account transfer: %w", err)
	}
	defer transaction.Rollback()
	if err := writeAccountRows(transaction, transfer.Account); err != nil {
		return err
	}
	var revision int
	var updated string
	var digestText string
	if err := transaction.QueryRowContext(context.Background(),
		`SELECT revision, updated_utc, payload_sha256 FROM cn_account_snapshot WHERE user_id = ?`,
		transfer.UserID,
	).Scan(&revision, &updated, &digestText); err != nil {
		return fmt.Errorf("read CN account transfer target: %w", err)
	}
	if digestText != transfer.MetadataDigest {
		updated = time.Now().UTC().Format(time.RFC3339Nano)
		result, err := transaction.ExecContext(context.Background(),
			`UPDATE cn_account_snapshot
			 SET schema_version = ?, revision = revision + 1, updated_utc = ?,
			     payload_json = ?, payload_sha256 = ?
			 WHERE user_id = ?`,
			saveSnapshotSchemaVersion, updated, transfer.Metadata, transfer.MetadataDigest, transfer.UserID,
		)
		if err != nil {
			return fmt.Errorf("write CN account transfer: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil || rows != 1 {
			return errors.New("CN account transfer update did not affect exactly one row")
		}
		if err := transaction.QueryRowContext(context.Background(),
			`SELECT revision FROM cn_account_snapshot WHERE user_id = ?`, transfer.UserID,
		).Scan(&revision); err != nil {
			return err
		}
	}
	if err := upsertAccountProjection(transaction, transfer.UserID, revision, updated, transfer.Account); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit CN account transfer: %w", err)
	}
	return nil
}

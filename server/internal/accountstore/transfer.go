package accountstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"kairisei.local/server/internal/gamestate"
)

type AccountCredential struct {
	Username   string
	Salt       []byte
	Hash       []byte
	Iterations int
}

type AccountTransfer struct {
	UserID         int
	Revision       int
	Metadata       []byte
	MetadataDigest string
	MasterDigest   string
	Account        gamestate.State
	Credential     AccountCredential
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
	credential, err := readAccountCredential(database, userID)
	if err != nil {
		return AccountTransfer{}, err
	}
	// Templates come from the card master already on both sides. Keeping them
	// in the transfer repeats that master and exceeds the request capture limit.
	state.CardTemplates = nil
	return AccountTransfer{
		UserID:         userID,
		Revision:       revision,
		Metadata:       metadata,
		MetadataDigest: digestText,
		Account:        state,
		Credential:     credential,
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
	if err := writeTransferredCredential(transaction, transfer); err != nil {
		return err
	}
	var revision int
	var updated string
	var digestText string
	err = transaction.QueryRowContext(context.Background(),
		`SELECT revision, updated_utc, payload_sha256 FROM cn_account_snapshot WHERE user_id = ?`,
		transfer.UserID,
	).Scan(&revision, &updated, &digestText)
	if errors.Is(err, sql.ErrNoRows) {
		updated = time.Now().UTC().Format(time.RFC3339Nano)
		if transfer.Revision < 1 {
			return errors.New("CN account transfer revision is invalid")
		}
		if _, err := transaction.ExecContext(context.Background(),
			`INSERT INTO cn_account_snapshot
			 (user_id, schema_version, revision, updated_utc, payload_json, payload_sha256)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			transfer.UserID, saveSnapshotSchemaVersion, transfer.Revision, updated, transfer.Metadata, transfer.MetadataDigest,
		); err != nil {
			return fmt.Errorf("insert CN account transfer: %w", err)
		}
		if err := upsertAccountProjection(transaction, transfer.UserID, transfer.Revision, updated, transfer.Account); err != nil {
			return err
		}
		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit CN account transfer: %w", err)
		}
		return nil
	}
	if err != nil {
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

func readAccountCredential(database *sql.DB, userID int) (AccountCredential, error) {
	var credential AccountCredential
	err := database.QueryRowContext(context.Background(),
		`SELECT username, password_salt, password_hash, iterations FROM cn_account_credentials WHERE user_id = ?`,
		userID,
	).Scan(&credential.Username, &credential.Salt, &credential.Hash, &credential.Iterations)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountCredential{}, nil
	}
	if err != nil {
		return AccountCredential{}, fmt.Errorf("export CN account credential: %w", err)
	}
	return credential, nil
}

func writeTransferredCredential(transaction *sql.Tx, transfer AccountTransfer) error {
	credential := transfer.Credential
	if credential.Username == "" && len(credential.Salt) == 0 && len(credential.Hash) == 0 && credential.Iterations == 0 {
		return nil
	}
	if credential.Username == "" || len(credential.Salt) != 16 || len(credential.Hash) != 32 || credential.Iterations != passwordIterations {
		return errors.New("CN account transfer credential is invalid")
	}
	var existingID int
	err := transaction.QueryRowContext(context.Background(),
		`SELECT user_id FROM cn_account_credentials WHERE username = ?`, credential.Username,
	).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && existingID != transfer.UserID {
		return errors.New("该账号名已使用")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = transaction.ExecContext(context.Background(),
		`INSERT INTO cn_account_credentials(user_id, username, password_salt, password_hash, iterations, updated_utc)
		 VALUES(?, ?, ?, ?, ?, ?) ON CONFLICT(user_id) DO UPDATE SET
		 username = excluded.username, password_salt = excluded.password_salt,
		 password_hash = excluded.password_hash, iterations = excluded.iterations, updated_utc = excluded.updated_utc`,
		transfer.UserID, credential.Username, credential.Salt, credential.Hash, credential.Iterations, now)
	if err != nil {
		return fmt.Errorf("write CN account credential: %w", err)
	}
	return nil
}

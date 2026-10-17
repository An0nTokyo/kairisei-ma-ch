package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Holder struct {
	UserID           int
	HolderID         string
	CheckoutRevision int
	MasterDigest     string
	Frozen           bool
}

func (accounts *Accounts) PutHolder(holder Holder) error {
	if holder.UserID < PrimaryUserID || holder.HolderID == "" {
		return errors.New("CN account holder is invalid")
	}
	database, err := accounts.storage.Open()
	if err != nil {
		return err
	}
	frozen := 0
	if holder.Frozen {
		frozen = 1
	}
	if _, err := database.ExecContext(context.Background(),
		`INSERT INTO cn_account_holder
		 (user_id, holder_id, checkout_revision, master_digest, frozen)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		  holder_id = excluded.holder_id,
		  checkout_revision = excluded.checkout_revision,
		  master_digest = excluded.master_digest,
		  frozen = excluded.frozen`,
		holder.UserID, holder.HolderID, holder.CheckoutRevision, holder.MasterDigest, frozen,
	); err != nil {
		return fmt.Errorf("write CN account holder: %w", err)
	}
	return nil
}

func (accounts *Accounts) Holder(userID int) (Holder, error) {
	database, err := accounts.storage.OpenRead()
	if err != nil {
		return Holder{}, err
	}
	var holder Holder
	var frozen int
	err = database.QueryRowContext(context.Background(),
		`SELECT user_id, holder_id, checkout_revision, master_digest, frozen
		 FROM cn_account_holder WHERE user_id = ?`, userID,
	).Scan(&holder.UserID, &holder.HolderID, &holder.CheckoutRevision, &holder.MasterDigest, &frozen)
	if errors.Is(err, sql.ErrNoRows) {
		return Holder{}, errors.New("CN account holder is missing")
	}
	if err != nil {
		return Holder{}, fmt.Errorf("read CN account holder: %w", err)
	}
	holder.Frozen = frozen == 1
	return holder, nil
}

func (accounts *Accounts) RefuseIfNotHolder(userID int, holderID string) error {
	holder, err := accounts.Holder(userID)
	if err != nil {
		return err
	}
	if holder.HolderID != holderID || holder.Frozen {
		return errors.New("CN account holder refused this server")
	}
	return nil
}

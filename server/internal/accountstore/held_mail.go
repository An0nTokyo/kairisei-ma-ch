package accountstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"kairisei.local/server/internal/gamestate"
)

func (accounts *Accounts) EnqueueHeldMail(userID int, rewardJSON []byte, eventKey string) error {
	if userID < PrimaryUserID || eventKey == "" || len(eventKey) > 256 || len(rewardJSON) == 0 {
		return errors.New("CN held mail is invalid")
	}
	var reward gamestate.Reward
	if err := json.Unmarshal(rewardJSON, &reward); err != nil {
		return fmt.Errorf("decode CN held mail: %w", err)
	}
	database, err := accounts.storage.Open()
	if err != nil {
		return err
	}
	if _, err := database.ExecContext(context.Background(),
		`INSERT INTO cn_account_held_mail (user_id, event_key, reward_json)
		 VALUES (?, ?, ?)
		 ON CONFLICT(user_id, event_key) DO NOTHING`,
		userID, eventKey, string(rewardJSON),
	); err != nil {
		return fmt.Errorf("queue CN held mail: %w", err)
	}
	return nil
}

func (accounts *Accounts) DeliverHeldMail(userID int) error {
	if userID < PrimaryUserID {
		return errors.New("CN held mail user is invalid")
	}
	database, err := accounts.storage.Open()
	if err != nil {
		return err
	}
	transaction, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin CN held mail delivery: %w", err)
	}
	defer transaction.Rollback()
	rows, err := transaction.QueryContext(context.Background(),
		`SELECT event_key, reward_json FROM cn_account_held_mail WHERE user_id = ? ORDER BY event_key`,
		userID,
	)
	if err != nil {
		return err
	}
	type queued struct {
		key    string
		reward gamestate.Reward
	}
	var mail []queued
	for rows.Next() {
		var item queued
		var raw string
		if err := rows.Scan(&item.key, &raw); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal([]byte(raw), &item.reward); err != nil {
			rows.Close()
			return err
		}
		mail = append(mail, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(mail) == 0 {
		return nil
	}
	var schemaVersion int
	var content []byte
	var expectedDigest string
	if err := transaction.QueryRowContext(context.Background(),
		`SELECT schema_version, payload_json, payload_sha256 FROM cn_account_snapshot WHERE user_id = ?`,
		userID,
	).Scan(&schemaVersion, &content, &expectedDigest); err != nil {
		return fmt.Errorf("read CN account for held mail: %w", err)
	}
	if schemaVersion != saveSnapshotSchemaVersion {
		return fmt.Errorf("unsupported CN account snapshot schema %d", schemaVersion)
	}
	sum := sha256.Sum256(content)
	if hex.EncodeToString(sum[:]) != expectedDigest {
		return errors.New("CN account snapshot digest mismatch")
	}
	state, err := accounts.storage.decodeAccount(transaction, content)
	if err != nil {
		return err
	}
	used := map[int64]bool{}
	for _, present := range append(state.Engagement.Presents, state.Engagement.Histories...) {
		used[present.PresentID] = true
	}
	empty := gamestate.Reward{CardSkillLevels: []int16{}}
	for _, item := range mail {
		presentID := time.Now().UnixNano()
		for used[presentID] {
			presentID++
		}
		used[presentID] = true
		reward := item.reward
		if reward.CardSkillLevels == nil {
			reward.CardSkillLevels = []int16{}
		}
		state.Engagement.Presents = append(state.Engagement.Presents, gamestate.Present{
			PresentID:    presentID,
			IssuedAtUnix: time.Now().Unix(),
			Title:        item.key,
			Reward:       reward,
			Reward0:      empty,
			Reward1:      empty,
			Reward2:      empty,
		})
	}
	updated := time.Now().UTC().Format(time.RFC3339Nano)
	encoded, err := EncodeAccountMetadata(state)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(encoded)
	if err := writeAccountSnapshot(transaction, userID, state, encoded, hex.EncodeToString(digest[:]), updated, nil); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(context.Background(),
		`DELETE FROM cn_account_held_mail WHERE user_id = ?`, userID,
	); err != nil {
		return fmt.Errorf("clear CN held mail: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit CN held mail: %w", err)
	}
	return nil
}

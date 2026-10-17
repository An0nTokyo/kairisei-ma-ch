package accountstore

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigratePrimarySnapshotCopiesSingletonAndIgnoresStaleMirror(t *testing.T) {
	storage := openMigrationDatabase(t)
	mustExec(t, storage, `INSERT INTO cn_local_account
		(user_id, login_uuid, session_key, created_utc, last_login_utc)
		VALUES (1000001, 'primary-uuid', 'primary-session', 't', 't')`)
	payload := []byte(`{"user":{"user_id":1000001}}`)
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	mustExec(t, storage, `INSERT INTO cn_save_snapshot
		(singleton, schema_version, revision, updated_utc, payload_json, payload_sha256)
		VALUES (1, 3, 7, 't', ?, ?)`, payload, digest)
	mustExec(t, storage, `INSERT INTO cn_account_snapshot
		(user_id, schema_version, revision, updated_utc, payload_json, payload_sha256)
		VALUES (1000001, 3, 1, 'old', ?, ?)`, []byte(`{"user":{"user_id":1000001},"stale":true}`), digestOf([]byte(`{"user":{"user_id":1000001},"stale":true}`)))

	if err := storage.MigratePrimarySnapshot(); err != nil {
		t.Fatal(err)
	}
	var revision int
	var got []byte
	var gotDigest string
	mustQuery(t, storage, `SELECT revision, payload_json, payload_sha256 FROM cn_account_snapshot WHERE user_id = 1000001`,
		&revision, &got, &gotDigest)
	if revision != 7 || string(got) != string(payload) || gotDigest != digest {
		t.Fatalf("migrated snapshot = revision %d body %s digest %s", revision, got, gotDigest)
	}
	var singletonCount int
	mustQuery(t, storage, `SELECT COUNT(*) FROM cn_save_snapshot WHERE singleton = 1`, &singletonCount)
	if singletonCount != 1 {
		t.Fatalf("singleton rows = %d", singletonCount)
	}
}

func TestPrimaryAccountRoundTripUsesAccountSnapshot(t *testing.T) {
	storage := openMigrationDatabase(t)
	state, err := LoadSaveState(storage.seedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := InitializeOnboardingSnapshot(&state, PrimaryUserID); err != nil {
		t.Fatal(err)
	}
	content, err := EncodeAccountMetadata(state)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, storage, `INSERT INTO cn_local_account
		(user_id, login_uuid, session_key, created_utc, last_login_utc)
		VALUES (1000001, 'primary-uuid', 'primary-session', 't', 't')`)
	mustExec(t, storage, `INSERT INTO cn_save_snapshot
		(singleton, schema_version, revision, updated_utc, payload_json, payload_sha256)
		VALUES (1, 3, 7, 't', ?, ?)`, content, digestOf(content))
	database, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAccountRows(transaction, state); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := storage.MigratePrimarySnapshot(); err != nil {
		t.Fatal(err)
	}
	accounts, err := NewAccounts(storage)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := accounts.LoadPersistentState(PrimaryUserID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.User.UserID != PrimaryUserID {
		t.Fatalf("user id = %d", loaded.User.UserID)
	}
	loaded.User.Gold += 11
	if err := accounts.PersistState(PrimaryUserID, loaded); err != nil {
		t.Fatal(err)
	}
	reloaded, err := accounts.LoadPersistentState(PrimaryUserID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.User.Gold != loaded.User.Gold {
		t.Fatalf("gold = %d, want %d", reloaded.User.Gold, loaded.User.Gold)
	}
	var revision int
	var still []byte
	mustQuery(t, storage, `SELECT revision, payload_json FROM cn_save_snapshot WHERE singleton = 1`, &revision, &still)
	if revision != 7 || string(still) != string(content) {
		t.Fatalf("singleton changed to revision %d (%d bytes)", revision, len(still))
	}
}

func TestMigratePrimarySnapshotRejectsDigestMismatch(t *testing.T) {
	storage := openMigrationDatabase(t)
	mustExec(t, storage, `INSERT INTO cn_local_account
		(user_id, login_uuid, session_key, created_utc, last_login_utc)
		VALUES (1000001, 'primary-uuid', 'primary-session', 't', 't')`)
	stale := []byte(`{"user":{"user_id":1000001},"stale":true}`)
	mustExec(t, storage, `INSERT INTO cn_account_snapshot
		(user_id, schema_version, revision, updated_utc, payload_json, payload_sha256)
		VALUES (1000001, 3, 1, 'old', ?, ?)`, stale, digestOf(stale))
	payload := []byte(`{"user":{"user_id":1000001}}`)
	mustExec(t, storage, `INSERT INTO cn_save_snapshot
		(singleton, schema_version, revision, updated_utc, payload_json, payload_sha256)
		VALUES (1, 3, 7, 't', ?, ?)`, payload, digestOf([]byte(`not-the-payload`)))

	err := storage.MigratePrimarySnapshot()
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("error = %v", err)
	}
	var revision int
	var got []byte
	mustQuery(t, storage, `SELECT revision, payload_json FROM cn_account_snapshot WHERE user_id = 1000001`,
		&revision, &got)
	if revision != 1 || string(got) != string(stale) {
		t.Fatalf("snapshot changed to revision %d body %s", revision, got)
	}
}

func openMigrationDatabase(t *testing.T) *Database {
	t.Helper()
	storage, err := OpenDatabase(filepath.Join(t.TempDir(), "save.json"), filepath.Join("..", "..", "config", "cn602-save-template.json"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := storage.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := storage.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	return storage
}

func mustExec(t *testing.T, storage *Database, query string, args ...any) {
	t.Helper()
	database, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func mustQuery(t *testing.T, storage *Database, query string, dest ...any) {
	t.Helper()
	database, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(query).Scan(dest...); err != nil {
		t.Fatal(err)
	}
}

func digestOf(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

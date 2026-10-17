package accountstore

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
)

func TestFindLoginDoesNotCreate(t *testing.T) {
	storage, err := OpenDatabase(
		filepath.Join(t.TempDir(), "save.json"),
		filepath.Join("..", "..", "config", "cn602-save-template.json"),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := storage.Close(); err != nil {
			t.Error(err)
		}
	})
	accounts, err := NewAccounts(storage)
	if err != nil {
		t.Fatal(err)
	}

	userID, found, err := accounts.FindLogin("00000000-0000-0000-0000-000000000099")
	if err != nil || found || userID != 0 {
		t.Fatalf("missing login = %d, %v, %v", userID, found, err)
	}
	var count int
	mustQuery(t, storage, `SELECT COUNT(*) FROM cn_local_account WHERE user_id < 1900000000`, &count)
	if count != 0 {
		t.Fatalf("player rows = %d", count)
	}

	mustExec(t, storage, `INSERT INTO cn_local_account
		(user_id, login_uuid, session_key, created_utc, last_login_utc)
		VALUES (1000002, '00000000-0000-0000-0000-000000000099', 'held-session', 't', 't')`)
	userID, found, err = accounts.FindLogin("00000000-0000-0000-0000-000000000099")
	if err != nil || !found || userID != 1000002 {
		t.Fatalf("held login = %d, %v, %v", userID, found, err)
	}

	userID, found, err = accounts.FindLogin("not-a-uuid")
	if err != nil || found || userID != 0 {
		t.Fatalf("invalid login = %d, %v, %v", userID, found, err)
	}
}

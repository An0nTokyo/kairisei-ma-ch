package accountstore

import (
	"database/sql"
	"strconv"
	"testing"

	"kairisei.local/server/internal/gamestate"
)

func TestSyncRosterClassifiesHoldersWithoutWriting(t *testing.T) {
	storage := openMigrationDatabase(t)
	accounts, err := NewAccounts(storage)
	if err != nil {
		t.Fatal(err)
	}
	database, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	insert := func(id int, name string) {
		t.Helper()
		if _, err := database.Exec(`INSERT INTO cn_local_account
			(user_id, login_uuid, session_key, created_utc, last_login_utc)
			VALUES (?, ?, ?, 't', 't')`, id, "uuid-"+strconv.Itoa(id), "session-"+strconv.Itoa(id)); err != nil {
			t.Fatal(err)
		}
		if name == "" {
			return
		}
		payload, digest, err := EncodeAccountProjection(gamestate.State{User: gamestate.User{UserID: id, Name: name}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`INSERT INTO cn_account_projection
			(user_id, schema_version, snapshot_revision, updated_utc, payload_json, payload_sha256)
			VALUES (?, 2, 1, 't', ?, ?)`, id, payload, digest); err != nil {
			t.Fatal(err)
		}
	}
	insert(1000008, "秋叶")
	insert(1000001, "秋")
	insert(1000002, "另一名")
	insert(1000003, "别人")
	insert(1000004, "冻结")
	insert(1000005, "电脑")
	insert(1000006, "")
	insert(SystemPartnerUserIDBase, "伙伴")
	if err := accounts.PutHolder(Holder{UserID: 1000002, HolderID: "device-a"}); err != nil {
		t.Fatal(err)
	}
	if err := accounts.PutHolder(Holder{UserID: 1000003, HolderID: "device-b"}); err != nil {
		t.Fatal(err)
	}
	if err := accounts.PutHolder(Holder{UserID: 1000004, HolderID: "s", Frozen: true}); err != nil {
		t.Fatal(err)
	}
	if err := accounts.PutHolder(Holder{UserID: 1000005, HolderID: "s"}); err != nil {
		t.Fatal(err)
	}
	before := holderSnapshot(t, database, 1000002)

	rows, err := accounts.SyncRoster("device-a")
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]SyncAccount{}
	var order []int
	for _, row := range rows {
		got[row.UserID] = row
		order = append(order, row.UserID)
	}
	if _, ok := got[SystemPartnerUserIDBase]; ok {
		t.Fatal("system partner was listed")
	}
	wantOrder := []int{1000001, 1000002, 1000003, 1000004, 1000005, 1000006, 1000008}
	if len(order) != len(wantOrder) {
		t.Fatalf("order = %v", order)
	}
	for i, id := range wantOrder {
		if order[i] != id {
			t.Fatalf("order = %v", order)
		}
	}
	checks := []struct {
		id, name, state string
		user            int
	}{
		{user: 1000001, name: "秋", state: SyncStateServer},
		{user: 1000002, name: "另一名", state: SyncStateMine},
		{user: 1000003, name: "别人", state: SyncStateOther},
		{user: 1000004, name: "冻结", state: SyncStateOther},
		{user: 1000005, name: "电脑", state: SyncStateServer},
		{user: 1000006, name: "", state: SyncStateServer},
	}
	for _, check := range checks {
		row := got[check.user]
		if row.Name != check.name || row.State != check.state {
			t.Fatalf("user %d = %+v", check.user, row)
		}
	}
	if after := holderSnapshot(t, database, 1000002); after != before {
		t.Fatalf("holder changed from %s to %s", before, after)
	}
}

func holderSnapshot(t *testing.T, database *sql.DB, userID int) string {
	t.Helper()
	var holder, digest string
	var revision, frozen int
	err := database.QueryRow(`SELECT holder_id, checkout_revision, master_digest, frozen
		FROM cn_account_holder WHERE user_id = ?`, userID).Scan(&holder, &revision, &digest, &frozen)
	if err != nil {
		t.Fatal(err)
	}
	return holder + " " + strconv.Itoa(revision) + " " + digest + " " + strconv.Itoa(frozen)
}

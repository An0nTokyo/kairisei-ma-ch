package accountstore

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestImportAccountIsIdempotent(t *testing.T) {
	storage := openMigrationDatabase(t)
	if _, err := storage.LoadOrImport(); err != nil {
		t.Fatal(err)
	}
	accounts, err := NewAccounts(storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.ResolveLogin("00000000-0000-4000-8000-0000000000b1"); err != nil {
		t.Fatal(err)
	}
	second, err := accounts.ResolveLogin("00000000-0000-4000-8000-0000000000b2")
	if err != nil || second.UserID != PrimaryUserID+1 {
		t.Fatalf("second user = %+v %v", second, err)
	}
	transfer, err := accounts.ExportAccount(second.UserID)
	if err != nil {
		t.Fatal(err)
	}
	transferType := reflect.TypeOf(transfer)
	for i := 0; i < transferType.NumField(); i++ {
		if strings.Contains(strings.ToLower(transferType.Field(i).Name), "clear") {
			t.Fatalf("transfer field %s stores clear counts", transferType.Field(i).Name)
		}
	}
	cards := queryInt(t, storage, `SELECT COUNT(*) FROM cn_account_card WHERE user_id = ?`, second.UserID)
	if cards == 0 {
		t.Fatal("exported account has no cards")
	}
	before, err := accounts.LoadPersistentState(second.UserID)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, storage, `DELETE FROM cn_account_card WHERE user_id = ?`, second.UserID)
	mustExec(t, storage, `CREATE TRIGGER fail_clear_insert BEFORE INSERT ON cn_battle_clear_daily BEGIN SELECT RAISE(ABORT,'clear written'); END`)
	if err := accounts.ImportAccount(transfer); err != nil {
		t.Fatal(err)
	}
	if err := accounts.ImportAccount(transfer); err != nil {
		t.Fatal(err)
	}
	reloaded, err := accounts.LoadPersistentState(second.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.User.Gold != before.User.Gold {
		t.Fatalf("gold = %d, want %d", reloaded.User.Gold, before.User.Gold)
	}
	if got := queryInt(t, storage, `SELECT COUNT(*) FROM cn_account_card WHERE user_id = ?`, second.UserID); got != cards {
		t.Fatalf("cards = %d, want %d", got, cards)
	}
	if got := queryInt(t, storage, `SELECT revision FROM cn_account_snapshot WHERE user_id = ?`, second.UserID); got != transfer.Revision {
		t.Fatalf("revision = %d, want %d", got, transfer.Revision)
	}

	if err := accounts.PutHolder(Holder{
		UserID:           second.UserID,
		HolderID:         "device-a",
		CheckoutRevision: transfer.Revision,
		MasterDigest:     "masters",
	}); err != nil {
		t.Fatal(err)
	}
	accountRowWriteHook = func() error { return errors.New("forced row write") }
	defer func() { accountRowWriteHook = nil }()
	if err := accounts.ImportAccount(transfer); err == nil {
		t.Fatal("row write failure was ignored")
	}
	accountRowWriteHook = nil
	holder, err := accounts.Holder(second.UserID)
	if err != nil || holder.HolderID != "device-a" || holder.Frozen {
		t.Fatalf("holder changed: %+v %v", holder, err)
	}
	if got := queryInt(t, storage, `SELECT COUNT(*) FROM cn_account_card WHERE user_id = ?`, second.UserID); got != cards {
		t.Fatalf("failed import left %d cards", got)
	}
}

func queryInt(t *testing.T, storage *Database, query string, args ...any) int {
	t.Helper()
	database, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	var value int
	if err := database.QueryRow(query, args...).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

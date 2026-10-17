package accountstore

import "testing"

func TestDeliverHeldMailOnce(t *testing.T) {
	storage := openMigrationDatabase(t)
	if _, err := storage.LoadOrImport(); err != nil {
		t.Fatal(err)
	}
	accounts, err := NewAccounts(storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.ResolveLogin("00000000-0000-4000-8000-0000000000c1"); err != nil {
		t.Fatal(err)
	}
	second, err := accounts.ResolveLogin("00000000-0000-4000-8000-0000000000c2")
	if err != nil || second.UserID != PrimaryUserID+1 {
		t.Fatalf("second user = %+v %v", second, err)
	}
	if err := accounts.PutHolder(Holder{
		UserID:           second.UserID,
		HolderID:         "device-a",
		CheckoutRevision: 1,
		MasterDigest:     "masters",
	}); err != nil {
		t.Fatal(err)
	}
	reward := []byte(`{"type":4,"num":7,"card_skill_lv":[]}`)
	if err := accounts.EnqueueHeldMail(second.UserID, reward, "mail-a"); err != nil {
		t.Fatal(err)
	}
	if err := accounts.EnqueueHeldMail(second.UserID, reward, "mail-b"); err != nil {
		t.Fatal(err)
	}
	before, err := accounts.LoadPersistentState(second.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if err := accounts.DeliverHeldMail(second.UserID); err != nil {
		t.Fatal(err)
	}
	if err := accounts.DeliverHeldMail(second.UserID); err != nil {
		t.Fatal(err)
	}
	after, err := accounts.LoadPersistentState(second.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Engagement.Presents) != len(before.Engagement.Presents)+2 {
		t.Fatalf("presents = %d, want %d", len(after.Engagement.Presents), len(before.Engagement.Presents)+2)
	}
	if got := queryInt(t, storage, `SELECT COUNT(*) FROM cn_account_held_mail WHERE user_id = ?`, second.UserID); got != 0 {
		t.Fatalf("held mail rows = %d", got)
	}
	if err := accounts.EnqueueHeldMail(second.UserID, reward, "mail-a"); err != nil {
		t.Fatal(err)
	}
	again, err := accounts.LoadPersistentState(second.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Engagement.Presents) != len(after.Engagement.Presents) {
		t.Fatalf("duplicate event added a present: %d", len(again.Engagement.Presents))
	}
}

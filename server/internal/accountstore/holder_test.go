package accountstore

import (
	"strings"
	"testing"
)

func TestHolderRefusesNonOwner(t *testing.T) {
	storage := openMigrationDatabase(t)
	accounts, err := NewAccounts(storage)
	if err != nil {
		t.Fatal(err)
	}
	if err := accounts.PutHolder(Holder{
		UserID:           PrimaryUserID,
		HolderID:         "device-a",
		CheckoutRevision: 3,
		MasterDigest:     "abc",
	}); err != nil {
		t.Fatal(err)
	}
	if err := accounts.RefuseIfNotHolder(PrimaryUserID, "device-b"); err == nil || !strings.Contains(err.Error(), "holder") {
		t.Fatalf("other holder error = %v", err)
	}
	if err := accounts.RefuseIfNotHolder(PrimaryUserID, "device-a"); err != nil {
		t.Fatal(err)
	}
	holder, err := accounts.Holder(PrimaryUserID)
	if err != nil {
		t.Fatal(err)
	}
	if holder.HolderID != "device-a" || holder.CheckoutRevision != 3 || holder.MasterDigest != "abc" || holder.Frozen {
		t.Fatalf("holder = %+v", holder)
	}
	holder.Frozen = true
	if err := accounts.PutHolder(holder); err != nil {
		t.Fatal(err)
	}
	if err := accounts.RefuseIfNotHolder(PrimaryUserID, "device-a"); err == nil || !strings.Contains(err.Error(), "holder") {
		t.Fatalf("frozen holder error = %v", err)
	}
}

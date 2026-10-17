package accountstore

import (
	"strings"
	"testing"
)

func TestMasterDigestChangesWhenEitherInputChanges(t *testing.T) {
	first := MasterDigest([]byte("cards"), []byte("growth"))
	if first == "" || first != MasterDigest([]byte("cards"), []byte("growth")) {
		t.Fatalf("digest = %q", first)
	}
	if first == MasterDigest([]byte("cards-changed"), []byte("growth")) ||
		first == MasterDigest([]byte("cards"), []byte("growth-changed")) {
		t.Fatal("master digest ignored an input")
	}
}

func TestMasterDigestMismatchSkipsImport(t *testing.T) {
	storage := openMigrationDatabase(t)
	if _, err := storage.LoadOrImport(); err != nil {
		t.Fatal(err)
	}
	accounts, err := NewAccounts(storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.ResolveLogin("00000000-0000-4000-8000-0000000000d1"); err != nil {
		t.Fatal(err)
	}
	second, err := accounts.ResolveLogin("00000000-0000-4000-8000-0000000000d2")
	if err != nil {
		t.Fatal(err)
	}
	transfer, err := accounts.ExportAccount(second.UserID)
	if err != nil {
		t.Fatal(err)
	}
	transfer.MasterDigest = MasterDigest([]byte("phone-cards"), []byte("phone-growth"))
	local := MasterDigest([]byte("server-cards"), []byte("server-growth"))
	called := false
	accountRowWriteHook = func() error {
		called = true
		return nil
	}
	defer func() { accountRowWriteHook = nil }()
	err = accounts.ImportMatchingMaster(transfer, local)
	if err == nil || !strings.Contains(err.Error(), "master digest") {
		t.Fatalf("error = %v", err)
	}
	if called {
		t.Fatal("import wrote account rows")
	}
}

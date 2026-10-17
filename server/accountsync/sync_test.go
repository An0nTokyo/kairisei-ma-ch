package accountsync_test

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kairisei.local/server/accountsync"
	"kairisei.local/server/localcn"
)

func TestBorrowAndReturn(t *testing.T) {
	serverStore, phoneStore, phoneDir := openSyncPair(t)
	userID, err := serverStore.ResolveLogin("00000000-0000-0000-0000-000000000099")
	if err != nil {
		t.Fatal(err)
	}
	cardsPath := filepath.Join(t.TempDir(), "cards.bin")
	growthPath := filepath.Join(t.TempDir(), "growth.bin")
	if err := os.WriteFile(cardsPath, []byte("cards-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(growthPath, []byte("growth-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := accountsync.Handler("secret", serverDB{store: serverStore, cardsPath: cardsPath, growthPath: growthPath})
	remote := httptest.NewServer(handler)
	t.Cleanup(remote.Close)
	phone := phoneDB{store: phoneStore, dir: phoneDir}
	const deviceID = "device-phone"

	request := httptest.NewRequest(http.MethodPost, "/local/sync/borrow", strings.NewReader(`{"user_id":1}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing token status = %d", recorder.Code)
	}

	if err := os.MkdirAll(filepath.Join(phoneDir, "masters"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phoneDir, "masters", "cards.bin"), []byte("other-cards"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phoneDir, "masters", "growth.bin"), []byte("other-growth"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := accountsync.Borrow(remote.URL, "secret", deviceID, userID, phone); err == nil || !strings.Contains(err.Error(), "master digest") {
		t.Fatalf("mismatch error = %v", err)
	}
	if _, found, err := phoneStore.FindLogin("00000000-0000-0000-0000-000000000099"); err != nil || found {
		t.Fatalf("phone login after mismatch = %v found %v", err, found)
	}
	holder, err := serverStore.Holder(userID)
	if err != nil {
		t.Fatal(err)
	}
	if holder.HolderID != localcn.ServerHolderID || holder.Frozen {
		t.Fatalf("server holder after mismatch = %+v", holder)
	}
	if err := os.RemoveAll(filepath.Join(phoneDir, "masters")); err != nil {
		t.Fatal(err)
	}

	if err := accountsync.Borrow(remote.URL, "secret", deviceID, userID, phone); err != nil {
		t.Fatal(err)
	}
	if _, found, err := phoneStore.FindLogin("00000000-0000-0000-0000-000000000099"); err != nil || !found {
		t.Fatalf("phone login after borrow = %v found %v", err, found)
	}
	holder, err = serverStore.Holder(userID)
	if err != nil || holder.HolderID != deviceID || holder.Frozen {
		t.Fatalf("server holder after borrow = %+v %v", holder, err)
	}
	before, err := serverStore.Gold(userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := phoneStore.AddGold(userID, 11); err != nil {
		t.Fatal(err)
	}
	if err := accountsync.Return(remote.URL, "secret", deviceID, userID, "", phone); err != nil {
		t.Fatal(err)
	}
	if gold, err := serverStore.Gold(userID); err != nil || gold != before+11 {
		t.Fatalf("gold after return = %d %v, before %d", gold, err, before)
	}
	holder, err = serverStore.Holder(userID)
	if err != nil || holder.HolderID != localcn.ServerHolderID || holder.Frozen {
		t.Fatalf("server holder after return = %+v %v", holder, err)
	}

	if err := accountsync.Borrow(remote.URL, "secret", deviceID, userID, phone); err != nil {
		t.Fatal(err)
	}
	if err := serverStore.AdvanceRevision(userID); err != nil {
		t.Fatal(err)
	}
	kept, err := serverStore.Gold(userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := phoneStore.AddGold(userID, 5); err != nil {
		t.Fatal(err)
	}
	err = accountsync.Return(remote.URL, "secret", deviceID, userID, "", phone)
	var choice *accountsync.ChoiceRequiredError
	if err == nil || !errors.As(err, &choice) {
		t.Fatalf("choice error = %v", err)
	}
	if choice.ServerRevision == 0 || choice.PhoneRevision == 0 {
		t.Fatalf("choice = %+v", choice)
	}
	if gold, err := serverStore.Gold(userID); err != nil || gold != kept {
		t.Fatalf("gold while choosing = %d %v", gold, err)
	}
	holder, err = serverStore.Holder(userID)
	if err != nil || holder.HolderID != deviceID {
		t.Fatalf("holder while choosing = %+v %v", holder, err)
	}
	if err := accountsync.Return(remote.URL, "secret", deviceID, userID, "server", phone); err != nil {
		t.Fatal(err)
	}
	if gold, err := serverStore.Gold(userID); err != nil || gold != kept {
		t.Fatalf("gold after keeping server = %d %v", gold, err)
	}
	holder, err = serverStore.Holder(userID)
	if err != nil || holder.HolderID != localcn.ServerHolderID {
		t.Fatalf("holder after keeping server = %+v %v", holder, err)
	}

	if err := accountsync.Borrow(remote.URL, "secret", deviceID, userID, phone); err != nil {
		t.Fatal(err)
	}
	if err := serverStore.AdvanceRevision(userID); err != nil {
		t.Fatal(err)
	}
	if err := phoneStore.AddGold(userID, 5); err != nil {
		t.Fatal(err)
	}
	if err := accountsync.Return(remote.URL, "secret", deviceID, userID, "phone", phone); err != nil {
		t.Fatal(err)
	}
	if gold, err := serverStore.Gold(userID); err != nil || gold != kept+5 {
		t.Fatalf("gold after keeping phone = %d %v, kept %d", gold, err, kept)
	}
	holder, err = serverStore.Holder(userID)
	if err != nil || holder.HolderID != localcn.ServerHolderID || holder.Frozen {
		t.Fatalf("holder after keeping phone = %+v %v", holder, err)
	}
}

func openSyncPair(t *testing.T) (*localcn.Store, *localcn.Store, string) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	seed := filepath.Join("..", "config", "cn602-save-template.json")
	serverStore, err := localcn.Open(filepath.Join(t.TempDir(), "save.json"), seed, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverStore.Close() })
	if err := serverStore.LoadOrImport(); err != nil {
		t.Fatal(err)
	}
	phoneDir := t.TempDir()
	phoneStore, err := localcn.Open(filepath.Join(phoneDir, "save.json"), seed, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = phoneStore.Close() })
	return serverStore, phoneStore, phoneDir
}

type serverDB struct {
	store      *localcn.Store
	cardsPath  string
	growthPath string
}

func (db serverDB) Holder(userID int) (accountsync.HolderView, error) {
	holder, err := db.store.Holder(userID)
	if err != nil {
		return accountsync.HolderView{}, err
	}
	return holderView(holder), nil
}

func (db serverDB) SaveHolder(userID int, holderID string, checkoutRevision int, masterDigest string, frozen bool) error {
	return db.store.SaveHolder(userID, holderID, checkoutRevision, masterDigest, frozen)
}

func (db serverDB) Revision(userID int) (int, error) { return db.store.SnapshotRevision(userID) }

func (db serverDB) Export(userID int) ([]byte, error) { return db.store.ExportTransferJSON(userID) }

func (db serverDB) Import(raw []byte) error { return db.store.ImportTransferJSON(raw) }

func (db serverDB) LoginUUID(userID int) (string, error) { return db.store.LoginUUID(userID) }

func (db serverDB) DeliverHeldMail(userID int) error { return db.store.DeliverHeldMail(userID) }

func (db serverDB) MasterFiles() ([]byte, []byte, error) {
	cards, err := os.ReadFile(db.cardsPath)
	if err != nil {
		return nil, nil, err
	}
	growth, err := os.ReadFile(db.growthPath)
	if err != nil {
		return nil, nil, err
	}
	return cards, growth, nil
}

type phoneDB struct {
	store *localcn.Store
	dir   string
}

func (phone phoneDB) EnsureLogin(userID int, loginUUID, session string) error {
	return phone.store.EnsureLogin(userID, loginUUID, session)
}

func (phone phoneDB) Import(raw []byte) error { return phone.store.ImportTransferJSON(raw) }

func (phone phoneDB) Export(userID int) ([]byte, error) {
	return phone.store.ExportTransferJSON(userID)
}

func (phone phoneDB) Holder(userID int) (accountsync.HolderView, error) {
	holder, err := phone.store.Holder(userID)
	if err != nil {
		return accountsync.HolderView{}, err
	}
	return holderView(holder), nil
}

func (phone phoneDB) SaveHolder(userID int, holderID string, checkoutRevision int, masterDigest string, frozen bool) error {
	return phone.store.SaveHolder(userID, holderID, checkoutRevision, masterDigest, frozen)
}

func (phone phoneDB) ReadMasters() ([]byte, []byte, bool, error) {
	return accountsync.ReadMasterFiles(phone.dir)
}

func (phone phoneDB) WriteMasters(cards, growth []byte) error {
	return accountsync.WriteMasterFiles(phone.dir, cards, growth)
}

func holderView(holder localcn.HolderView) accountsync.HolderView {
	return accountsync.HolderView{
		HolderID:         holder.HolderID,
		CheckoutRevision: holder.CheckoutRevision,
		MasterDigest:     holder.MasterDigest,
		Frozen:           holder.Frozen,
	}
}

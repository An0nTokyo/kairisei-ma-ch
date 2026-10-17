package sync

import (
	"kairisei.local/server/accountsync"
	"kairisei.local/server/localcn"
)

type phoneStore struct {
	store *localcn.Store
	dir   string
}

func Borrow(baseURL, token, deviceID string, userID int, dataDir string, store *localcn.Store) error {
	return accountsync.Borrow(baseURL, token, deviceID, userID, phoneStore{store: store, dir: dataDir})
}

func Return(baseURL, token, deviceID string, userID int, choice, dataDir string, store *localcn.Store) error {
	return accountsync.Return(baseURL, token, deviceID, userID, choice, phoneStore{store: store, dir: dataDir})
}

func (phone phoneStore) EnsureLogin(userID int, loginUUID, session string) error {
	return phone.store.EnsureLogin(userID, loginUUID, session)
}

func (phone phoneStore) Import(raw []byte) error { return phone.store.ImportTransferJSON(raw) }

func (phone phoneStore) Export(userID int) ([]byte, error) {
	return phone.store.ExportTransferJSON(userID)
}

func (phone phoneStore) Holder(userID int) (accountsync.HolderView, error) {
	holder, err := phone.store.Holder(userID)
	if err != nil {
		return accountsync.HolderView{}, err
	}
	return accountsync.HolderView{
		HolderID:         holder.HolderID,
		CheckoutRevision: holder.CheckoutRevision,
		MasterDigest:     holder.MasterDigest,
		Frozen:           holder.Frozen,
	}, nil
}

func (phone phoneStore) SaveHolder(userID int, holderID string, checkoutRevision int, masterDigest string, frozen bool) error {
	return phone.store.SaveHolder(userID, holderID, checkoutRevision, masterDigest, frozen)
}

func (phone phoneStore) ReadMasters() ([]byte, []byte, bool, error) {
	return accountsync.ReadMasterFiles(phone.dir)
}

func (phone phoneStore) WriteMasters(cards, growth []byte) error {
	return accountsync.WriteMasterFiles(phone.dir, cards, growth)
}

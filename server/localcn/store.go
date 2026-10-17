package localcn

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"kairisei.local/server/internal/accountstore"
	"kairisei.local/server/internal/cnbootstrap"
)

const ServerHolderID = "s"

type HolderView struct {
	HolderID         string
	CheckoutRevision int
	MasterDigest     string
	Frozen           bool
}

func Attach(accounts *accountstore.Accounts) *Store {
	return &Store{accounts: accounts}
}

// Store is the part of the CN account database that another module is allowed to call.
// The account implementation stays in the server module's internal packages.
type Store struct {
	database *accountstore.Database
	accounts *accountstore.Accounts
}

func Open(jsonPath, seedPath string, logger *slog.Logger) (*Store, error) {
	database, err := accountstore.OpenDatabase(jsonPath, seedPath, logger)
	if err != nil {
		return nil, err
	}
	accounts, err := accountstore.NewAccounts(database)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	return &Store{database: database, accounts: accounts}, nil
}

func (store *Store) Close() error {
	if store.database == nil {
		return nil
	}
	return store.database.Close()
}

func (store *Store) LoadOrImport() error {
	if store.database == nil {
		return errors.New("CN store has no database")
	}
	_, err := store.database.LoadOrImport()
	return err
}

func (store *Store) ResolveLogin(loginUUID string) (int, error) {
	return store.accounts.ResolveLoginUser(loginUUID)
}

func MasterDigest(cards, growth []byte) string {
	return accountstore.MasterDigest(cards, growth)
}

func (store *Store) ExportTransferJSON(userID int) ([]byte, error) {
	transfer, err := store.accounts.ExportAccount(userID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(transfer)
}

func (store *Store) ImportTransferJSON(raw []byte) error {
	var transfer accountstore.AccountTransfer
	if err := json.Unmarshal(raw, &transfer); err != nil {
		return err
	}
	return store.accounts.ImportAccount(transfer)
}

func (store *Store) FindLogin(loginUUID string) (int, bool, error) {
	return store.accounts.FindLogin(loginUUID)
}

func (store *Store) RefuseIfNotHolder(userID int, holderID string) error {
	return store.accounts.RefuseIfNotHolder(userID, holderID)
}

func (store *Store) PutHolder(userID int, holderID string) error {
	return store.accounts.PutHolder(accountstore.Holder{UserID: userID, HolderID: holderID})
}

func (store *Store) SaveHolder(userID int, holderID string, checkoutRevision int, masterDigest string, frozen bool) error {
	return store.accounts.PutHolder(accountstore.Holder{
		UserID:           userID,
		HolderID:         holderID,
		CheckoutRevision: checkoutRevision,
		MasterDigest:     masterDigest,
		Frozen:           frozen,
	})
}

func (store *Store) Holder(userID int) (HolderView, error) {
	holder, err := store.accounts.Holder(userID)
	if err != nil {
		return HolderView{}, err
	}
	return HolderView{
		HolderID:         holder.HolderID,
		CheckoutRevision: holder.CheckoutRevision,
		MasterDigest:     holder.MasterDigest,
		Frozen:           holder.Frozen,
	}, nil
}

func (store *Store) ExportAccount(userID int) (accountstore.AccountTransfer, error) {
	return store.accounts.ExportAccount(userID)
}

func (store *Store) ImportAccount(transfer accountstore.AccountTransfer) error {
	return store.accounts.ImportAccount(transfer)
}

func (store *Store) ImportMatchingMaster(transfer accountstore.AccountTransfer, masterDigest string) error {
	return store.accounts.ImportMatchingMaster(transfer, masterDigest)
}

func (store *Store) DeliverHeldMail(userID int) error {
	return store.accounts.DeliverHeldMail(userID)
}

func (store *Store) LoginUUID(userID int) (string, error) {
	database, err := store.accountsOpen()
	if err != nil {
		return "", err
	}
	var loginUUID string
	err = database.QueryRow(`SELECT login_uuid FROM cn_local_account WHERE user_id = ?`, userID).Scan(&loginUUID)
	return loginUUID, err
}

func (store *Store) SnapshotRevision(userID int) (int, error) {
	database, err := store.accountsOpen()
	if err != nil {
		return 0, err
	}
	var revision int
	err = database.QueryRow(`SELECT revision FROM cn_account_snapshot WHERE user_id = ?`, userID).Scan(&revision)
	return revision, err
}

func (store *Store) AdvanceRevision(userID int) error {
	database, err := store.accountsOpen()
	if err != nil {
		return err
	}
	_, err = database.Exec(`UPDATE cn_account_snapshot SET revision = revision + 1 WHERE user_id = ?`, userID)
	return err
}

func (store *Store) accountsOpen() (*sql.DB, error) {
	if store.database != nil {
		return store.database.Open()
	}
	return store.accounts.OpenStorage()
}

func (store *Store) InsertLogin(userID int, loginUUID, session string) error {
	return store.insertLogin(userID, loginUUID, session, false)
}

func (store *Store) EnsureLogin(userID int, loginUUID, session string) error {
	return store.insertLogin(userID, loginUUID, session, true)
}

func (store *Store) insertLogin(userID int, loginUUID, session string, ignoreConflict bool) error {
	database, err := store.accountsOpen()
	if err != nil {
		return err
	}
	query := `INSERT INTO cn_local_account
		(user_id, login_uuid, session_key, created_utc, last_login_utc)
		VALUES (?, ?, ?, 't', 't')`
	if ignoreConflict {
		query += ` ON CONFLICT(user_id) DO NOTHING`
	}
	_, err = database.Exec(query, userID, loginUUID, session)
	return err
}

func (store *Store) Gold(userID int) (int64, error) {
	state, err := store.accounts.LoadPersistentState(userID)
	if err != nil {
		return 0, err
	}
	return int64(state.User.Gold), nil
}

func (store *Store) AddGold(userID int, delta int64) error {
	state, err := store.accounts.LoadPersistentState(userID)
	if err != nil {
		return err
	}
	state.User.Gold += int(delta)
	return store.accounts.PersistState(userID, state)
}

func (store *Store) PlayerCount() (int, error) {
	database, err := store.database.Open()
	if err != nil {
		return 0, err
	}
	var count int
	err = database.QueryRow(`SELECT COUNT(*) FROM cn_local_account WHERE user_id < 1900000000`).Scan(&count)
	return count, err
}

func (store *Store) SessionKey(userID int) (string, error) {
	database, err := store.database.Open()
	if err != nil {
		return "", err
	}
	var session string
	err = database.QueryRow(`SELECT session_key FROM cn_local_account WHERE user_id = ?`, userID).Scan(&session)
	return session, err
}

func (store *Store) WriteHeldLogin(writer http.ResponseWriter, request *http.Request, host string, port int) {
	cnbootstrap.LoginHandler(store.accounts, host, port).ServeHTTP(writer, request)
}

func WriteServerList(writer http.ResponseWriter, request *http.Request, host string, port int) {
	cnbootstrap.ServerListHandler(host, port).ServeHTTP(writer, request)
}

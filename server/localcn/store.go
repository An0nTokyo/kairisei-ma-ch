package localcn

import (
	"log/slog"
	"net/http"

	"kairisei.local/server/internal/accountstore"
	"kairisei.local/server/internal/cnbootstrap"
)

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
	return store.database.Close()
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

func (store *Store) InsertLogin(userID int, loginUUID, session string) error {
	database, err := store.database.Open()
	if err != nil {
		return err
	}
	_, err = database.Exec(`INSERT INTO cn_local_account
		(user_id, login_uuid, session_key, created_utc, last_login_utc)
		VALUES (?, ?, ?, 't', 't')`, userID, loginUUID, session)
	return err
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

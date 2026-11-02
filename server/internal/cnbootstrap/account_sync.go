package cnbootstrap

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"

	"kairisei.local/server/accountsync"
	"kairisei.local/server/internal/accountstore"
)

type accountSyncDB struct {
	accounts   *accountstore.Accounts
	cardsPath  string
	growthPath string
	masters    MastersConfig
}

func (db accountSyncDB) Holder(userID int) (accountsync.HolderView, error) {
	holder, err := db.accounts.Holder(userID)
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

func (db accountSyncDB) SaveHolder(userID int, holderID string, checkoutRevision int, masterDigest string, frozen bool) error {
	return db.accounts.PutHolder(accountstore.Holder{
		UserID:           userID,
		HolderID:         holderID,
		CheckoutRevision: checkoutRevision,
		MasterDigest:     masterDigest,
		Frozen:           frozen,
	})
}

func (db accountSyncDB) Revision(userID int) (int, error) {
	database, err := db.accounts.OpenStorage()
	if err != nil {
		return 0, err
	}
	var revision int
	err = database.QueryRow(`SELECT revision FROM cn_account_snapshot WHERE user_id = ?`, userID).Scan(&revision)
	return revision, err
}

func (db accountSyncDB) Export(userID int) ([]byte, error) {
	transfer, err := db.accounts.ExportAccount(userID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(transfer)
}

func (db accountSyncDB) Import(raw []byte) error {
	var transfer accountstore.AccountTransfer
	if err := json.Unmarshal(raw, &transfer); err != nil {
		return err
	}
	return db.accounts.ImportAccount(transfer)
}

func (db accountSyncDB) LoginUUID(userID int) (string, error) {
	database, err := db.accounts.OpenStorage()
	if err != nil {
		return "", err
	}
	var loginUUID string
	err = database.QueryRow(`SELECT login_uuid FROM cn_local_account WHERE user_id = ?`, userID).Scan(&loginUUID)
	if err == sql.ErrNoRows {
		return "", err
	}
	return loginUUID, err
}

func (db accountSyncDB) DeliverHeldMail(userID int) error {
	return db.accounts.DeliverHeldMail(userID)
}

func (db accountSyncDB) MasterFiles() ([]byte, []byte, error) {
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

func (db accountSyncDB) ExtraMasters() (map[string][]byte, error) {
	paths := map[string]string{
		"explore.bin":     db.masters.Explore,
		"story.bin":       db.masters.Story,
		"navi.bin":        db.masters.Navi,
		"items.bin":       db.masters.Items,
		"avatar.bin":      db.masters.Avatar,
		"stamps.bin":      db.masters.Stamps,
		"honors.bin":      db.masters.Honors,
		"login-bonus.bin": db.masters.LoginBonus,
		"battle.bin":      db.masters.Battle,
		"pvp.bin":         db.masters.PVP,
	}
	files := map[string][]byte{}
	for name, path := range paths {
		if path == "" {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files[name] = body
	}
	if battle := db.masters.Battle; battle != "" {
		questPath := filepath.Join(filepath.Dir(battle), "cn602-normal-quest-runtime.json")
		body, err := os.ReadFile(questPath)
		if err != nil {
			return nil, err
		}
		files["cn602-normal-quest-runtime.json"] = body
	}
	return files, nil
}

func (db accountSyncDB) AccountRoster(deviceID string) ([]accountsync.AccountBrief, error) {
	rows, err := db.accounts.SyncRoster(deviceID)
	if err != nil {
		return nil, err
	}
	out := make([]accountsync.AccountBrief, len(rows))
	for i, row := range rows {
		out[i] = accountsync.AccountBrief{UserID: row.UserID, Name: row.Name, State: row.State}
	}
	return out, nil
}

func (db accountSyncDB) DefenseDecks(userID int) ([]byte, error) {
	states, err := db.accounts.ListPVPOpponents(userID)
	if err != nil {
		return nil, err
	}
	if len(states) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(states)
}

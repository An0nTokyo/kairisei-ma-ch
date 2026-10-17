package accountstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"kairisei.local/server/internal/gamestate"
)

const (
	saveSnapshotSchemaVersion = 3
	defaultSQLiteReadConns    = 32
)

type Database struct {
	schemaOnce         sync.Once
	schemaErr          error
	poolMu             sync.Mutex
	pool               *sql.DB
	readPool           *sql.DB
	readConns          int
	closed             bool
	catalog            *gamestate.State // immutable public configuration, initialized before serving accounts
	databasePath       string
	legacyDatabasePath string
	jsonPath           string
	seedPath           string
	logger             *slog.Logger
}

func OpenDatabase(jsonPath string, seedPath string, logger *slog.Logger) (*Database, error) {
	if jsonPath == "" {
		return nil, errors.New("CN save path is required")
	}
	if seedPath == "" {
		return nil, errors.New("CN save migration seed path is required")
	}
	absoluteJSONPath, err := filepath.Abs(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("resolve CN save path: %w", err)
	}
	extension := filepath.Ext(absoluteJSONPath)
	basePath := absoluteJSONPath[:len(absoluteJSONPath)-len(extension)]
	databasePath := basePath + "-state.sqlite3"
	legacyDatabasePath := basePath + ".sqlite3"
	if databasePath == absoluteJSONPath {
		return nil, errors.New("CN save database path must differ from the JSON path")
	}
	absoluteSeedPath, err := filepath.Abs(seedPath)
	if err != nil {
		return nil, fmt.Errorf("resolve CN save migration seed path: %w", err)
	}
	readConns := defaultSQLiteReadConns
	if value := os.Getenv("KAIRI_SQLITE_READ_CONNS"); value != "" {
		readConns, err = strconv.Atoi(value)
		if err != nil || readConns < 1 || readConns > 128 {
			return nil, errors.New("KAIRI_SQLITE_READ_CONNS must be 1 through 128")
		}
	}
	return &Database{
		readConns:          readConns,
		databasePath:       databasePath,
		legacyDatabasePath: legacyDatabasePath,
		jsonPath:           absoluteJSONPath,
		seedPath:           absoluteSeedPath,
		logger:             logger,
	}, nil
}

func (storage *Database) LoadOrImport() (gamestate.State, error) {
	if err := storage.EnsureSchema(); err != nil {
		return gamestate.State{}, err
	}
	database, err := storage.Open()
	if err != nil {
		return gamestate.State{}, err
	}
	var singleton int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cn_save_snapshot WHERE singleton = 1`).Scan(&singleton); err != nil {
		return gamestate.State{}, fmt.Errorf("read primary CN snapshot: %w", err)
	}
	if singleton == 1 {
		if err := storage.ensureUnboundPrimaryLogin(); err != nil {
			return gamestate.State{}, err
		}
		if err := storage.MigratePrimarySnapshot(); err != nil {
			return gamestate.State{}, err
		}
	}
	var snapshot int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cn_account_snapshot WHERE user_id = ?`, PrimaryUserID).Scan(&snapshot); err != nil {
		return gamestate.State{}, err
	}
	if snapshot == 0 {
		if err := storage.seedPrimaryAccount(); err != nil {
			return gamestate.State{}, err
		}
	}
	return storage.readPrimaryAccount()
}

func (storage *Database) ensureUnboundPrimaryLogin() error {
	database, err := storage.Open()
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = database.Exec(
		`INSERT INTO cn_local_account
		 (user_id, login_uuid, session_key, created_utc, last_login_utc)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO NOTHING`,
		PrimaryUserID,
		unboundPrimaryLoginUUID,
		unboundPrimarySessionKey,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("create primary CN login row: %w", err)
	}
	return nil
}

func (storage *Database) seedPrimaryAccount() error {
	database, err := storage.Open()
	if err != nil {
		return err
	}
	transaction, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin primary CN account seed: %w", err)
	}
	defer transaction.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := transaction.Exec(
		`INSERT INTO cn_local_account
		 (user_id, login_uuid, session_key, created_utc, last_login_utc)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO NOTHING`,
		PrimaryUserID,
		unboundPrimaryLoginUUID,
		unboundPrimarySessionKey,
		now,
		now,
	); err != nil {
		return fmt.Errorf("create primary CN login row: %w", err)
	}
	if err := storage.seedAccountSnapshot(context.Background(), transaction, PrimaryUserID, now); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit primary CN account seed: %w", err)
	}
	if storage.logger != nil {
		storage.logger.Info("initialized CN account in SQLite", "database_path", storage.databasePath)
	}
	return nil
}

func (storage *Database) seedAccountSnapshot(ctx context.Context, transaction *sql.Tx, userID int, now string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state, err := LoadSaveState(storage.seedPath)
	if err != nil {
		return fmt.Errorf("load CN account seed: %w", err)
	}
	if err := InitializeOnboardingSnapshot(&state, userID); err != nil {
		return err
	}
	content, err := EncodeAccountMetadata(state)
	if err != nil {
		return err
	}
	if err := writeAccountRows(transaction, state); err != nil {
		return err
	}
	digest := sha256.Sum256(content)
	if _, err := transaction.ExecContext(ctx,
		`INSERT INTO cn_account_snapshot (user_id, schema_version, revision, updated_utc, payload_json, payload_sha256)
         VALUES (?, ?, 1, ?, ?, ?)`, userID, saveSnapshotSchemaVersion, now, content, hex.EncodeToString(digest[:])); err != nil {
		return fmt.Errorf("create CN account snapshot: %w", err)
	}
	return upsertAccountProjection(transaction, userID, 1, now, state)
}

func (storage *Database) readPrimaryAccount() (gamestate.State, error) {
	database, err := storage.Open()
	if err != nil {
		return gamestate.State{}, err
	}
	transaction, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		return gamestate.State{}, err
	}
	defer transaction.Rollback()
	var schemaVersion int
	var content []byte
	var expectedDigest string
	if err := transaction.QueryRow(
		`SELECT schema_version, payload_json, payload_sha256 FROM cn_account_snapshot WHERE user_id = ?`,
		PrimaryUserID,
	).Scan(&schemaVersion, &content, &expectedDigest); err != nil {
		return gamestate.State{}, fmt.Errorf("read CN account snapshot: %w", err)
	}
	if schemaVersion != saveSnapshotSchemaVersion {
		return gamestate.State{}, fmt.Errorf("CN SQLite snapshot schema_version is %d, want %d", schemaVersion, saveSnapshotSchemaVersion)
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != expectedDigest {
		return gamestate.State{}, errors.New("CN SQLite snapshot digest mismatch")
	}
	state, err := storage.decodeAccount(transaction, content)
	if err != nil {
		return gamestate.State{}, fmt.Errorf("decode CN SQLite account: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return gamestate.State{}, err
	}
	return state, nil
}

func CloneItemShopTabs(source []gamestate.ItemShopTab) []gamestate.ItemShopTab {
	result := make([]gamestate.ItemShopTab, len(source))
	for tabIndex, tab := range source {
		result[tabIndex].TabType = tab.TabType
		result[tabIndex].Lineup = make([]gamestate.ItemShopLineup, len(tab.Lineup))
		for lineupIndex, lineup := range tab.Lineup {
			result[tabIndex].Lineup[lineupIndex] = lineup
			result[tabIndex].Lineup[lineupIndex].Interiors = append(
				[]gamestate.ItemShopInterior(nil),
				lineup.Interiors...,
			)
		}
	}
	return result
}

func cloneGachas(source []gamestate.GachaProfile) []gamestate.GachaProfile {
	return gamestate.CloneGachas(source)
}

func cloneReleaseDeck(deck gamestate.Deck) gamestate.Deck {
	deck.CardUniqueIDs = append([]int64(nil), deck.CardUniqueIDs...)
	deck.SupportCardUniqueIDs = append([]int64(nil), deck.SupportCardUniqueIDs...)
	deck.SphereUniqueIDs = append([]int64(nil), deck.SphereUniqueIDs...)
	deck.BuddyUniqueIDs = append([]int64(nil), deck.BuddyUniqueIDs...)
	return deck
}

func (storage *Database) Persist(state gamestate.State) error {
	return storage.persistWithAudit(state, nil)
}

func (storage *Database) persistWithAudit(state gamestate.State, audit *AdminAudit) error {
	content, err := EncodeAccountMetadata(state)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(content)
	digestText := hex.EncodeToString(digest[:])
	database, err := storage.Open()
	if err != nil {
		return err
	}

	transaction, err := database.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin CN SQLite save: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = transaction.Rollback()
		}
	}()
	updatedUTC := time.Now().UTC().Format(time.RFC3339Nano)
	if err := writeAccountSnapshot(transaction, state.User.UserID, state, content, digestText, updatedUTC, audit); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit CN SQLite save: %w", err)
	}
	committed = true
	return nil
}

// Open returns the single-writer pool, owned by Database. Callers close rows and
// transactions, not the pool; the service owner calls Database.Close after drain.
func (storage *Database) Open() (*sql.DB, error) {
	return storage.openPool(false)
}

// OpenRead returns the separate, read-only pool. Waiting writers must not occupy
// all the connections needed for session checks and other WAL readers.
func (storage *Database) OpenRead() (*sql.DB, error) {
	return storage.openPool(true)
}

func (storage *Database) openPool(readOnly bool) (*sql.DB, error) {
	storage.poolMu.Lock()
	defer storage.poolMu.Unlock()
	if storage.closed {
		return nil, errors.New("CN SQLite database is closed")
	}
	if storage.pool == nil {
		pool, err := storage.newPool(false, 1)
		if err != nil {
			return nil, err
		}
		storage.pool = pool
	}
	if !readOnly {
		return storage.pool, nil
	}
	if storage.readPool == nil {
		limit := storage.readConns
		if limit == 0 {
			limit = defaultSQLiteReadConns
		}
		pool, err := storage.newPool(true, limit)
		if err != nil {
			return nil, err
		}
		storage.readPool = pool
	}
	return storage.readPool, nil
}

func (storage *Database) newPool(readOnly bool, limit int) (*sql.DB, error) {
	// Acquire the writer reservation before taking a read snapshot. A deferred
	// read-then-write transaction can fail with SQLITE_BUSY_SNAPSHOT immediately,
	// even with busy_timeout, when another account commits between the two.
	// modernc keeps explicitly read-only transactions deferred.
	path := filepath.ToSlash(storage.databasePath)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Driver DSN pragmas run for EVERY physical connection, including those
	// created after a broken connection is replaced. Exec on a pool would only
	// configure one arbitrarily selected connection.
	query := url.Values{"_txlock": {"immediate"}, "_pragma": {
		"busy_timeout(5000)", "synchronous(FULL)", "foreign_keys(ON)",
	}}
	if readOnly {
		query.Set("mode", "ro")
		query.Set("_txlock", "deferred")
		query.Add("_pragma", "query_only(ON)")
	} else {
		query.Add("_pragma", "journal_mode(WAL)")
	}
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: query.Encode()}).String()
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open CN SQLite database: %w", err)
	}
	database.SetMaxOpenConns(limit)
	database.SetMaxIdleConns(limit)
	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("configure CN SQLite database: %w", err)
	}
	if !readOnly {
		if err := protectLocalDatabase(storage.databasePath); err != nil {
			_ = database.Close()
			return nil, err
		}
	}
	return database, nil
}

// Close releases all pooled connections after the owner has stopped new work
// and drained active requests. A closed Database cannot silently reopen.
func (storage *Database) Close() error {
	storage.poolMu.Lock()
	defer storage.poolMu.Unlock()
	if storage.closed {
		return nil
	}
	storage.closed = true
	var err error
	if storage.readPool != nil {
		err = storage.readPool.Close()
	}
	if storage.pool != nil {
		err = errors.Join(err, storage.pool.Close())
	}
	return err
}

func protectLocalDatabase(databasePath string) error {
	info, err := os.Stat(databasePath)
	if err != nil {
		return fmt.Errorf("stat CN SQLite database: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("CN SQLite database is not a regular file")
	}
	if err := os.Chmod(databasePath, 0o600); err != nil {
		return fmt.Errorf("protect CN SQLite database: %w", err)
	}
	return nil
}

// setCatalog installs the prepared, immutable master snapshot before serving accounts.
func (storage *Database) SetCatalog(state gamestate.State) { storage.catalog = &state }

func (accounts *Accounts) Database() *Database { return accounts.storage }

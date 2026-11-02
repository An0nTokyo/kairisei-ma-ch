# Unified Account Storage And Android Micro-Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store every player, including user 1000001, in `cn_account_snapshot`, then let an Android app borrow that account and serve solo play through a Go process.

**Architecture:** Phase A and B stay inside `server` and do not import `mserver`. `mserver` is a sibling module that replaces `kairisei.local/server` with `../server`. A Kotlin foreground service starts `kairi-mserver`. Game rules stay in the existing server packages. Unmounted routes share one degrader.

**Tech Stack:** Go 1.25, `modernc.org/sqlite`, existing `accountstore` tests, Kotlin Android foreground service, `GOOS=android GOARCH=arm64`.

## Global Constraints

- Game client changes only the server address.
- `1000001` remains the lowest legal player id. Friend, password, mail, and id allocation bounds stay as they are.
- Login for `1000001` still returns `uid = local-cn-user` and `session = local-cn-session`. `sess_key` and `userid` stay the account's own values.
- Sm does not create accounts and does not serve CPK, patch, or image bodies.
- Friends, real multiplayer, admin, CDN, and registration are not part of Sm.
- Present-id collisions are recorded only and are not handled.
- Schema 3 startup validation stays. `cn_save_snapshot` stays in the database and is not read or written after migration.
- One account has one holder. Stopping Sm does not change the holder.
- Transfer payload is frozen account JSON, balance, cards, items, and presents, plus the master digest. It excludes clear counts, global config, and other accounts.
- Degrader response is HTTP 200, `res_code` `-1`, `res_str` `该功能需要连接电脑上的服务器`. It does not write the account database.
- Device holder id is a UUID created on first app launch and stored in app storage.
- A repeated import of the same payload digest does not increment `revision`.

---

## File map

| File | Responsibility |
| --- | --- |
| `server/internal/accountstore/primary_migration.go` | Copy singleton into `cn_account_snapshot` for user 1000001 |
| `server/internal/accountstore/accounts.go` | Load, save, and register every user through `cn_account_snapshot` |
| `server/internal/accountstore/account_transaction.go` | One write path for a batch of accounts |
| `server/internal/accountstore/battle_history.go` | Solo clear reads and writes `cn_account_snapshot` |
| `server/internal/accountstore/account_projection.go` | Projection revision comes from `cn_account_snapshot` |
| `server/internal/accountstore/admin_queries.go` | Account list and snapshot metadata use `cn_account_snapshot` |
| `server/internal/accountstore/operational_cleanup.go` | Cleanup visits `cn_account_snapshot` only |
| `server/internal/accountstore/holder.go` | Holder row, freeze flag, refuse service |
| `server/internal/accountstore/transfer.go` | Export, import, digest match, idempotent import |
| `server/internal/accountstore/held_mail.go` | Queue settlements while frozen, insert once on return to S |
| `mserver/go.mod` | Module with `replace kairisei.local/server => ../server` |
| `mserver/internal/degrade/degrade.go` | Single unimplemented-route response |
| `mserver/cmd/kairi-mserver/main.go` | Listen on `127.0.0.1`, login URLs, degrader |
| `mserver/android/` | Kotlin page, foreground service, notification |

A and B do not import `mserver`. C can start with no borrowed account. D calls B's export and import. E mounts existing handlers. F stays on the degrader until its routes are mounted. Defense decks for F are stored beside the account database and are not part of the account transaction.

## Phase A — unified account snapshot

### Task 1: Migrate the singleton into the per-account snapshot

**Files:**
- Create: `server/internal/accountstore/primary_migration.go`
- Test: `server/internal/accountstore/primary_migration_test.go`

**Interfaces:**
- Consumes: `OpenDatabase`, `EnsureSchema`, `PrimaryUserID`, `cn_save_snapshot`, `cn_account_snapshot`
- Produces: `func (storage *Database) MigratePrimarySnapshot() error`

- [ ] **Step 1: Write the failing test**

```go
func TestMigratePrimarySnapshotCopiesSingletonAndIgnoresStaleMirror(t *testing.T) {
	storage := openMigrationDatabase(t)
	mustExec(t, storage, `INSERT INTO cn_local_account
		(user_id, login_uuid, session_key, created_utc, last_login_utc)
		VALUES (1000001, 'primary-uuid', 'primary-session', 't', 't')`)
	payload := []byte(`{"user":{"user_id":1000001}}`)
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	mustExec(t, storage, `INSERT INTO cn_save_snapshot
		(singleton, schema_version, revision, updated_utc, payload_json, payload_sha256)
		VALUES (1, 3, 7, 't', ?, ?)`, payload, digest)
	mustExec(t, storage, `INSERT INTO cn_account_snapshot
		(user_id, schema_version, revision, updated_utc, payload_json, payload_sha256)
		VALUES (1000001, 3, 1, 'old', ?, ?)`, []byte(`{"user":{"user_id":1000001},"stale":true}`), digestOf([]byte(`{"user":{"user_id":1000001},"stale":true}`)))

	if err := storage.MigratePrimarySnapshot(); err != nil {
		t.Fatal(err)
	}
	var revision int
	var got []byte
	var gotDigest string
	mustQuery(t, storage, `SELECT revision, payload_json, payload_sha256 FROM cn_account_snapshot WHERE user_id = 1000001`,
		&revision, &got, &gotDigest)
	if revision != 7 || string(got) != string(payload) || gotDigest != digest {
		t.Fatalf("migrated snapshot = revision %d body %s digest %s", revision, got, gotDigest)
	}
}
```

Add a second test in the same file: corrupt `payload_sha256` on the singleton, call `MigratePrimarySnapshot`, and assert the error contains `digest mismatch` and `cn_account_snapshot` is unchanged.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestMigratePrimarySnapshot`
Expected: FAIL with `MigratePrimarySnapshot` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
func (storage *Database) MigratePrimarySnapshot() error {
	if err := storage.EnsureSchema(); err != nil {
		return err
	}
	database, err := storage.Open()
	if err != nil {
		return err
	}
	transaction, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var revision int
	var updated string
	var payload []byte
	var digestText string
	err = transaction.QueryRow(`SELECT revision, updated_utc, payload_json, payload_sha256
		FROM cn_save_snapshot WHERE singleton = 1`).Scan(&revision, &updated, &payload, &digestText)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read primary CN snapshot: %w", err)
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != digestText {
		return errors.New("primary CN account snapshot digest mismatch")
	}
	if _, err := transaction.Exec(`INSERT INTO cn_account_snapshot
		(user_id, schema_version, revision, updated_utc, payload_json, payload_sha256)
		VALUES (?, 3, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
		 schema_version = excluded.schema_version,
		 revision = excluded.revision,
		 updated_utc = excluded.updated_utc,
		 payload_json = excluded.payload_json,
		 payload_sha256 = excluded.payload_sha256`,
		PrimaryUserID, revision, updated, payload, digestText); err != nil {
		return fmt.Errorf("copy primary CN snapshot: %w", err)
	}
	return transaction.Commit()
}
```

`ON CONFLICT` replaces a stale mirror. The singleton row is not deleted.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestMigratePrimarySnapshot`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/accountstore/primary_migration.go server/internal/accountstore/primary_migration_test.go
git commit -m "Copy the primary snapshot into the per-account table"
```

### Task 2: Load and save user 1000001 from cn_account_snapshot

**Files:**
- Modify: `server/internal/accountstore/accounts.go` (`LoadPersistentState`, `persistAccountData`)
- Modify: `server/internal/accountstore/account_transaction.go`
- Modify: `server/internal/accountstore/battle_history.go` (`PersistSoloClear`)
- Test: `server/internal/accountstore/primary_migration_test.go`

**Interfaces:**
- Consumes: `MigratePrimarySnapshot`, `writeAccountSnapshot`
- Produces: `LoadPersistentState(1000001)` and `PersistState(1000001, state)` use `cn_account_snapshot`

- [ ] **Step 1: Write the failing test**

Extend the migration test file. After `MigratePrimarySnapshot`, call `LoadPersistentState(PrimaryUserID)` and assert `state.User.UserID == 1000001`. Change gold, call `PersistState`, reload, and assert gold changed. Query `cn_save_snapshot.payload_json` and assert it is still the pre-save bytes.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestPrimaryAccountRoundTripUsesAccountSnapshot`
Expected: FAIL because load still uses `LoadOrImport` and save still updates `cn_save_snapshot`.

- [ ] **Step 3: Write minimal implementation**

In `LoadPersistentState`, delete the `userID == PrimaryUserID` branch that calls `LoadOrImport`. Call `MigratePrimarySnapshot` once before the shared `cn_account_snapshot` read.

In `persistAccountData`, delete the `userID == PrimaryUserID` branch that calls `persistWithAudit`. Use the existing `writeAccountSnapshot` path for every user id.

In `account_transaction.go`, delete the `writePrimaryAccountSnapshot` branch. Call `writeAccountSnapshot` for every state.

In `PersistSoloClear`, read and write `cn_account_snapshot` for every user, including `1000001`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server; go test ./internal/accountstore/ ./internal/cnbootstrap/ ./internal/admin/ -count=1`
Expected: PASS, or failures only in tests that still SELECT progress from `cn_save_snapshot`. Update those SELECTs to `cn_account_snapshot WHERE user_id = 1000001`. Known sites: `operational_cleanup.go`, `operational_cleanup_external_test.go`, `audit_cleanup_external_test.go`, `admin/admin_account_audit_test.go`, `account_projection.go`, `admin_queries.go`.

- [ ] **Step 5: Commit**

```bash
git add server/internal/accountstore server/internal/admin/admin_account_audit_test.go
git commit -m "Load and save the primary account from the per-account snapshot"
```

### Task 3: Create the first account without a singleton

**Files:**
- Modify: `server/internal/accountstore/accounts.go` (`resolveLoginTransaction`)
- Modify: `server/internal/accountstore/save_sqlite.go` (`LoadOrImport`)
- Modify: `server/internal/cnbootstrap/bootstrap.go`
- Test: `server/internal/cnbootstrap/accounts_test.go`

**Interfaces:**
- Consumes: `insertSeedSnapshot`, `MigratePrimarySnapshot`
- Produces: a new database has no `cn_save_snapshot` row; the first login inserts user 1000001 through `insertSeedSnapshot`

- [ ] **Step 1: Write the failing test**

In `accounts_test.go`, open a fresh database, call the startup load used by bootstrap, and assert `SELECT COUNT(*) FROM cn_save_snapshot` is 0. Resolve a new login UUID and assert `UserID == 1000001` and `cn_account_snapshot` has that user. Resolve a second UUID and assert `UserID == 1000002`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server; go test ./internal/cnbootstrap/ -count=1 -run TestFirstAccountUsesAccountSnapshot`
Expected: FAIL because the first account still attaches the singleton.

- [ ] **Step 3: Write minimal implementation**

Startup calls `MigratePrimarySnapshot` and, when no `cn_account_snapshot` row exists for `1000001`, inserts the seed through `insertSeedSnapshot` inside a transaction that also inserts `cn_local_account` only if that login row is absent. `LoadOrImport` stops inserting `cn_save_snapshot`.

`resolveLoginTransaction` always calls `insertSeedSnapshot` for a new user, including `1000001`.

```go
if err := accounts.insertSeedSnapshot(ctx, transaction, identity.UserID, now); err != nil {
	return accountIdentity{}, err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server; go test ./internal/accountstore/ ./internal/cnbootstrap/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/accountstore server/internal/cnbootstrap/bootstrap.go server/internal/cnbootstrap/accounts_test.go
git commit -m "Create the first account in the per-account snapshot"
```

## Phase B — holder, freeze, transfer, mail queue

### Task 4: Holder and freeze

**Files:**
- Create: `server/internal/accountstore/holder.go`
- Test: `server/internal/accountstore/holder_test.go`
- Modify: `server/internal/accountstore/database_schema.go` (additive table only; schema version stays 3)

**Interfaces:**
- Consumes: `PrimaryUserID`
- Produces:
  - `type Holder struct { UserID int; HolderID string; CheckoutRevision int; MasterDigest string; Frozen bool }`
  - `func (accounts *Accounts) PutHolder(holder Holder) error`
  - `func (accounts *Accounts) Holder(userID int) (Holder, error)`
  - `func (accounts *Accounts) RefuseIfNotHolder(userID int, holderID string) error`

- [ ] **Step 1: Write the failing test**

Create the table in the test through `EnsureSchema`. `PutHolder` for user 1000001 and holder `device-a`. `RefuseIfNotHolder(1000001, "device-b")` returns an error containing `holder`. `RefuseIfNotHolder(1000001, "device-a")` returns nil while `Frozen` is false. Set `Frozen` true and assert `device-a` is also refused.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestHolderRefusesNonOwner`
Expected: FAIL with `PutHolder` undefined.

- [ ] **Step 3: Write minimal implementation**

Add table `cn_account_holder (user_id INTEGER PRIMARY KEY, holder_id TEXT NOT NULL, checkout_revision INTEGER NOT NULL, master_digest TEXT NOT NULL, frozen INTEGER NOT NULL CHECK (frozen IN (0,1)))`. Include it in `fixedStateTableColumns`. `RefuseIfNotHolder` returns an error when the row is missing, the holder id differs, or `frozen` is 1.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestHolderRefusesNonOwner`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/accountstore/holder.go server/internal/accountstore/holder_test.go server/internal/accountstore/database_schema.go
git commit -m "Record which server holds each account"
```

### Task 5: Transactional export and idempotent import

**Files:**
- Create: `server/internal/accountstore/transfer.go`
- Test: `server/internal/accountstore/transfer_test.go`

**Interfaces:**
- Consumes: `LoadPersistentState`, `writeAccountRows`, `writeAccountSnapshot`, `Holder`
- Produces:
  - `type AccountTransfer struct { UserID int; Revision int; Metadata []byte; MetadataDigest string; MasterDigest string }`
  - `func (accounts *Accounts) ExportAccount(userID int) (AccountTransfer, error)`
  - `func (accounts *Accounts) ImportAccount(transfer AccountTransfer) error`

- [ ] **Step 1: Write the failing test**

Export user 1000002. Drop that user's card row. Import the same payload twice. Assert gold, card count, and `revision` match the exported values after both imports. Force `writeAccountRows` to fail with a test hook or a closed database transaction and assert `Holder` is unchanged and the old card row remains. Assert the exported struct has no field named clear count and the import does not execute `INSERT INTO cn_battle_clear_daily`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestImportAccountIsIdempotent`
Expected: FAIL with `ExportAccount` undefined.

- [ ] **Step 3: Write minimal implementation**

`ExportAccount` reads `cn_account_snapshot` for that user only. `ImportAccount` runs one transaction: `writeAccountRows`, then update `cn_account_snapshot` and the projection. If `payload_sha256` already equals `MetadataDigest`, commit without `revision = revision + 1`. Do not read or write `cn_battle_clear_daily`. Do not change `cn_account_holder` in this function.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestImportAccount`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/accountstore/transfer.go server/internal/accountstore/transfer_test.go
git commit -m "Import a frozen account in one transaction"
```

### Task 6: Held mail inserted once when S owns the account again

**Files:**
- Create: `server/internal/accountstore/held_mail.go`
- Test: `server/internal/accountstore/held_mail_test.go`
- Modify: `server/internal/accountstore/database_schema.go`

**Interfaces:**
- Consumes: `ImportAccount`, `Holder`
- Produces:
  - `func (accounts *Accounts) EnqueueHeldMail(userID int, rewardJSON []byte, eventKey string) error`
  - `func (accounts *Accounts) DeliverHeldMail(userID int) error`

- [ ] **Step 1: Write the failing test**

Enqueue two mails with different `eventKey` values while the holder is not the local server id `s`. Call `DeliverHeldMail` twice. Assert the account has two presents and the queue is empty. Enqueue the same `eventKey` again and assert it does not add a third present.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestDeliverHeldMailOnce`
Expected: FAIL with `EnqueueHeldMail` undefined.

- [ ] **Step 3: Write minimal implementation**

Table `cn_account_held_mail (user_id INTEGER NOT NULL, event_key TEXT NOT NULL, reward_json TEXT NOT NULL, PRIMARY KEY(user_id, event_key))`. `DeliverHeldMail` inserts presents with S's existing present id allocation, then deletes the queue rows, in the same transaction as the account snapshot update. Calling it again sees an empty queue.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestDeliverHeldMailOnce`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/accountstore/held_mail.go server/internal/accountstore/held_mail_test.go server/internal/accountstore/database_schema.go
git commit -m "Queue settlements for a frozen account"
```

### Task 7: Master digest gate

**Files:**
- Create: `server/internal/accountstore/master_digest.go`
- Test: `server/internal/accountstore/master_digest_test.go`

**Interfaces:**
- Consumes: card template JSON and growth table bytes already loaded by `CatalogState`
- Produces: `func MasterDigest(cardTemplates, growthTables []byte) string` as hex SHA-256 of `len(cardTemplates)` + bytes + `len(growthTables)` + bytes

- [ ] **Step 1: Write the failing test**

`MasterDigest([]byte("cards"), []byte("growth"))` is stable. Changing either input changes the digest. A transfer whose `MasterDigest` differs from the caller's digest returns an error containing `master digest` and does not call `ImportAccount`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestMasterDigest`
Expected: FAIL with `MasterDigest` undefined.

- [ ] **Step 3: Write minimal implementation**

Hash the two byte slices with explicit lengths, matching `resourceVersionNamespace` style. Compare digests before `ImportAccount`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server; go test ./internal/accountstore/ -count=1 -run TestMasterDigest`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/accountstore/master_digest.go server/internal/accountstore/master_digest_test.go
git commit -m "Refuse account transfer when card masters differ"
```

## Phase C — superseded

Tasks 8–10 below described a loopback listener and a fixed advertise host. Implement Phase C from `docs/superpowers/plans/2026-10-17-mserver-phase-c.md` instead. Do not implement the steps in Tasks 8–10.

## Phase C — mserver process and degrader

### Task 8: Degrader does not write

**Files:**
- Create: `mserver/go.mod`
- Create: `mserver/internal/degrade/degrade.go`
- Create: `mserver/internal/degrade/degrade_test.go`

**Interfaces:**
- Consumes: none from `server`
- Produces: `func Handler() http.Handler`

- [ ] **Step 1: Write the failing test**

Request `POST /FriendSearch` with any body. Assert status 200, JSON `res_code` is `-1`, `res_str` is `该功能需要连接电脑上的服务器`. Pass a database pointer that fails if `Exec` is called and assert it was not called.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd mserver; go test ./internal/degrade/ -count=1`
Expected: FAIL because the module or package does not exist.

- [ ] **Step 3: Write minimal implementation**

`mserver/go.mod`:

```go
module kairisei.local/mserver

go 1.25.0

require kairisei.local/server v0.0.0
replace kairisei.local/server => ../server
```

`Handler` writes the fixed JSON and does not touch a database.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd mserver; go test ./internal/degrade/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/go.mod mserver/internal/degrade
git commit -m "Reject unimplemented micro-server routes without writing"
```

### Task 9: Loopback process advertises itself

**Files:**
- Create: `mserver/cmd/kairi-mserver/main.go`
- Test: `mserver/cmd/kairi-mserver/main_test.go`

**Interfaces:**
- Consumes: `degrade.Handler`
- Produces: process flags `-port` (default `26020`) and `-advertise-host` fixed to `127.0.0.1` in Sm

- [ ] **Step 1: Write the failing test**

Start the handler on `127.0.0.1:0`. `POST /loginSDK.php` is not mounted yet, so it uses the degrader. `GET /local/resources/cpk/example.cpk` also uses the degrader and does not open a file.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd mserver; go test ./cmd/kairi-mserver/ -count=1 -run TestResourceBodyIsDegraded`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

Listen only on `127.0.0.1`. Mount the degrader as the default route. Do not register CPK, patch, or image file handlers.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd mserver; go test ./cmd/kairi-mserver/ -count=1 -run TestResourceBodyIsDegraded`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/cmd/kairi-mserver
git commit -m "Listen on loopback and degrade resource bodies"
```

### Task 10: Android foreground service starts and stops the process

**Files:**
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt`
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/ServerService.kt`
- Create: `mserver/android/app/src/main/AndroidManifest.xml`

**Interfaces:**
- Consumes: `kairi-mserver -port`
- Produces: a foreground notification while the process is running; stop destroys the process and does not call export or import

- [ ] **Step 1: Write the service**

`ServerService` is a foreground service. `onStartCommand` starts the bundled `kairi-mserver` with the saved local port and shows a persistent notification. `onDestroy` destroys that process. It does not open a network connection to S.

`MainActivity` saves S host, S port, and local port, and has Start and Stop buttons. Stop calls `stopService`. On first launch, if `files/device-id` is missing, write a new UUID there and reuse it as `HolderID`.

- [ ] **Step 2: Build the Android project**

Run the project's Gradle assemble for `arm64`. Expected: APK builds. The Go binary is built with `CGO_ENABLED=0 GOOS=android GOARCH=arm64` and packaged where `ServerService` executes it.

- [ ] **Step 3: Commit**

```bash
git add mserver/android
git commit -m "Add the Android page and foreground service for the micro-server"
```

## Phase D — borrow and return

### Task 11: Borrow and return commands

**Files:**
- Create: `mserver/internal/sync/client.go`
- Test: `mserver/internal/sync/client_test.go`
- Modify: `server` HTTP or local admin route that calls `ExportAccount`, `ImportAccount`, `PutHolder`, and `DeliverHeldMail`

**Interfaces:**
- Consumes: `ExportAccount`, `ImportAccount`, `MasterDigest`, `PutHolder`, `DeliverHeldMail`
- Produces: `func Borrow(sAddress string, userID int, deviceID string) error` and `func Return(choice string) error` where `choice` is `phone` or `server`

- [ ] **Step 1: Write the failing test**

Use `httptest` as S. Borrow refuses when the master digest differs. Borrow with a matching digest stores the payload locally and S's holder becomes the device id. Return with the same digest imports once and S's holder becomes `s`. Change S's revision before return and assert the client returns `choice required` without importing. `Return("phone")` then imports. `Return("server")` does not import and still sets the holder to `s`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd mserver; go test ./internal/sync/ -count=1 -run TestBorrowAndReturn`
Expected: FAIL with `Borrow` undefined.

- [ ] **Step 3: Write minimal implementation**

S exposes borrow and return on the existing server, protected so only a configured sync token can call them. Borrow sets `Frozen` true, exports, imports on the phone, then `PutHolder` for the device. Return to S sets `Frozen` true, imports only for `choice=phone`, `DeliverHeldMail`, then `PutHolder` for `s` and clears `Frozen`. The Android page sends the token only to the configured S address. After borrow, Sm's `POST /loginSDK.php` for that account's UUID returns `api_url` on `127.0.0.1` and the saved resource version. An unknown UUID gets the degrader response and does not insert `cn_local_account`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd mserver; go test ./internal/sync/ -count=1 -run TestBorrowAndReturn`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/internal/sync server/internal/accountstore
git commit -m "Borrow an account to the phone and return it"
```

## Phase E — solo routes

### Task 12: Mount solo handlers behind the holder check

**Files:**
- Create: `mserver/internal/solo/routes.go`
- Test: `mserver/internal/solo/routes_test.go`

**Interfaces:**
- Consumes: existing CN handlers for `HomeShow`, card, story, explore, `TeamBattleSoloStart`, `TeamBattleSoloEnd`, gacha, shop, mission, and present routes
- Produces: those paths write the local account; `FriendSearch` and `TeamBattleMultiRoomCreate` stay on `degrade.Handler`

- [ ] **Step 1: Write the failing test**

With a borrowed fixture account, `POST /HomeShow` persists a session change to the local database. `POST /FriendSearch` returns `res_code` `-1` and does not change `revision`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd mserver; go test ./internal/solo/ -count=1 -run TestSoloRouteWritesAndFriendRouteDoesNot`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

Register the solo paths from `server/internal/cnbootstrap/game_routes.go` for home, card, story, explore, solo battle, gacha, shop, mission, and present. Leave every other path on the degrader. Check the holder before the handler runs.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd mserver; go test ./internal/solo/ -count=1 -run TestSoloRouteWritesAndFriendRouteDoesNot`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/internal/solo
git commit -m "Serve borrowed solo play and degrade other routes"
```

## Phase F — stays degraded until mounted

### Task 13: AI room and frozen defense decks

**Files:**
- Create: `mserver/internal/solo/defense.go`
- Test: `mserver/internal/solo/defense_test.go`

**Interfaces:**
- Consumes: borrow response field `defense_decks`, stored outside `AccountTransfer`
- Produces: `PvpStart2` uses those decks when the file exists; otherwise the degrader handles `PvpStart2` and `TeamBattleAIRoomCreate`

- [ ] **Step 1: Write the failing test**

Without `defense_decks`, `POST /PvpStart2` returns `res_code` `-1` and does not write. With a fixture file, the same route does not call the degrader.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd mserver; go test ./internal/solo/ -count=1 -run TestPvpWithoutDefenseDecksIsDegraded`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

Borrow saves `defense_decks` next to the account database. Mount `PvpStart2` and `TeamBattleAIRoomCreate` only when that file exists. Do not add the file to `AccountTransfer` or `cn_battle_clear_daily`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd mserver; go test ./internal/solo/ -count=1 -run 'TestPvp|TestAI'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/internal/solo/defense.go mserver/internal/solo/defense_test.go
git commit -m "Serve local AI rooms and frozen defense decks when they were borrowed"
```

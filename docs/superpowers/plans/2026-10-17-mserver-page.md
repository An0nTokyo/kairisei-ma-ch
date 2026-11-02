# Phone Page And Account List Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show the three service states, a separate transfer page, and a pinned log strip, backed by a read-only account list from the computer.

**Architecture:** The computer adds `POST /local/sync/accounts` on the existing sync token handler. The phone's Go process exposes `POST /local/launcher/accounts` on loopback and writes transfer results to stdout, which already lands in `server.log`. Kotlin decides search, row color, button text, refresh, and the three home states, and appends a log line only when the request never reaches Go.

**Tech Stack:** Go 1.25, `modernc.org/sqlite`, existing `accountsync` token handler, Kotlin Android UI built in code, JUnit 4. No new Android libraries.

## Global Constraints

- Borrow, return, snapshot choice, and holder rules stay as they are.
- `POST /local/sync/accounts` uses `X-Account-Sync-Token`. An empty or wrong token returns 404. An empty `device_id` returns 400 with `account sync body is invalid`.
- The list is read-only. It does not change the holder and does not write the save.
- Player ids run from `1000001` up to, but not including, `1900000000`.
- `state` is `server`, `mine`, or `other`. Missing holder, or holder `s` while not frozen, is `server`. This device id while not frozen is `mine`. Everything else, including frozen, is `other`.
- Display name is projection `user.name`. An empty name stays an empty string on the wire. The page shows `未命名`.
- The phone route accepts only loopback. A non-loopback request returns 404. An illegal computer address, port, or token returns 400 and does not connect.
- A computer 404 becomes the existing text `account sync failed`.
- Go writes the transfer log lines. Android appends a line only for `电脑地址、端口或口令无效` or `没有连上本机服务`.
- Do not stage unrelated dirty files in this worktree.
- On this machine, clear `GOOS`, `GOARCH`, and `CGO_ENABLED` before a host `go test`. PowerShell does not accept `&&`.

---

## File map

| File | Responsibility |
| --- | --- |
| `server/internal/accountstore/sync_roster.go` | Read player id, display name, and holder, then assign `state` |
| `server/accountsync/sync.go` | `AccountBrief`, `DB.AccountRoster`, `POST /local/sync/accounts`, `ListAccounts` |
| `server/internal/cnbootstrap/account_sync.go` | Adapt the account-store read onto `accountsync.DB` |
| `server/internal/cnbootstrap/router.go` | Mount the new sync path next to borrow and return |
| `mserver/internal/server/sync_log.go` | The exact log sentences |
| `mserver/internal/sync/client.go` | Phone-side `List` wrapper |
| `mserver/internal/server/sync_http.go` | Loopback list route and log calls around borrow and return |
| `mserver/android/.../PageRules.kt` | Home state, search, button, refresh, and the three-line tail |

---

### Task 1: Read-only account roster on the computer

**Files:**
- Create: `server/internal/accountstore/sync_roster.go`
- Test: `server/internal/accountstore/sync_roster_test.go`
- Modify: `server/accountsync/sync.go`
- Modify: `server/accountsync/sync_test.go`
- Modify: `server/internal/cnbootstrap/account_sync.go`
- Modify: `server/internal/cnbootstrap/router.go`

**Interfaces:**
- Consumes: `accountstore.PrimaryUserID`, `accountstore.SystemPartnerUserIDBase`, `Accounts.PutHolder`, `EncodeAccountProjection`
- Produces: `func (accounts *Accounts) SyncRoster(deviceID string) ([]SyncAccount, error)` with `SyncAccount{UserID int; Name string; State string}`; `accountsync.AccountBrief`; `DB.AccountRoster(deviceID string) ([]AccountBrief, error)`; `func ListAccounts(baseURL, token, deviceID string) ([]AccountBrief, error)`

- [ ] **Step 1: Write the failing roster test**

`server/internal/accountstore/sync_roster_test.go`:

```go
package accountstore

import (
	"database/sql"
	"strconv"
	"testing"

	"kairisei.local/server/internal/gamestate"
)

func TestSyncRosterClassifiesHoldersWithoutWriting(t *testing.T) {
	storage := openMigrationDatabase(t)
	accounts, err := NewAccounts(storage)
	if err != nil {
		t.Fatal(err)
	}
	database, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	insert := func(id int, name string) {
		t.Helper()
		if _, err := database.Exec(`INSERT INTO cn_local_account
			(user_id, login_uuid, session_key, created_utc, last_login_utc)
			VALUES (?, ?, ?, 't', 't')`, id, "uuid-"+strconv.Itoa(id), "session-"+strconv.Itoa(id)); err != nil {
			t.Fatal(err)
		}
		if name == "" {
			return
		}
		payload, digest, err := EncodeAccountProjection(gamestate.State{User: gamestate.User{UserID: id, Name: name}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`INSERT INTO cn_account_projection
			(user_id, schema_version, snapshot_revision, updated_utc, payload_json, payload_sha256)
			VALUES (?, 2, 1, 't', ?, ?)`, id, payload, digest); err != nil {
			t.Fatal(err)
		}
	}
	insert(1000008, "秋叶")
	insert(1000001, "秋")
	insert(1000002, "另一名")
	insert(1000003, "别人")
	insert(1000004, "冻结")
	insert(1000005, "电脑")
	insert(1000006, "")
	insert(SystemPartnerUserIDBase, "伙伴")
	if err := accounts.PutHolder(Holder{UserID: 1000002, HolderID: "device-a"}); err != nil {
		t.Fatal(err)
	}
	if err := accounts.PutHolder(Holder{UserID: 1000003, HolderID: "device-b"}); err != nil {
		t.Fatal(err)
	}
	if err := accounts.PutHolder(Holder{UserID: 1000004, HolderID: "s", Frozen: true}); err != nil {
		t.Fatal(err)
	}
	if err := accounts.PutHolder(Holder{UserID: 1000005, HolderID: "s"}); err != nil {
		t.Fatal(err)
	}
	before := holderSnapshot(t, database, 1000002)

	rows, err := accounts.SyncRoster("device-a")
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]SyncAccount{}
	var order []int
	for _, row := range rows {
		got[row.UserID] = row
		order = append(order, row.UserID)
	}
	if _, ok := got[SystemPartnerUserIDBase]; ok {
		t.Fatal("system partner was listed")
	}
	wantOrder := []int{1000001, 1000002, 1000003, 1000004, 1000005, 1000006, 1000008}
	if len(order) != len(wantOrder) {
		t.Fatalf("order = %v", order)
	}
	for i, id := range wantOrder {
		if order[i] != id {
			t.Fatalf("order = %v", order)
		}
	}
	checks := []struct {
		id, name, state string
		user            int
	}{
		{user: 1000001, name: "秋", state: SyncStateServer},
		{user: 1000002, name: "另一名", state: SyncStateMine},
		{user: 1000003, name: "别人", state: SyncStateOther},
		{user: 1000004, name: "冻结", state: SyncStateOther},
		{user: 1000005, name: "电脑", state: SyncStateServer},
		{user: 1000006, name: "", state: SyncStateServer},
	}
	for _, check := range checks {
		row := got[check.user]
		if row.Name != check.name || row.State != check.state {
			t.Fatalf("user %d = %+v", check.user, row)
		}
	}
	if after := holderSnapshot(t, database, 1000002); after != before {
		t.Fatalf("holder changed from %s to %s", before, after)
	}
}

func holderSnapshot(t *testing.T, database *sql.DB, userID int) string {
	t.Helper()
	var holder, digest string
	var revision, frozen int
	err := database.QueryRow(`SELECT holder_id, checkout_revision, master_digest, frozen
		FROM cn_account_holder WHERE user_id = ?`, userID).Scan(&holder, &revision, &digest, &frozen)
	if err != nil {
		t.Fatal(err)
	}
	return holder + " " + strconv.Itoa(revision) + " " + digest + " " + strconv.Itoa(frozen)
}
```

Import `database/sql`, `strconv`, and `kairisei.local/server/internal/gamestate`. `session_key` is unique, so each row uses its own session. Projection columns are `user_id`, `schema_version`, `snapshot_revision`, `updated_utc`, `payload_json`, `payload_sha256`.

- [ ] **Step 2: Run the roster test to verify it fails**

```powershell
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
go test ./internal/accountstore/ -run TestSyncRosterClassifiesHoldersWithoutWriting -count=1
```

Working directory: `server`

Expected: FAIL because `SyncRoster` is undefined.

- [ ] **Step 3: Implement the roster and the sync route**

`server/internal/accountstore/sync_roster.go`:

```go
package accountstore

import (
	"context"
	"database/sql"
	"fmt"
)

const (
	SyncStateServer = "server"
	SyncStateMine   = "mine"
	SyncStateOther  = "other"
)

type SyncAccount struct {
	UserID int
	Name   string
	State  string
}

func (accounts *Accounts) SyncRoster(deviceID string) ([]SyncAccount, error) {
	database, err := accounts.storage.OpenRead()
	if err != nil {
		return nil, err
	}
	rows, err := database.QueryContext(context.Background(), `
		SELECT a.user_id,
		       COALESCE(json_extract(CAST(p.payload_json AS TEXT), '$.user.name'), ''),
		       h.holder_id,
		       h.frozen
		FROM cn_local_account a
		LEFT JOIN cn_account_projection p ON p.user_id = a.user_id
		LEFT JOIN cn_account_holder h ON h.user_id = a.user_id
		WHERE a.user_id >= ? AND a.user_id < ?
		ORDER BY a.user_id`, PrimaryUserID, SystemPartnerUserIDBase)
	if err != nil {
		return nil, fmt.Errorf("list CN sync accounts: %w", err)
	}
	defer rows.Close()
	var out []SyncAccount
	for rows.Next() {
		var row SyncAccount
		var holder sql.NullString
		var frozen sql.NullInt64
		if err := rows.Scan(&row.UserID, &row.Name, &holder, &frozen); err != nil {
			return nil, fmt.Errorf("list CN sync accounts: %w", err)
		}
		row.State = syncRosterState(deviceID, holder.String, holder.Valid, frozen.Int64 == 1)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list CN sync accounts: %w", err)
	}
	if out == nil {
		out = []SyncAccount{}
	}
	return out, nil
}

func syncRosterState(deviceID, holderID string, hasHolder, frozen bool) string {
	if !hasHolder || (holderID == "s" && !frozen) {
		return SyncStateServer
	}
	if holderID == deviceID && !frozen {
		return SyncStateMine
	}
	return SyncStateOther
}
```

Add to `accountsync.DB` in `server/accountsync/sync.go`:

```go
AccountRoster(deviceID string) ([]AccountBrief, error)
```

Add the type next to `HolderView`:

```go
type AccountBrief struct {
	UserID int    `json:"user_id"`
	Name   string `json:"name"`
	State  string `json:"state"`
}
```

In `Handler`, add this case before `default`:

```go
case "/local/sync/accounts":
    var payload struct {
        DeviceID string `json:"device_id"`
    }
    if err := json.Unmarshal(body, &payload); err != nil || payload.DeviceID == "" {
        writeError(writer, http.StatusBadRequest, "account sync body is invalid")
        return
    }
    accounts, err := service.DB.AccountRoster(payload.DeviceID)
    if err != nil {
        writeError(writer, http.StatusBadRequest, err.Error())
        return
    }
    if accounts == nil {
        accounts = []AccountBrief{}
    }
    writeJSON(writer, http.StatusOK, map[string]any{"accounts": accounts})
```

Add the client at the end of `sync.go`:

```go
func ListAccounts(baseURL, token, deviceID string) ([]AccountBrief, error) {
    var result struct {
        Accounts []AccountBrief `json:"accounts"`
    }
    if err := post(baseURL, token, "/local/sync/accounts", map[string]any{
        "device_id": deviceID,
    }, &result, nil); err != nil {
        return nil, err
    }
    if result.Accounts == nil {
        result.Accounts = []AccountBrief{}
    }
    return result.Accounts, nil
}
```

`serverDB` in `server/accountsync/sync_test.go` must implement the new method or the package will not compile. Add the field and method used by `TestAccountRosterRoute`:

```go
func (db serverDB) AccountRoster(string) ([]accountsync.AccountBrief, error) {
	return db.roster, nil
}
```

`server/internal/cnbootstrap/account_sync.go`:

```go
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
```

In `server/internal/cnbootstrap/router.go`, add `"/local/sync/accounts"` to the existing sync path slice.

Add `roster []accountsync.AccountBrief` and `writes *int` to `serverDB`. The existing methods use a value receiver, so an `int` field would not be visible to the test. Increment `*db.writes` at the start of `SaveHolder` and `Import` when `writes` is non-nil.

```go
func TestAccountRosterRoute(t *testing.T) {
	writes := 0
	db := serverDB{
		roster: []accountsync.AccountBrief{{UserID: 1000001, Name: "秋", State: "server"}},
		writes: &writes,
	}
	handler := accountsync.Handler("secret", db)
	post := func(token, body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/local/sync/accounts", strings.NewReader(body))
		if token != "" {
			request.Header.Set("X-Account-Sync-Token", token)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}
	missing := post("secret", `{}`)
	if missing.Code != http.StatusBadRequest || !strings.Contains(missing.Body.String(), "account sync body is invalid") {
		t.Fatalf("empty device = %d %s", missing.Code, missing.Body.String())
	}
	for _, token := range []string{"", "nope"} {
		got := post(token, `{"device_id":"device-a"}`)
		if got.Code != http.StatusNotFound {
			t.Fatalf("token %q status = %d", token, got.Code)
		}
	}
	ok := post("secret", `{"device_id":"device-a"}`)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"user_id":1000001`) || !strings.Contains(ok.Body.String(), `"state":"server"`) {
		t.Fatalf("roster = %d %s", ok.Code, ok.Body.String())
	}
	if writes != 0 {
		t.Fatalf("writes = %d", writes)
	}
}
```

- [ ] **Step 4: Run the roster tests**

```powershell
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
go test ./internal/accountstore/ ./accountsync/ -run "TestSyncRosterClassifiesHoldersWithoutWriting|TestAccountRosterRoute|TestBorrowAndReturn|TestExtraMasterNameIsRejected" -count=1
```

Working directory: `server`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- server/internal/accountstore/sync_roster.go server/internal/accountstore/sync_roster_test.go server/accountsync/sync.go server/accountsync/sync_test.go server/internal/cnbootstrap/account_sync.go server/internal/cnbootstrap/router.go
git commit -m @"
List sync accounts without changing holders.
"@
```

Do not stage the other dirty files in `router.go` if a backup-and-restore split is required. The commit may contain only the new path string inside the sync slice. If `router.go` also has unrelated startup edits, copy the file aside, restore HEAD, reapply only the path slice line, commit, then restore the aside copy. That is the same split used for earlier sync commits.

---

### Task 2: Phone list route and transfer log lines

**Files:**
- Create: `mserver/internal/server/sync_log.go`
- Test: `mserver/internal/server/sync_log_test.go`
- Test: `mserver/internal/server/sync_http_test.go`
- Modify: `mserver/internal/sync/client.go`
- Modify: `mserver/internal/server/sync_http.go`
- Modify: `mserver/internal/server/server.go`

**Interfaces:**
- Consumes: `accountsync.ListAccounts`, `accountsync.AccountBrief`, `accountsync.ChoiceRequiredError`, existing `serveAccountSync`
- Produces: `sync.List(baseURL, token, deviceID string) ([]accountsync.AccountBrief, error)`; `POST /local/launcher/accounts`; `logList` and `logTransfer`

- [ ] **Step 1: Write the failing log and route tests**

`mserver/internal/server/sync_log.go` is created in Step 3. First write `mserver/internal/server/sync_log_test.go`:

```go
package server

import (
	"errors"
	"strings"
	"testing"

	"kairisei.local/server/accountsync"
)

func TestTransferLogLines(t *testing.T) {
	var lines []string
	write := func(line string) { lines = append(lines, line) }

	logList(write, 0, nil)
	logList(write, 2, nil)
	logList(write, 0, errors.New("account sync failed"))
	logTransfer(write, "借出", 1000001, nil, nil)
	logTransfer(write, "交还", 1000001, errors.New("refused"), nil)
	logTransfer(write, "交还", 1000001, nil, &accountsync.ChoiceRequiredError{ServerRevision: 4, PhoneRevision: 9})
	logTransfer(write, "保留手机进度", 1000001, nil, nil)
	logTransfer(write, "保留电脑进度", 1000002, errors.New("refused"), nil)

	text := strings.Join(lines, "\n")
	for _, want := range []string{
		"获取账号列表\n账号列表 0 个",
		"获取账号列表\n账号列表 2 个",
		"账号列表失败: account sync failed",
		"借出 1000001\n完成 1000001",
		"交还 1000001\n失败 1000001: refused",
		"交还 1000001\n需要选择进度 1000001: 电脑 4，手机 9",
		"保留手机进度 1000001\n完成 1000001",
		"保留电脑进度 1000002\n失败 1000002: refused",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(lines[4], "获取账号列表") {
		t.Fatal("a failed list also wrote the success header")
	}
}
```

`lines[4]` is the failed-list call, the third `logList`. Count the appended lines: success 0 adds 2, success 2 adds 2, failure adds 1, so the failure line is index 4. Assert `lines[4] == "账号列表失败: account sync failed"`.

`mserver/internal/server/sync_http_test.go`:

```go
package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLauncherAccountList(t *testing.T) {
	var lines []string
	previous := writeSyncLog
	writeSyncLog = func(line string) { lines = append(lines, line) }
	t.Cleanup(func() { writeSyncLog = previous })

	called := false
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		called = true
		if request.Header.Get("X-Account-Sync-Token") != "secret" {
			http.NotFound(writer, request)
			return
		}
		var payload struct {
			DeviceID string `json:"device_id"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.DeviceID != "device-a" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"accounts":[{"user_id":1000001,"name":"秋","state":"server"}]}`)
	}))
	t.Cleanup(remote.Close)
	handler := NewHandler(Deps{DeviceID: "device-a"})

	post := func(remoteAddr, body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/local/launcher/accounts", strings.NewReader(body))
		request.RemoteAddr = remoteAddr
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}
	away := post("192.0.2.8:9", `{"host":"127.0.0.1","port":1,"token":"secret"}`)
	if away.Code != http.StatusNotFound || called {
		t.Fatalf("non-loopback = %d called %v", away.Code, called)
	}
	for _, body := range []string{
		`{"host":"","port":26020,"token":"secret"}`,
		`{"host":"127.0.0.1","port":0,"token":"secret"}`,
		`{"host":"127.0.0.1","port":26020,"token":""}`,
	} {
		got := post("127.0.0.1:9", body)
		if got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), "account sync body is invalid") || called {
			t.Fatalf("invalid %s = %d %s called %v", body, got.Code, got.Body.String(), called)
		}
	}

	computer, err := http.NewRequest(http.MethodGet, remote.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	host := computer.URL.Hostname()
	port := computer.URL.Port()
	missing := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	}))
	t.Cleanup(missing.Close)
	missingURL, err := http.NewRequest(http.MethodGet, missing.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	failed := post("127.0.0.1:9", `{"host":"`+missingURL.URL.Hostname()+`","port":`+missingURL.URL.Port()+`,"token":"secret"}`)
	if failed.Code != http.StatusBadRequest || !strings.Contains(failed.Body.String(), "account sync failed") {
		t.Fatalf("upstream 404 = %d %s", failed.Code, failed.Body.String())
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "账号列表失败: account sync failed") || strings.Contains(joined, "获取账号列表") {
		t.Fatalf("failure log = %s", joined)
	}

	lines = nil
	ok := post("127.0.0.1:9", `{"host":"`+host+`","port":`+port+`,"token":"secret"}`)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"name":"秋"`) {
		t.Fatalf("list = %d %s", ok.Code, ok.Body.String())
	}
	joined = strings.Join(lines, "\n")
	if !strings.Contains(joined, "获取账号列表") || !strings.Contains(joined, "账号列表 1 个") {
		t.Fatalf("success log = %s", joined)
	}
}
```

`writeSyncLog` is assigned in this test, so declare it in Step 3 before running the test the second time. The first run fails on the missing symbol. The invalid-body cases use ports and hosts that are rejected before dialing, so `called` stays false.

- [ ] **Step 2: Run the tests to verify they fail**

```powershell
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
go test ./internal/server/ -run "TestTransferLogLines|TestLauncherAccountList" -count=1
```

Working directory: `mserver`

Expected: FAIL because `logList` and the route do not exist.

- [ ] **Step 3: Implement the log lines and the phone route**

`mserver/internal/server/sync_log.go`:

```go
package server

import (
	"fmt"

	"kairisei.local/server/accountsync"
)

func logList(write func(string), count int, err error) {
	if err != nil {
		write("账号列表失败: " + err.Error())
		return
	}
	write("获取账号列表")
	write(fmt.Sprintf("账号列表 %d 个", count))
}

func logTransfer(write func(string), action string, userID int, err error, choice *accountsync.ChoiceRequiredError) {
	write(fmt.Sprintf("%s %d", action, userID))
	if choice != nil {
		write(fmt.Sprintf("需要选择进度 %d: 电脑 %d，手机 %d", userID, choice.ServerRevision, choice.PhoneRevision))
		return
	}
	if err != nil {
		write(fmt.Sprintf("失败 %d: %s", userID, err.Error()))
		return
	}
	write(fmt.Sprintf("完成 %d", userID))
}
```

`mserver/internal/sync/client.go`:

```go
func List(baseURL, token, deviceID string) ([]accountsync.AccountBrief, error) {
	return accountsync.ListAccounts(baseURL, token, deviceID)
}
```

In `sync_http.go`, add:

```go
var writeSyncLog = func(line string) {
	fmt.Println(line)
}
```

Before the current body validation, handle `POST /local/launcher/accounts` when `request.URL.Path` is that path. Parse only `host`, `port`, and `token`. If `syncHostOK` fails, the port is outside `1..65535`, or the token is empty, write 400 `account sync body is invalid` and return. Otherwise call `sync.List`. On error, `logList(writeSyncLog, 0, err)` and write 400 with `err.Error()`. On success, `logList(writeSyncLog, len(accounts), nil)` and write 200 `{"accounts": accounts}`.

For borrow and return, choose the action before the call:

- path borrow: `借出`
- return with choice `phone`: `保留手机进度`
- return with choice `server`: `保留电脑进度`
- any other return: `交还`

After the call, if the error is `*accountsync.ChoiceRequiredError`, call `logTransfer` with that error and a nil Go error, then keep the existing 409 response. Otherwise call `logTransfer` with the Go error. Do not write a second line in the HTTP JSON. The JSON `error` stays the raw Go error, as it does today.

Register the route in `NewHandler`:

```go
mux.Handle("POST /local/launcher/accounts", accountSync)
```

- [ ] **Step 4: Run the phone route tests**

```powershell
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
go test ./internal/server/ ./internal/sync/ -count=1
```

Working directory: `mserver`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- mserver/internal/server/sync_log.go mserver/internal/server/sync_log_test.go mserver/internal/server/sync_http_test.go mserver/internal/server/sync_http.go mserver/internal/server/server.go mserver/internal/sync/client.go
git commit -m @"
Serve the account list on the phone and log transfers.
"@
```

---

### Task 3: Kotlin rules for state, search, buttons, refresh, and the log tail

**Files:**
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/PageRules.kt`
- Test: `mserver/android/app/src/test/kotlin/local/kairisei/mserver/PageRulesTest.kt`

**Interfaces:**
- Consumes: `SyncTarget.host`, `SyncTarget.remotePort`
- Produces: `HomeState.of`, `AccountRow`, `TransferAction.of`, `AccountList.applyRefresh`, `LogTail.latest`, `TransferRequest.body`

- [ ] **Step 1: Write the failing Kotlin test**

`PageRulesTest.kt`:

```kotlin
package local.kairisei.mserver

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PageRulesTest {
    @Test
    fun homeHasThreeStates() {
        assertEquals(HomeState("未启动", "启动", true, false), HomeState.of(false, false))
        assertEquals(HomeState("未启动", "启动", true, false), HomeState.of(false, true))
        assertEquals(HomeState("监测中", "监测中", false, false), HomeState.of(true, false))
        assertEquals(HomeState("已启动", "停止", true, true), HomeState.of(true, true))
    }

    @Test
    fun searchMatchesIdOrNameButNotThePlaceholder() {
        val named = AccountRow(1000001, "秋", "server")
        val blank = AccountRow(1000006, "", "server")
        assertEquals("1000001 · 秋", named.label())
        assertEquals("1000006 · 未命名", blank.label())
        assertTrue(named.matches(" 秋 "))
        assertTrue(named.matches("0001"))
        assertFalse(named.matches("另一"))
        assertTrue(blank.matches(""))
        assertFalse(blank.matches("未命名"))
        assertTrue(blank.matches("1000006"))
    }

    @Test
    fun buttonFollowsSelection() {
        assertEquals(TransferAction("借出/交还", false, ""), TransferAction.of(null))
        assertEquals(TransferAction("借出", true, ""), TransferAction.of(AccountRow(1, "秋", "server")))
        assertEquals(TransferAction("交还", true, ""), TransferAction.of(AccountRow(1, "秋", "mine")))
        assertEquals(
            TransferAction("借出/交还", false, "该账号在别的设备上"),
            TransferAction.of(AccountRow(1, "秋", "other")),
        )
        assertEquals(TransferAction("借出/交还", false, ""), TransferAction.of(AccountRow(1, "秋", "weird")))
    }

    @Test
    fun refreshKeepsTheOldListWhenTheFetchFails() {
        val old = listOf(AccountRow(1000001, "秋", "mine"))
        val failed = AccountList.applyRefresh(old, 1000001, null)
        assertEquals(old, failed.rows)
        assertEquals(1000001, failed.selected)
        val gone = AccountList.applyRefresh(old, 1000001, emptyList())
        assertEquals(emptyList<AccountRow>(), gone.rows)
        assertNull(gone.selected)
        val kept = AccountList.applyRefresh(old, 1000001, listOf(AccountRow(1000001, "秋", "server")))
        assertEquals(1000001, kept.selected)
        assertFalse(AccountRefresh.pulls(0))
        assertTrue(AccountRefresh.pulls(2))
        assertFalse(AccountRefresh.button(0, false))
        assertTrue(AccountRefresh.button(0, true))
        assertFalse(AccountRefresh.button(2, true))
    }

    @Test
    fun logTailShowsTheLastThreeNonBlankLines() {
        assertEquals("这次启动还没有输出", LogTail.latest(""))
        assertEquals("这次启动还没有输出", LogTail.latest("\n  \n"))
        assertEquals("one", LogTail.latest("one\n"))
        assertEquals("two\nthree\nfour", LogTail.latest("one\n\ntwo\nthree\nfour\n"))
    }

    @Test
    fun invalidTargetDoesNotBuildARequest() {
        assertNull(TransferRequest.body("http://10.0.0.1", "26020", "secret"))
        assertNull(TransferRequest.body("10.0.0.1", "0", "secret"))
        assertNull(TransferRequest.body("10.0.0.1", "26020", " "))
        val body = TransferRequest.body(" 10.0.0.1 ", "26020", " secret ")
        assertEquals("10.0.0.1", body?.getString("host"))
        assertEquals(26020, body?.getInt("port"))
        assertEquals("secret", body?.getString("token"))
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

```powershell
$env:ANDROID_HOME = "D:\autumn\Android\Storage\Sdk"
& "C:\Users\17864\.gradle\wrapper\dists\gradle-8.8-all\cgq7lt248yu0xgohgi051w98x\gradle-8.8\bin\gradle.bat" :app:testDebugUnitTest --offline --tests local.kairisei.mserver.PageRulesTest
```

Working directory: `mserver/android`

Expected: FAIL to compile because `PageRules.kt` does not exist. Do not use `gradlew.bat`; the wrapper tries to download Gradle 8.7 and times out.

- [ ] **Step 3: Write `PageRules.kt`**

```kotlin
package local.kairisei.mserver

import org.json.JSONObject

data class HomeState(
    val status: String,
    val button: String,
    val buttonEnabled: Boolean,
    val showTransfer: Boolean,
) {
    companion object {
        fun of(running: Boolean, addressesReady: Boolean): HomeState {
            if (!running) return HomeState("未启动", "启动", true, false)
            if (!addressesReady) return HomeState("监测中", "监测中", false, false)
            return HomeState("已启动", "停止", true, true)
        }
    }
}

data class AccountRow(val userID: Int, val name: String, val state: String) {
    fun label(): String = "$userID · " + if (name.isEmpty()) "未命名" else name

    fun matches(query: String): Boolean {
        val text = query.trim()
        if (text.isEmpty()) return true
        return userID.toString().contains(text, ignoreCase = true) ||
            name.contains(text, ignoreCase = true)
    }

    fun borrowedBackground(): Boolean = state != "server"
}

data class TransferAction(val text: String, val enabled: Boolean, val note: String) {
    companion object {
        fun of(row: AccountRow?): TransferAction {
            if (row == null) return TransferAction("借出/交还", false, "")
            return when (row.state) {
                "server" -> TransferAction("借出", true, "")
                "mine" -> TransferAction("交还", true, "")
                "other" -> TransferAction("借出/交还", false, "该账号在别的设备上")
                else -> TransferAction("借出/交还", false, "")
            }
        }
    }
}

data class AccountListResult(val rows: List<AccountRow>, val selected: Int?)

object AccountList {
    fun applyRefresh(current: List<AccountRow>, selected: Int?, fetched: List<AccountRow>?): AccountListResult {
        if (fetched == null) return AccountListResult(current, selected)
        val still = if (selected != null && fetched.any { it.userID == selected }) selected else null
        return AccountListResult(fetched, still)
    }
}

object AccountRefresh {
    fun pulls(visible: Int): Boolean = visible > 0

    fun button(visible: Int, revealed: Boolean): Boolean = visible == 0 && revealed
}

object LogTail {
    fun latest(text: String): String {
        val lines = text.lineSequence().filter { it.isNotBlank() }.toList()
        if (lines.isEmpty()) return "这次启动还没有输出"
        return lines.takeLast(3).joinToString("\n")
    }
}

object TransferRequest {
    fun body(host: String, portText: String, token: String): JSONObject? {
        val computer = SyncTarget.host(host) ?: return null
        val port = SyncTarget.remotePort(portText.trim()) ?: return null
        val secret = token.trim()
        if (secret.isEmpty()) return null
        return JSONObject().put("host", computer).put("port", port).put("token", secret)
    }
}
```

- [ ] **Step 4: Run the unit tests**

Use the same Gradle command as Step 2, without `--tests`, so the existing port and address tests run too.

Expected: `BUILD SUCCESSFUL`.

- [ ] **Step 5: Commit**

```powershell
git add -- mserver/android/app/src/main/kotlin/local/kairisei/mserver/PageRules.kt mserver/android/app/src/test/kotlin/local/kairisei/mserver/PageRulesTest.kt
git commit -m @"
Cover the phone page rules with Kotlin tests.
"@
```

---

### Task 4: Home, transfer page, and the pinned log

**Files:**
- Modify: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt`
- Modify: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/LogActivity.kt`
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/TransferActivity.kt`
- Modify: `mserver/android/app/src/main/AndroidManifest.xml`

**Interfaces:**
- Consumes: `HomeState`, `AccountRow`, `TransferAction`, `AccountList`, `AccountRefresh`, `LogTail`, `TransferRequest`, `SyncClient.post`, `POST /local/launcher/accounts`, `POST /local/launcher/borrow`, `POST /local/launcher/return`
- Produces: the three pages described in `docs/superpowers/specs/2026-10-17-mserver-page-design.md`

- [ ] **Step 1: Keep the existing unit tests as the regression check**

Do not add an emulator test. Task 3 already locks the decisions this task must call. Step 4 reruns those tests after the activities compile.

- [ ] **Step 2: Replace the home page**

Remove the log button, computer fields, account field, borrow button, return button, choice buttons, and `syncStatus` from `MainActivity`. Stop reading and writing `sync-user`.

The content view is a vertical layout. The upper child is a `ScrollView` with weight `1`. Inside it: status `TextView`, label `本机端口`, port field, action button, `server-error` text, label `服务器地址`, address list, and the transfer button. The lower child is the log strip, outside the `ScrollView`.

`render` reads `server-running`. `addressesReady` is an activity field. It becomes true only after a non-null address body parses as a JSON array during this running period. It becomes false when `server-running` is not `1`. A later failed refresh does not clear it and does not clear the current address rows.

Call `HomeState.of(running, addressesReady)`.

- Status `未启动` uses color `#FFB4A8`. `监测中` uses `#FFD60A`. `已启动` uses `#30D158`.
- The action button text and `isEnabled` come from `HomeState`. `监测中` has no click listener effect because the button is disabled.
- The transfer button is visible only when `showTransfer` is true. Its text is `移交`. It starts `TransferActivity`.
- The port field is enabled only when the state is `未启动`.
- `启动` still writes `server-running` `1`, truncates `server.log`, deletes `server-error`, and starts `ServerService`.
- `停止` still writes `server-running` `0` and stops the service.
- Address rows still copy `host:port` and show no toast.
- The strip uses `LogTail.latest` on `server.log`. Its height fits three lines. Tapping any part of it starts `LogActivity`. The home refresh stays every 2 seconds and also reloads the strip.

- [ ] **Step 3: Add the transfer page**

Register `TransferActivity` in the manifest with `android:exported="false"`.

Layout, top to bottom: title `移交`, computer address, computer port, token, search field, list region, action button, choice block, status text. Those sit in a column above a fixed log strip. The list region is the only part that scrolls. The strip uses the same `LogTail` view as the home page and is not inside the list scroller.

Persist address, port, and token with the existing files `server-host`, `remote-port`, and `sync-token`. Do not touch `sync-user`.

Every 2 seconds, reload the strip and read `server-running`. If it is not `1`, `finish()` the activity.

On create, and after every borrow, return, or choice request finishes, call the list route. Manual refresh uses that same call. While a call is running, ignore another tap or pull and disable the action button.

`TransferRequest.body` returning null means do not call `SyncClient`. Set the status and append `电脑地址、端口或口令无效` to `server.log`. If `SyncClient.post` throws, set the status and append `没有连上本机服务`. Do not append when a response arrives. The Go process already logged that result.

Parse `accounts` into `AccountRow`. `AccountList.applyRefresh(null fetched)` is the failure path: keep rows and selection. A successful body, including an empty array, replaces the rows. Filter with `AccountRow.matches`. A `state` other than `server` uses background `#1F7A3A` and white text. `server` uses white background and black text. The selected row gets a `#8AB4FF` outline.

The empty list region starts without a button. Clicking that region sets the revealed flag. `AccountRefresh.button` decides whether `刷新` is shown. Clicking `刷新` fetches. A successful fetch that has visible rows clears the flag.

When `AccountRefresh.pulls` is true, a downward drag that starts with the list already at the top refreshes on release. Use a threshold of `48 * density` pixels. Movement while the list is not at the top only scrolls. The search text stays as typed.

`TransferAction.of` sets the action button text, enabled state, and the note under it. `other` shows `该账号在别的设备上`. Any other non-action state shows no note. Changing the selection hides the choice buttons and the revision line.

The action sends `user_id` of the selected row to `/local/launcher/borrow` or `/local/launcher/return`. Choice buttons send `phone` or `server` to return. A 409 `choice required` shows `电脑进度版本 x，手机进度版本 y` and the two buttons `保留手机进度` and `保留电脑进度`. Success shows `完成`. A borrow or return failure shows `失败 <id>: <error>`. A list failure shows `账号列表失败: <error>`. After success, after failure, and after `choice required`, refresh the list. A failed refresh keeps the rows already on screen. The status stays `完成` until a later list failure replaces it or the player starts another action.

- [ ] **Step 4: Make the full log selectable**

`LogActivity` keeps the title `查看日志`. Put the file text in a `TextView` with `setTextIsSelectable(true)` inside a `ScrollView`. Do not add the bottom strip. Load the file in `onCreate`. Keep the existing truncation of `server.log` on the home page's start action.

- [ ] **Step 5: Run the unit tests**

```powershell
$env:ANDROID_HOME = "D:\autumn\Android\Storage\Sdk"
& "C:\Users\17864\.gradle\wrapper\dists\gradle-8.8-all\cgq7lt248yu0xgohgi051w98x\gradle-8.8\bin\gradle.bat" :app:testDebugUnitTest --offline
```

Working directory: `mserver/android`

Expected: `BUILD SUCCESSFUL`.

- [ ] **Step 6: Commit**

```powershell
git add -- mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt mserver/android/app/src/main/kotlin/local/kairisei/mserver/LogActivity.kt mserver/android/app/src/main/kotlin/local/kairisei/mserver/TransferActivity.kt mserver/android/app/src/main/AndroidManifest.xml
git commit -m @"
Show the transfer page and pin the log to the bottom.
"@
```

- [ ] **Step 7: Manual check after the activities are installed**

Confirm on a device: 未启动, 监测中 with the gray disabled button, 已启动 with addresses and `移交`. The transfer page lists green and white rows. An empty list reveals `刷新` only after a tap. A non-empty list refreshes from a pull at the top. The bottom three lines stay put while the list scrolls. Tapping them opens the full log, which scrolls and can be copied. Borrow and return lines appear in that log.

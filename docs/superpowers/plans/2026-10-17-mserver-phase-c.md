# Phase C Phone Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Start and stop an Android micro-server that listens on every IPv4 interface, lists the addresses a client can use, and answers login with the address that client connected to.

**Architecture:** `mserver` is a sibling Go module that replaces `kairisei.local/server` with `../server`. Kotlin shows the page and runs `kairi-mserver` in a foreground service. The Go process listens on `0.0.0.0` for the game port and the next port, chooses login hosts from the accepted socket, and sends every other game route to one degrader. It does not call `LoadOrImport`, so starting it does not seed a player.

**Tech Stack:** Go 1.25, `net/http`, existing `accountstore` and `cnbootstrap` login JSON, Kotlin Android foreground service, `GOOS=android GOARCH=arm64`.

## Global Constraints

- Game client changes only the server address.
- Sm does not create accounts and does not serve CPK, patch, or image bodies.
- Degrader response is HTTP 200, `res_code` `-1`, `res_str` `该功能需要连接电脑上的服务器`. It does not write the account database and does not delete downloaded game resources.
- Unknown UUID login, and login for an account this phone does not hold, returns HTTP 200, `res_code` `-1`, `res_str` `该账号不在这台手机上`, allocates no `user_id`, and does not write.
- Listen address is `0.0.0.0`. There is no advertise-host flag and no Admin listener.
- Default game port is `26020`. A port is a canonical decimal integer from `1` through `65534`. Battle listens on that port plus `1`.
- Login, resource URLs, and the server list use the local IP of the accepted TCP connection and ignore the Host header. Battle uses that same IP and the battle port.
- `device-id` is created only when the file is absent. A valid UUID is reused. Invalid contents are left in place and startup fails.
- Stopping the service kills the process and leaves account files in the app files directory.
- Phase C has no PC address, borrow, return, or snapshot-choice page.
- Do not stage unrelated dirty files in this worktree.

---

## File map

| File | Responsibility |
| --- | --- |
| `mserver/go.mod` | Module `kairisei.local/mserver`, replace `../server` |
| `mserver/internal/degrade/degrade.go` | Fixed unimplemented-route response |
| `mserver/internal/addresses/addresses.go` | Choose client-usable IPv4 rows from interface snapshots |
| `mserver/internal/deviceid/deviceid.go` | Create, reuse, or refuse `device-id` |
| `mserver/internal/port/port.go` | Accept only a canonical port from 1 through 65534 |
| `server/internal/accountstore/login_lookup.go` | Read a login UUID without inserting a player |
| `server/internal/cnbootstrap/login_export.go` | Expose the existing login JSON and server-list CSV |
| `mserver/internal/server/server.go` | Routes, listeners, shutdown |
| `mserver/cmd/kairi-mserver/main.go` | Flags `-port` and `-data` |
| `mserver/android/` | Two pages, foreground service, notification |

---

### Task 1: Degrader does not write

**Files:**
- Create: `mserver/go.mod`
- Create: `mserver/internal/degrade/degrade.go`
- Test: `mserver/internal/degrade/degrade_test.go`

**Interfaces:**
- Consumes: none
- Produces: `func Handler() http.Handler` and `const Message = "该功能需要连接电脑上的服务器"`

- [ ] **Step 1: Write the failing test**

`mserver/internal/degrade/degrade_test.go`:

```go
package degrade

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandlerReturnsFixedRefusalAndCreatesNoFile(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "account.sqlite3")
	request := httptest.NewRequest(http.MethodPost, "/FriendSearch", nil)
	request.Header.Set("X-Data-Dir", dir)
	response := httptest.NewRecorder()

	Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["res_code"] != float64(-1) || payload["res_str"] != Message {
		t.Fatalf("payload = %#v", payload)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("degrader created %s", marker)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/degrade/ -count=1`

Working directory: `mserver`

Expected: FAIL because the module or package does not exist.

- [ ] **Step 3: Write minimal implementation**

`mserver/go.mod`:

```go
module kairisei.local/mserver

go 1.25.0

require kairisei.local/server v0.0.0

replace kairisei.local/server => ../server
```

Run `go mod tidy` in `mserver` after the first import of `kairisei.local/server` exists. Task 1 does not import it yet, so tidy can wait until Task 4's caller exists. The require line may be omitted until then if `go test` rejects an unused require; keep the module line and the replace line.

`mserver/internal/degrade/degrade.go`:

```go
package degrade

import (
	"encoding/json"
	"net/http"
)

const Message = "该功能需要连接电脑上的服务器"

func Handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"res_code": -1,
			"res_str":  Message,
		})
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/degrade/ -count=1`

Working directory: `mserver`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/go.mod mserver/internal/degrade
git commit -m "Reject unimplemented micro-server routes without writing"
```

---

### Task 2: Choose client-usable addresses

**Files:**
- Create: `mserver/internal/addresses/addresses.go`
- Test: `mserver/internal/addresses/addresses_test.go`

**Interfaces:**
- Consumes: none
- Produces:

```go
type Interface struct {
	Name  string
	Up    bool
	Addrs []net.IP
}

type Row struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Loopback bool   `json:"loopback"`
	Label    string `json:"label"`
}

func Select(interfaces []Interface) []Row
```

- [ ] **Step 1: Write the failing test**

```go
func TestSelectKeepsUsableIPv4Only(t *testing.T) {
	rows := Select([]Interface{
		{Name: "lo", Up: true, Addrs: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("127.0.0.2"),
		}},
		{Name: "wlan0", Up: true, Addrs: []net.IP{
			net.ParseIP("192.168.1.23"),
			net.ParseIP("0.0.0.0"),
			net.ParseIP("169.254.4.5"),
			net.ParseIP("fe80::1"),
		}},
		{Name: "down0", Up: false, Addrs: []net.IP{net.ParseIP("10.0.0.8")}},
	})
	if len(rows) != 2 {
		t.Fatalf("rows = %#v", rows)
	}
	if rows[0].Label != "本机 · lo" || rows[0].Host != "127.0.0.1" || !rows[0].Loopback || rows[0].Name != "lo" {
		t.Fatalf("loopback row = %#v", rows[0])
	}
	if rows[1].Label != "wlan0" || rows[1].Host != "192.168.1.23" || rows[1].Loopback || rows[1].Name != "wlan0" {
		t.Fatalf("wifi row = %#v", rows[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/addresses/ -count=1`

Working directory: `mserver`

Expected: FAIL with `Select` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
func Select(interfaces []Interface) []Row {
	linkLocal := &net.IPNet{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)}
	var rows []Row
	for _, iface := range interfaces {
		if !iface.Up {
			continue
		}
		for _, addr := range iface.Addrs {
			ip := addr.To4()
			if ip == nil || ip.Equal(net.IPv4zero) || linkLocal.Contains(ip) {
				continue
			}
			if ip.IsLoopback() {
				if !ip.Equal(net.IPv4(127, 0, 0, 1)) {
					continue
				}
				rows = append(rows, Row{Name: iface.Name, Host: "127.0.0.1", Loopback: true, Label: "本机 · " + iface.Name})
				continue
			}
			rows = append(rows, Row{Name: iface.Name, Host: ip.String(), Loopback: false, Label: iface.Name})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Loopback != rows[j].Loopback {
			return rows[i].Loopback
		}
		return rows[i].Host < rows[j].Host
	})
	if rows == nil {
		return []Row{}
	}
	return rows
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/addresses/ -count=1`

Working directory: `mserver`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/internal/addresses
git commit -m "Select the addresses a client can use"
```

---

### Task 3: Device id and port text

**Files:**
- Create: `mserver/internal/deviceid/deviceid.go`
- Test: `mserver/internal/deviceid/deviceid_test.go`
- Create: `mserver/internal/port/port.go`
- Test: `mserver/internal/port/port_test.go`

**Interfaces:**
- Consumes: none
- Produces: `func LoadOrCreate(dir string) (string, error)` and `func Parse(text string) (int, error)`

- [ ] **Step 1: Write the failing tests**

Device id: a missing `device-id` file is created as a lowercase UUID. A second call returns the same text and does not change the file bytes. An existing `ABCDEFAB-CDEF-ABCD-EFAB-CDEFABCDEFAB` is returned lowercased and the file bytes stay unchanged. A file containing `nope` returns an error containing `device id is invalid`, and the file still contains `nope`.

Port: `Parse("26020")` is 26020. `Parse("1")` and `Parse("65534")` succeed. `Parse("")`, `Parse("0")`, `Parse("01")`, `Parse("65535")`, `Parse("26020 ")`, and `Parse("+26020")` return an error containing `本机端口无效`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/deviceid/ ./internal/port/ -count=1`

Working directory: `mserver`

Expected: FAIL because the packages do not exist.

- [ ] **Step 3: Write minimal implementation**

`LoadOrCreate` reads `filepath.Join(dir, "device-id")`. A missing file gets 16 `crypto/rand` bytes, with byte 6 set to `(b & 0x0f) | 0x40` and byte 8 set to `(b & 0x3f) | 0x80`, formatted as 36 lowercase hex characters with dashes after bytes 4, 6, 8, and 10. Write those bytes with mode `0600` and return them. An existing file is trimmed and lowercased in memory. Accept it only when its length is 36, dashes are at indexes 8, 13, 18, and 23, and every other character is `0-9` or `a-f`. Do not write the file back. Any other content returns `errors.New("device id is invalid")`.

`Parse` returns `errors.New("本机端口无效")` unless every character is a digit, the first character is `1` through `9`, and `strconv.Atoi` yields a value of at most 65534.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/deviceid/ ./internal/port/ -count=1`

Working directory: `mserver`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/internal/deviceid mserver/internal/port
git commit -m "Keep the phone device id and validate the port"
```

---

### Task 4: Look up a login without creating a player

**Files:**
- Create: `server/internal/accountstore/login_lookup.go`
- Test: `server/internal/accountstore/login_lookup_test.go`

**Interfaces:**
- Consumes: `OpenDatabase`, `NewAccounts`, `validLoginUUID` shape, `cn_local_account`
- Produces: `func (accounts *Accounts) FindLogin(loginUUID string) (userID int, found bool, err error)`

- [ ] **Step 1: Write the failing test**

Open a database with `OpenDatabase` and `NewAccounts`. Do not call `LoadOrImport`. Call `FindLogin("00000000-0000-0000-0000-000000000099")`. Assert `found` is false and `err` is nil. Assert `SELECT COUNT(*) FROM cn_local_account WHERE user_id < 1900000000` is 0. Insert user `1000002` with that UUID and session `held-session`, call `FindLogin` again, and assert user id `1000002` and `found` true. Call `FindLogin("not-a-uuid")` and assert `found` false and no error.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/accountstore/ -count=1 -run TestFindLoginDoesNotCreate`

Working directory: `server`

Expected: FAIL with `FindLogin` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
func (accounts *Accounts) FindLogin(loginUUID string) (int, bool, error) {
	loginUUID = strings.ToLower(strings.TrimSpace(loginUUID))
	if !validLoginUUID(loginUUID) {
		return 0, false, nil
	}
	database, err := accounts.storage.OpenRead()
	if err != nil {
		return 0, false, err
	}
	var userID int
	err = database.QueryRowContext(context.Background(),
		`SELECT user_id FROM cn_local_account WHERE login_uuid = ?`, loginUUID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return userID, true, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/accountstore/ -count=1 -run TestFindLoginDoesNotCreate`

Working directory: `server`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/accountstore/login_lookup.go server/internal/accountstore/login_lookup_test.go
git commit -m "Look up a login without creating an account"
```

---

### Task 5: Login uses the socket address

**Files:**
- Create: `server/internal/cnbootstrap/login_export.go`
- Create: `mserver/internal/server/server.go`
- Test: `mserver/internal/server/login_test.go`

**Interfaces:**
- Consumes: `degrade.Handler`, `addresses.Select`, `accountstore.FindLogin`, `accountstore.RefuseIfNotHolder`, `accountstore.PutHolder`, `accountstore.NewAccounts`
- Produces:

```go
func LoginHandler(accounts *accountstore.Accounts, host string, port int) http.Handler
func ServerListHandler(host string, port int) http.Handler
func WithLocalAddr(ctx context.Context, host string) context.Context
func NewHandler(deps Deps) http.Handler

type Deps struct {
	Accounts *accountstore.Accounts
	DeviceID string
	GamePort int
	Interfaces func() ([]addresses.Interface, error)
}
```

- [ ] **Step 1: Write the failing test**

`login_export.go` returns the existing handlers:

```go
func LoginHandler(accounts *accountstore.Accounts, host string, port int) http.Handler {
	return cnBootstrapLogin(accounts, host, port, CDNConfig{}, resourceVersionNamespace(nil, nil, nil), "images")
}

func ServerListHandler(host string, port int) http.Handler {
	return cnBootstrapServerList(host, port)
}
```

The test opens accounts without `LoadOrImport`, writes `device-id` through `deviceid.LoadOrCreate`, and builds `NewHandler`. It inserts `cn_local_account` user `1000002`, UUID `00000000-0000-0000-0000-000000000099`, session `held-session`, then `PutHolder` with that user and the device id.

Three login requests all use `clver` `6.0.7`, `Host` `evil.example:9999`, and `WithLocalAddr(ctx, "192.0.2.8")`:

- The held UUID returns 200. `api_url` is `http://192.0.2.8:26020/`. `version_url`, `res_patch_url`, `res_cpk_url`, and `res_img_url` contain `192.0.2.8` and do not contain `evil.example`. `sess_key` is not `held-session`.
- UUID `00000000-0000-0000-0000-000000000088` returns `res_code` `-1` and `res_str` `该账号不在这台手机上`. The player-id count below `1900000000` stays 1, and user `1000002`'s session is unchanged by this request.
- Change the holder id to `other-phone` and repeat the held UUID. The response is the same refusal, and the session stays at the value left by the successful login.

`GET /local/server/default.list` with the same local address returns a body containing `192.0.2.8` and `26020`, and not `evil.example`. The same host is used for `apple-review.list` and `qa.list`. Any other `/local/server/` path uses the degrader.

`GET /local/launcher/addresses` with a lister that fails returns 200 and `[]`. With the Task 2 fixture, the JSON includes `本机 · lo` and `192.168.1.23`.

`POST /local/version/resources/Android/patch/catalog.dat` and `GET /local/resources/cpk/example.cpk` return the degrader message. The player count stays 1.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -count=1 -run TestLoginUsesSocketAddress`

Working directory: `mserver`

Expected: FAIL with `NewHandler` undefined.

- [ ] **Step 3: Write minimal implementation**

`NewHandler` registers `POST /loginSDK.php`, `GET /local/launcher/addresses`, and `GET /local/server/default.list`, `GET /local/server/apple-review.list`, and `GET /local/server/qa.list`. Every other path uses `degrade.Handler`. The existing login JSON is not given a new battle field. Battle's advertised host is the same socket host, and its port is `Deps.GamePort+1`, which Task 6 binds.

The login handler reads and restores the body, then refuses before the existing login handler can insert a player:

```go
body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
if err != nil {
	http.Error(writer, "read login", http.StatusBadRequest)
	return
}
request.Body = io.NopCloser(bytes.NewReader(body))
var payload struct {
	UUID string `json:"uuid"`
}
_ = json.Unmarshal(body, &payload)
userID, found, err := deps.Accounts.FindLogin(payload.UUID)
if err != nil {
	http.Error(writer, "read login", http.StatusBadRequest)
	return
}
if !found || deps.Accounts.RefuseIfNotHolder(userID, deps.DeviceID) != nil {
	writePhoneRefusal(writer)
	return
}
host, ok := localHost(request)
if !ok {
	http.Error(writer, "connection address is missing", http.StatusInternalServerError)
	return
}
request.Body = io.NopCloser(bytes.NewReader(body))
cnbootstrap.LoginHandler(deps.Accounts, host, deps.GamePort).ServeHTTP(writer, request)
```

`writePhoneRefusal` writes HTTP 200, `res_code` `-1`, and `res_str` `该账号不在这台手机上`. `localHost` reads only the value stored by `WithLocalAddr`. The three server-list routes call `cnbootstrap.ServerListHandler(host, deps.GamePort)` with that same host. The address route writes `[]` with status 200 when `Deps.Interfaces` returns an error, and otherwise writes `addresses.Select(ifaces)` as JSON. The existing login JSON is not given a new battle field. Battle's port is `Deps.GamePort+1`, which Task 6 binds on `0.0.0.0`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -count=1 -run TestLoginUsesSocketAddress`

Working directory: `mserver`

Expected: PASS

Run: `go mod tidy` in `mserver`, then `go test ./internal/degrade/ ./internal/addresses/ ./internal/deviceid/ ./internal/port/ ./internal/server/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/cnbootstrap/login_export.go mserver/internal/server mserver/go.mod mserver/go.sum
git commit -m "Answer login with the connection's own address"
```

---

### Task 6: Listen on every interface and keep files after shutdown

**Files:**
- Modify: `mserver/internal/server/server.go`
- Test: `mserver/internal/server/process_test.go`
- Create: `mserver/cmd/kairi-mserver/main.go`

**Interfaces:**
- Consumes: `port.Parse`, `deviceid.LoadOrCreate`, `NewHandler`
- Produces:

```go
func Start(dataDir string, portText string) (*Running, error)
func (running *Running) Shutdown(ctx context.Context) error

type Running struct {
	GamePort   int
	BattlePort int
	DeviceID   string
	BaseURL    string
}
```

`main` accepts `-port` default `26020` and `-data` required. It does not define an advertise-host flag.

- [ ] **Step 1: Write the failing test**

Create `filepath.Join(dir, "account-marker.txt")` before `Start`. `Start(dir, "0")` and `Start(dir, "65535")` return an error containing `本机端口无效`, and no TCP listener remains on those ports. The marker file still exists.

Write `device-id` with `nope` and call `Start(dir, "26020")`. The error contains `device id is invalid`. The file still contains `nope`. The marker still exists.

Replace `device-id` by deleting it, then `Start` on a free game port whose next port is also free. `GamePort` is that port and `BattlePort` is `GamePort+1`. Dial `127.0.0.1` on both ports; both connections succeed. `GET http://127.0.0.1:{GamePort}/loginSDK.php` is not the assertion. `POST` login for an unknown UUID with `Host: evil.example` returns `该账号不在这台手机上`, and `api_url` is absent. A second process `Start` on the same port fails. `SELECT COUNT(*) FROM cn_local_account WHERE user_id < 1900000000` is 0. Dial `127.0.0.1` on `GamePort+2` and assert it fails, because Admin is not listening. `Shutdown` returns nil, both game and battle dials then fail, and the marker file still exists.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -count=1 -run 'TestStart|TestShutdown'`

Working directory: `mserver`

Expected: FAIL with `Start` undefined.

- [ ] **Step 3: Write minimal implementation**

`Start` calls `port.Parse` before any listener. It calls `deviceid.LoadOrCreate` next. It opens `filepath.Join(dataDir, "save.json")` with `OpenDatabase`, using `filepath.Join(dataDir, "unused-seed.json")` as the seed path, then `NewAccounts`. It does not call `LoadOrImport`.

Listen `0.0.0.0:gamePort` and `0.0.0.0:battlePort`. If the second listen fails, close the first and return the error. The battle listener stays open and is not given a BattleSv protocol. `http.Server.ConnContext` stores `conn.LocalAddr()` for `WithLocalAddr` by taking the TCP IP string. `Shutdown` closes both listeners and the HTTP server. It does not remove files in `dataDir`.

`main` parses the two flags, calls `Start`, and on error writes the error to stderr and exits 1. On success it waits for `SIGINT` or `SIGTERM`, then `Shutdown`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -count=1 -run 'TestStart|TestShutdown|TestLogin'`

Working directory: `mserver`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/internal/server mserver/cmd/kairi-mserver
git commit -m "Listen on every interface and keep account files on shutdown"
```

---

### Task 7: Android page and foreground service

**Files:**
- Create: `mserver/android/settings.gradle.kts`
- Create: `mserver/android/build.gradle.kts`
- Create: `mserver/android/gradle.properties`
- Create: `mserver/android/app/build.gradle.kts`
- Create: `mserver/android/app/src/main/AndroidManifest.xml`
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/PortText.kt`
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/AddressLine.kt`
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt`
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/LogActivity.kt`
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/ServerService.kt`
- Test: `mserver/android/app/src/test/kotlin/local/kairisei/mserver/PortTextTest.kt`
- Test: `mserver/android/app/src/test/kotlin/local/kairisei/mserver/AddressLineTest.kt`

**Interfaces:**
- Consumes: `kairi-mserver -port {port} -data {filesDir}`
- Produces: the two pages and a foreground service that does not delete files on stop

- [ ] **Step 1: Write the failing unit tests**

`PortText.parse("26020")` is 26020. `parse("1")` and `parse("65534")` return those ints. `parse("")`, `parse("0")`, `parse("01")`, `parse("65535")`, and `parse("26020 ")` return null.

`AddressLine.of("本机 · lo", "127.0.0.1", 26020)` has title `本机 · lo` and copy text `127.0.0.1:26020`. `of("wlan0", "192.168.1.23", 26020)` has copy text `192.168.1.23:26020`. Neither title nor copy text contains `复制`.

- [ ] **Step 2: Run test to verify it fails**

Run: `gradlew.bat :app:testDebugUnitTest`

Working directory: `mserver/android`

Expected: FAIL because the classes do not exist. If `gradlew.bat` is absent, run `gradle wrapper --gradle-version 8.7` in that directory first.

- [ ] **Step 3: Write minimal implementation**

`PortText.parse` uses the same canonical decimal rule as `port.Parse`. `AddressLine.of` stores the server-provided label and builds `host + ":" + port`. It has no hint field.

`app/build.gradle.kts` uses `compileSdk = 34`, `minSdk = 26`, `targetSdk = 34`, namespace and applicationId `local.kairisei.mserver`, and `testImplementation("junit:junit:4.13.2")`. The root plugin is Android application 8.5.2 and Kotlin 1.9.24. `gradle.properties` sets `android.useAndroidX=true`.

The manifest declares `INTERNET`, `FOREGROUND_SERVICE`, `FOREGROUND_SERVICE_SPECIAL_USE`, and `POST_NOTIFICATIONS`. `MainActivity` is the launcher. `LogActivity` is not exported. `ServerService` is not exported and uses `foregroundServiceType="specialUse"` with property `android.app.PROPERTY_SPECIAL_USE_FGS_SUBTYPE` value `local game server`.

`MainActivity` builds the first page in code: a port field, one button that switches between `启动` and `停止`, a vertical list, and a button labeled `查看日志`. There is no IP field, no copy button, and no text `点一行即可复制` or `复制`. While stopped, the port field is enabled. Its text is the contents of `server-port`, or `26020` when that file is absent. Changing it writes `server-port` only when `PortText.parse` succeeds. An illegal port leaves the previous file unchanged and shows `本机端口无效` on the page. Start with an illegal port does not call `startForegroundService`.

Start writes `server-running` as `1`, truncates `server.log`, deletes `server-error` if present, and starts the service. Stop writes `server-running` as `0` and calls `stopService`. Leaving the activity does not call `stopService`. Every two seconds while the activity is started and `server-running` is `1`, fetch `http://127.0.0.1:{port}/local/launcher/addresses`. For each JSON row, add a button whose text is the label and whose second line is `AddressLine.of(label, host, port).copyText`. Clicking that button sets the clipboard to the copy text and shows no toast. While stopped, the list is empty and the port field is enabled again. If `server-error` exists, show its text on the first page.

`LogActivity` shows the title `查看日志` and the contents of `server.log`.

`ServerService.onStartCommand` reads `server-running`. When it is not `1`, it stops itself and returns `START_NOT_STICKY`. When it is `1`, it shows one ongoing notification whose tap opens `MainActivity`, then starts `libkairi-mserver.so` from `applicationInfo.nativeLibraryDir` if that process is not already alive. Arguments are `-port` and `-data`, with `-data` set to `filesDir`. Append stdout and stderr to `server.log`. When the process exits by itself, write its last stderr line to `server-error`, write `server-running` as `0`, remove the notification, and `stopSelf`. Do not start the process again in that callback. `onDestroy` destroys the process if it is alive. It does not delete `device-id`, `server-port`, `server.log`, `save.json`, or `save-state.sqlite3`. Return `START_STICKY` from the running path so a system kill while `server-running` is still `1` comes back through `onStartCommand` and starts the process again.

Build the binary before the APK:

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "android"
$env:GOARCH = "arm64"
go build -o android/app/src/main/jniLibs/arm64-v8a/libkairi-mserver.so ./cmd/kairi-mserver
```

Working directory: `mserver`

- [ ] **Step 4: Run test to verify it passes**

Run: `gradlew.bat :app:testDebugUnitTest`

Working directory: `mserver/android`

Expected: `BUILD SUCCESSFUL`

Run: `gradlew.bat :app:assembleDebug`

Expected: `BUILD SUCCESSFUL` and an APK under `app/build/outputs/apk/debug/`. This needs `ANDROID_HOME` with platform 34. Do not launch an emulator.

- [ ] **Step 5: Commit**

```bash
git add mserver/android
git commit -m "Add the Android page and foreground service for the micro-server"
```

Do not add `app/build/` or the generated `.so` if it is produced only on the build machine. Do add `jniLibs` only when the binary is intentionally stored. The Gradle project and Kotlin sources are the committed deliverable.

Manual check, with no emulator required for the unit tests: install the APK, start it, and confirm the stopped page has a port and no IP field. After start, the list shows `本机` for `127.0.0.1` and the Wi-Fi interface name for its IPv4. Tapping a row copies `地址:端口` without a hint line. `查看日志` shows process output. Leaving the activity leaves the notification. Stop removes the notification and leaves `device-id` in the app files directory.

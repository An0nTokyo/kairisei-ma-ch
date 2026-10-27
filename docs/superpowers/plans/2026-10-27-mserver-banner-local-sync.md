# mserver Banner Local Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sync home/gacha banner PNGs from PC to the phone via a transfer-page button; serve them locally with an embedded placeholder fallback; keep patch/cpk proxy-only.

**Architecture:** PC exposes `GET /local/sync/banners` (account-sync token). Phone loopback `POST /local/launcher/banners` forwards, validates paths, and writes under `filesDir/banners/`. Runtime home/gacha handlers read that tree or embed a default PNG; patch/cpk stay on `resource-upstream` only.

**Tech Stack:** Go 1.25, chi, existing `accountsync` token header, mserver ServeMux, Android `TransferActivity` / `SyncClient`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-10-27-mserver-banner-local-sync-design.md`.
- Do not cache or locally serve patch/cpk on the phone.
- home/gacha must not use `resource-upstream` after this work.
- Sync auth uses `X-Account-Sync-Token` (same secret as borrow/list).
- Banner `path` values must start with `/local/home/` or `/local/gacha/` and must not contain `..`.
- Missing local banner → HTTP 200 + default PNG (never degrader `-1` for these paths).
- Do not modify generic `degrade.Handler` semantics.
- Default PNG: reuse existing CN intro placeholder bytes (`cn602IntroPlaceholderPNG` / 1×1 PNG already in `server/internal/cnbootstrap/web_handlers.go`) — no large banner artwork is checked into the repo.

---

## File map

| File | Responsibility |
|------|----------------|
| `server/accountsync/banners.go` | `BannerFile` type, `BuildBannerFiles`, `BannersHandler(token, files)` for `GET /local/sync/banners` |
| `server/accountsync/banners_test.go` | Auth + JSON shape tests |
| `server/internal/cnbootstrap/router.go` | Mount `GET /local/sync/banners` when `SyncToken` set, from `app.resources` paths |
| `mserver/internal/server/default_banner.png` | Checked-in 1×1 PNG (same bytes as intro placeholder) |
| `mserver/internal/server/banners.go` | Local banner root, path map, serve home/gacha, write sync payload, placeholder embed |
| `mserver/internal/server/banners_test.go` | Serve + write + path rejection tests |
| `mserver/internal/server/server.go` | Wire home/gacha to local serve; patch/cpk proxy only; mount launcher banners |
| `mserver/internal/server/sync_http.go` | `POST /local/launcher/banners` forward + write |
| `mserver/internal/sync/client.go` | `FetchBanners(baseURL, token)` |
| `mserver/android/.../TransferActivity.kt` | 「同步图片」button |
| `mserver/android/.../PageRules.kt` (+ test) | Status copy helpers if useful |
| `mserver/android/app/src/main/assets/default-banner.png` | Same 1×1 bytes (APK asset; runtime Go uses embed) |

---

### Task 1: PC `GET /local/sync/banners`

**Files:**
- Create: `server/accountsync/banners.go`
- Create: `server/accountsync/banners_test.go`
- Modify: `server/internal/cnbootstrap/router.go`

**Interfaces:**
- Consumes: disk paths from `app.resources` (HomeBanner, HomeEventBanner, GachaBanner, FiveStarGachaBanner, GachaBanners)
- Produces: `type BannerFile struct { Path, SHA256, Bytes string }` JSON; `func BannersHandler(token string, files []BannerFile, missing int) http.Handler`; `func BuildBannerFiles(entries []BannerSource) (files []BannerFile, missing int)` where `BannerSource` is `{Path string, File string}`

- [ ] **Step 1: Write the failing test**

Create `server/accountsync/banners_test.go`:

```go
package accountsync_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"kairisei.local/server/accountsync"
)

func TestBannersHandlerRequiresTokenAndReturnsFiles(t *testing.T) {
	dir := t.TempDir()
	png := mustDecodePNG(t)
	path := filepath.Join(dir, "banner.png")
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(png)
	files, missing := accountsync.BuildBannerFiles([]accountsync.BannerSource{
		{URLPath: "/local/home/banner.png", File: path},
		{URLPath: "/local/gacha/missing.png", File: filepath.Join(dir, "nope.png")},
	})
	if missing != 1 || len(files) != 1 || files[0].Path != "/local/home/banner.png" {
		t.Fatalf("files=%v missing=%d", files, missing)
	}
	handler := accountsync.BannersHandler("secret", files, missing)

	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/local/sync/banners", nil))
	if denied.Code != http.StatusNotFound {
		t.Fatalf("no token status=%d", denied.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/local/sync/banners", nil)
	request.Header.Set("X-Account-Sync-Token", "secret")
	ok := httptest.NewRecorder()
	handler.ServeHTTP(ok, request)
	if ok.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", ok.Code, ok.Body.String())
	}
	var body struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Bytes  string `json:"bytes"`
		} `json:"files"`
		Missing int `json:"missing"`
	}
	if err := json.Unmarshal(ok.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Missing != 1 || len(body.Files) != 1 || body.Files[0].Path != "/local/home/banner.png" {
		t.Fatalf("%+v", body)
	}
	if body.Files[0].SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha=%s", body.Files[0].SHA256)
	}
	got, err := base64.StdEncoding.DecodeString(body.Files[0].Bytes)
	if err != nil || string(got) != string(png) {
		t.Fatalf("bytes mismatch err=%v", err)
	}
}

func mustDecodePNG(t *testing.T) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd server
go test ./accountsync -count=1 -run TestBannersHandlerRequiresTokenAndReturnsFiles
```

Expected: FAIL (undefined symbols).

- [ ] **Step 3: Implement**

Create `server/accountsync/banners.go`:

```go
package accountsync

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
)

type BannerSource struct {
	URLPath string
	File    string
}

type BannerFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  string `json:"bytes"`
}

func BuildBannerFiles(sources []BannerSource) ([]BannerFile, int) {
	var files []BannerFile
	missing := 0
	for _, source := range sources {
		body, err := os.ReadFile(source.File)
		if err != nil {
			missing++
			continue
		}
		sum := sha256.Sum256(body)
		files = append(files, BannerFile{
			Path:   source.URLPath,
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  base64.StdEncoding.EncodeToString(body),
		})
	}
	return files, missing
}

func BannersHandler(token string, files []BannerFile, missing int) http.Handler {
	if files == nil {
		files = []BannerFile{}
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || token == "" ||
			subtle.ConstantTimeCompare([]byte(request.Header.Get(tokenHeader)), []byte(token)) != 1 {
			http.NotFound(writer, request)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"files": files, "missing": missing})
	})
}
```

In `router.go`, when `SyncToken != ""`, after mounting POST sync routes:

```go
sources := bannerSourcesFromResources(app.resources)
files, missing := accountsync.BuildBannerFiles(sources)
router.Get("/local/sync/banners", accountsync.BannersHandler(app.config.SyncToken, files, missing).ServeHTTP)
```

Add helper in the same package (e.g. `banner_sync.go`):

```go
func bannerSourcesFromResources(resources resourceRuntime) []accountsync.BannerSource {
	out := []accountsync.BannerSource{
		{URLPath: "/local/home/banner.png", File: resources.HomeBanner},
		{URLPath: "/local/home/event-banner.png", File: resources.HomeEventBanner},
		{URLPath: "/local/gacha/banner.png", File: resources.GachaBanner},
		{URLPath: "/local/gacha/five-star-banner.png", File: resources.FiveStarGachaBanner},
	}
	for key, file := range resources.GachaBanners {
		if key == "five_star_ticket" {
			continue
		}
		out = append(out, accountsync.BannerSource{URLPath: "/local/gacha/" + key + ".png", File: file})
	}
	return out
}
```

Use the real field names on whatever struct holds banners (`app.resources` / runtime type). Build the file list **at request time** if paths can change; otherwise at mount is fine for CN static config. Prefer request-time:

```go
router.Get("/local/sync/banners", func(w http.ResponseWriter, r *http.Request) {
	files, missing := accountsync.BuildBannerFiles(bannerSourcesFromResources(app.resources))
	accountsync.BannersHandler(app.config.SyncToken, files, missing).ServeHTTP(w, r)
})
```

- [ ] **Step 4: Run tests**

```bash
cd server
go test ./accountsync -count=1 -run TestBannersHandlerRequiresTokenAndReturnsFiles
go test ./internal/cnbootstrap -count=1 -run Router  # or a focused compile/test if existing router tests cover mount
```

Expected: PASS (accountsync test); cnbootstrap still compiles.

- [ ] **Step 5: Commit**

```bash
git add server/accountsync/banners.go server/accountsync/banners_test.go server/internal/cnbootstrap/router.go server/internal/cnbootstrap/banner_sync.go
git commit -m "Add account-sync banner pack endpoint for phone pull."
```

---

### Task 2: Phone local home/gacha serve + remove from proxy

**Files:**
- Create: `mserver/internal/server/default_banner.png` (write the 1×1 PNG bytes)
- Create: `mserver/internal/server/banners.go`
- Create: `mserver/internal/server/banners_test.go`
- Modify: `mserver/internal/server/server.go`
- Modify: `mserver/internal/server/resource_catalog_test.go` if it assumed home/gacha proxy
- Create: `mserver/android/app/src/main/assets/default-banner.png` (same bytes)

**Interfaces:**
- Consumes: `deps.DataDir`
- Produces: `func bannerRoot(dataDir string) string` → `filepath.Join(dataDir, "banners")`; `func serveLocalBanners(root string) http.Handler`; `func writeBannerFiles(root string, files []bannerFile) (synced int, err error)` (used in Task 3)

- [ ] **Step 1: Write failing tests**

`mserver/internal/server/banners_test.go`:

```go
package server

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHomeBannerServesLocalFileOrPlaceholder(t *testing.T) {
	dir := t.TempDir()
	root := bannerRoot(dir)
	handler := serveLocalBanners(root)

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/local/home/banner.png", nil))
	if missing.Code != http.StatusOK || missing.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("placeholder status=%d type=%s", missing.Code, missing.Header().Get("Content-Type"))
	}
	if got := missing.Body.Bytes(); len(got) < 8 || string(got[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("want PNG magic, got %x", got[:min(8, len(got))])
	}

	local := defaultBannerPNG()
	target := filepath.Join(root, "local", "home", "banner.png")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	// slightly different body still PNG: rewrite same placeholder is fine; mark by serving after write
	if err := os.WriteFile(target, local, 0o600); err != nil {
		t.Fatal(err)
	}
	hit := httptest.NewRecorder()
	handler.ServeHTTP(hit, httptest.NewRequest(http.MethodGet, "/local/home/banner.png", nil))
	if hit.Code != http.StatusOK || hit.Body.Len() != len(local) {
		t.Fatalf("local status=%d len=%d", hit.Code, hit.Body.Len())
	}
}

func TestRejectBannerPathTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := writeBannerFiles(bannerRoot(dir), []bannerSyncFile{{
		Path:  "/local/home/../../etc/passwd",
		Bytes: base64.StdEncoding.EncodeToString(defaultBannerPNG()),
	}})
	if err == nil {
		t.Fatal("expected rejection")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
cd mserver
go test ./internal/server -count=1 -run 'TestHomeBannerServesLocalFileOrPlaceholder|TestRejectBannerPathTraversal'
```

- [ ] **Step 3: Implement**

`banners.go` (sketch):

```go
package server

import (
	_ "embed"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed default_banner.png
var defaultBannerPNGBytes []byte

func defaultBannerPNG() []byte { return defaultBannerPNGBytes }

func bannerRoot(dataDir string) string { return filepath.Join(dataDir, "banners") }

func serveLocalBanners(root string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rel, err := bannerURLToRel(request.URL.Path)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		body := defaultBannerPNG()
		if raw, err := os.ReadFile(path); err == nil && len(raw) >= 8 && string(raw[:8]) == "\x89PNG\r\n\x1a\n" {
			body = raw
		}
		writer.Header().Set("Content-Type", "image/png")
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
		writer.WriteHeader(http.StatusOK)
		if request.Method != http.MethodHead {
			_, _ = writer.Write(body)
		}
	})
}

func bannerURLToRel(urlPath string) (string, error) {
	if strings.Contains(urlPath, "..") {
		return "", errors.New("invalid banner path")
	}
	switch {
	case strings.HasPrefix(urlPath, "/local/home/"):
		return "local/home/" + strings.TrimPrefix(urlPath, "/local/home/"), nil
	case strings.HasPrefix(urlPath, "/local/gacha/"):
		return "local/gacha/" + strings.TrimPrefix(urlPath, "/local/gacha/"), nil
	default:
		return "", errors.New("invalid banner path")
	}
}

type bannerSyncFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  string `json:"bytes"`
}

func writeBannerFiles(root string, files []bannerSyncFile) (int, error) {
	synced := 0
	for _, file := range files {
		rel, err := bannerURLToRel(file.Path)
		if err != nil {
			return synced, err
		}
		raw, err := base64.StdEncoding.DecodeString(file.Bytes)
		if err != nil || len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return synced, err
		}
		tmp := target + ".tmp"
		if err := os.WriteFile(tmp, raw, 0o600); err != nil {
			return synced, err
		}
		if err := os.Rename(tmp, target); err != nil {
			return synced, err
		}
		synced++
	}
	return synced, nil
}
```

Write `default_banner.png` with the known 1×1 PNG bytes (PowerShell/`go` decode). Copy same file to `mserver/android/app/src/main/assets/default-banner.png`.

In `server.go` `NewHandler`:

```go
banners := serveLocalBanners(bannerRoot(deps.DataDir))
mux.Handle("GET /local/home/{path...}", banners)
mux.Handle("HEAD /local/home/{path...}", banners)
mux.Handle("GET /local/gacha/{path...}", banners)
mux.Handle("HEAD /local/gacha/{path...}", banners)
downloads := proxyResourceDownloads(readResourceUpstream(deps.DataDir))
mux.Handle("GET /local/resources/patch/{path...}", downloads)
mux.Handle("HEAD /local/resources/patch/{path...}", downloads)
mux.Handle("GET /local/resources/cpk/{path...}", downloads)
mux.Handle("HEAD /local/resources/cpk/{path...}", downloads)
// do NOT mount home/gacha on downloads
```

- [ ] **Step 4: Run tests**

```bash
cd mserver
go test ./internal/server -count=1 -run 'TestHomeBanner|TestRejectBanner|TestResourceCatalog'
```

Expected: PASS. Adjust catalog test if it expected home/gacha to degrade/proxy.

- [ ] **Step 5: Commit**

```bash
git add mserver/internal/server/banners.go mserver/internal/server/banners_test.go mserver/internal/server/default_banner.png mserver/internal/server/server.go mserver/android/app/src/main/assets/default-banner.png
git commit -m "Serve phone home/gacha banners from local files with PNG placeholder."
```

---

### Task 3: Loopback `POST /local/launcher/banners` + PC fetch

**Files:**
- Modify: `mserver/internal/sync/client.go`
- Create: `mserver/internal/sync/banners_test.go` (optional httptest against fake PC)
- Modify: `mserver/internal/server/sync_http.go`
- Modify: `mserver/internal/server/server.go` (register route)
- Modify: `mserver/internal/server/sync_http_test.go` / new test

**Interfaces:**
- Consumes: Task 1 JSON; Task 2 `writeBannerFiles`
- Produces: `func FetchBanners(baseURL, token string) (files []BannerFile, missing int, err error)` in sync package; launcher returns `{"synced":N,"missing":M}`

- [ ] **Step 1: Failing test**

Append to server tests (or sync_http_test):

```go
func TestLauncherBannersWritesLocalFiles(t *testing.T) {
	dir := t.TempDir()
	png := defaultBannerPNG()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/local/sync/banners" || r.Header.Get("X-Account-Sync-Token") != "secret" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"missing": 0,
			"files": []map[string]string{{
				"path":   "/local/gacha/banner.png",
				"sha256": "x",
				"bytes":  base64.StdEncoding.EncodeToString(png),
			}},
		})
	}))
	t.Cleanup(upstream.Close)
	u, _ := url.Parse(upstream.URL)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)

	handler := NewHandler(Deps{DataDir: dir, DeviceID: "dev", GamePort: 26020})
	body := fmt.Sprintf(`{"host":%q,"port":%d,"token":"secret"}`, host, port)
	req := httptest.NewRequest(http.MethodPost, "/local/launcher/banners", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	stored := filepath.Join(bannerRoot(dir), "local", "gacha", "banner.png")
	got, err := os.ReadFile(stored)
	if err != nil || !bytes.Equal(got, png) {
		t.Fatalf("stored err=%v", err)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
cd mserver
go test ./internal/server -count=1 -run TestLauncherBannersWritesLocalFiles
```

- [ ] **Step 3: Implement**

In `mserver/internal/sync/client.go` (or new `banners.go`):

```go
func FetchBanners(baseURL, token string) (files []struct {
	Path, SHA256, Bytes string
}, missing int, err error) {
	// GET baseURL+"/local/sync/banners" with X-Account-Sync-Token
	// decode JSON {files, missing}
}
```

Follow existing `accountsync` HTTP client patterns (`post` helper style but GET).

In `serveAccountSync` / dedicated function for path `/local/launcher/banners`:

- loopback only
- validate host/port/token like accounts
- `FetchBanners`
- `writeBannerFiles(bannerRoot(deps.DataDir), …)`
- JSON `{"synced": n, "missing": m}`
- on fetch error → 400 with message; do not delete existing banners

Register: `mux.Handle("POST /local/launcher/banners", accountSync)` (same handler switch).

- [ ] **Step 4: Run**

```bash
cd mserver
go test ./internal/server -count=1 -run 'TestLauncherBanners|TestHomeBanner|TestResourceCatalog'
go test ./internal/sync -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mserver/internal/sync mserver/internal/server/sync_http.go mserver/internal/server/server.go mserver/internal/server/*banners*
git commit -m "Forward banner sync from phone launcher to PC account-sync pack."
```

---

### Task 4: Transfer page 「同步图片」button

**Files:**
- Modify: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/TransferActivity.kt`
- Modify: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/PageRules.kt` (optional status helper)
- Modify: `mserver/android/app/src/test/kotlin/local/kairisei/mserver/PageRulesTest.kt`
- Modify: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/SyncClient.kt` only if response fields need parsing (`synced`/`missing`)

**Interfaces:**
- Consumes: `POST /local/launcher/banners` with `{host,port,token}`
- Produces: UI status strings

- [ ] **Step 1: Unit test for status copy (optional but preferred)**

```kotlin
object BannerSyncStatus {
    fun of(code: Int, synced: Int, missing: Int, error: String): String {
        if (code !in 200..299) return if (error.isNotEmpty()) "同步图片失败: $error" else "同步图片失败"
        if (missing > 0) return "已同步 ${synced} 张，缺 ${missing} 张"
        return "已同步 ${synced} 张"
    }
}
```

Test in `PageRulesTest.kt`.

- [ ] **Step 2: Run Android unit test — expect FAIL then implement helper — PASS**

```bat
cd mserver\android
& "C:\Users\17864\.gradle\wrapper\dists\gradle-8.8-all\cgq7lt248yu0xgohgi051w98x\gradle-8.8\bin\gradle.bat" :app:testDebugUnitTest --offline --tests local.kairisei.mserver.PageRulesTest
```

- [ ] **Step 3: Wire TransferActivity**

- Add `Button` labeled `同步图片` near refresh/action area (after token fields / before or after list is fine; place below token, above list).
- On click: if `busy` return; build `TransferRequest.body` from host/port/token; require local port; `busy=true`; background thread `SyncClient.post(localPort, "/local/launcher/banners", body)`; parse `synced`/`missing`/`error` from JSON; `runOnUiThread` set `statusView` via `BannerSyncStatus.of`; `busy=false`.
- Extend `SyncClient.Response` or parse `body` JSONObject for `synced`/`missing` ints.

- [ ] **Step 4: Assemble (smoke)**

```bat
cd mserver\android
& "...\gradle.bat" :app:assembleDebug --offline
```

Expected: BUILD SUCCESSFUL.

- [ ] **Step 5: Commit**

```bash
git add mserver/android
git commit -m "Add transfer-page button to sync home and gacha banners."
```

---

## Spec coverage self-check

| Spec requirement | Task |
|------------------|------|
| PC `/local/sync/banners` + token | Task 1 |
| home/gacha local + placeholder, no upstream | Task 2 |
| patch/cpk proxy-only | Task 2 (unchanged mounts) |
| Launcher forward + write + path/PNG checks | Task 3 |
| Transfer button + status copy | Task 4 |
| Default PNG from existing placeholder bytes | Task 2 |
| No degrader for missing banners | Task 2 |
| No auto sync / no cpk cache | All (YAGNI) |

## Placeholder scan

No TBD steps; concrete paths, commands, and code included. Field names on `app.resources` must match the live `resourceRuntime` / resources struct at implement time (`HomeEventBanner` vs similar).

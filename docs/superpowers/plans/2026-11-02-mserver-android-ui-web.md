# mserver Android Web UI (Scheme A) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the imperative View UI with a pixel-faithful Scheme A Web UI (390×844 baseline) hosted in one `WebView`, wired through a JSON bridge to existing server/sync/transfer/log logic.

**Architecture:** Pure Kotlin builds an `UiSnapshot` and handles bridge commands (unit-tested). `MainActivity` hosts `file:///android_asset/web/index.html`, exposes `AndroidBridge`, and pushes `uiState` via `evaluateJavascript`. Web assets are rebuilt pages (not the PAGX 3512px artboard). Old `TransferActivity` / `LogActivity` UIs are removed after logic moves into the bridge.

**Tech Stack:** Kotlin 17, Android `Activity` + `WebView` (minSdk 26), JUnit 4 unit tests, static HTML/CSS/JS in `app/src/main/assets/web/`, design fonts from `D:\springfield\download\Page 1\assets\fonts\`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-11-02-mserver-android-ui-web-design.md`.
- Visual: Scheme A only — `#0B0D12`, `#22D3C5`, purple nebula, glass cards; design frame **390×844**.
- Pixel fidelity: scale the 390-wide layout to the device; do not freestyle spacing.
- Do not load the PAGX export HTML as the runtime page.
- Do not change Sm/PC borrow/return/accounts/banner protocol semantics.
- Account card map: `mine`→returnable (white), `server`→borrowable (green), `other`→blocked (orange), else→disabled (gray).
- Button copy on transfer: 「借出至本地」 / 「归还到远程」(with selection count when useful).
- Web never opens LAN sockets; only Kotlin/`SyncClient` does.
- Business errors render in-page; notification permission stays native.
- Unit tests: `mserver/android/gradlew.bat :app:testDebugUnitTest --tests <class>` (Windows) or `./gradlew` on Unix.

---

## File map

| File | Responsibility |
|------|----------------|
| `mserver/android/app/src/main/kotlin/.../UiSnapshot.kt` | Snapshot data classes + JSON encode |
| `mserver/android/app/src/main/kotlin/.../BridgeDispatcher.kt` | Pure command→result (port, gates, connection fields validation) |
| `mserver/android/app/src/main/kotlin/.../PageRules.kt` | Extend transfer chrome labels + card kind helpers |
| `mserver/android/app/src/main/kotlin/.../MainActivity.kt` | WebView host, bridge glue, service start/stop, poll snapshot |
| `mserver/android/app/src/main/kotlin/.../WebAppInterface.kt` | `@JavascriptInterface` entry calling dispatcher + side effects |
| `mserver/android/app/src/main/assets/web/index.html` | Shell: splash + tabs + panes |
| `mserver/android/app/src/main/assets/web/css/tokens.css` | Scheme A tokens |
| `mserver/android/app/src/main/assets/web/css/app.css` | Layout matching 390×844 |
| `mserver/android/app/src/main/assets/web/js/bridge.js` | `AndroidBridge` calls + `onUiState` |
| `mserver/android/app/src/main/assets/web/js/app.js` | Routing, render from snapshot |
| `mserver/android/app/src/main/assets/web/fonts/*.woff2` | Copied design fonts |
| `mserver/android/app/src/test/kotlin/.../UiSnapshotTest.kt` | Snapshot JSON shape |
| `mserver/android/app/src/test/kotlin/.../BridgeDispatcherTest.kt` | Command gates |
| `mserver/android/app/src/test/kotlin/.../PageRulesTest.kt` | Updated transfer chrome |
| Delete or gut: `TransferActivity.kt`, `LogActivity.kt`, `LogStrip.kt` usages; manifest entries |

---

### Task 1: UiSnapshot JSON

**Files:**
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/UiSnapshot.kt`
- Create: `mserver/android/app/src/test/kotlin/local/kairisei/mserver/UiSnapshotTest.kt`

**Interfaces:**
- Consumes: none
- Produces: `data class UiSnapshot(...)` with nested `ServerUi`, `ConnectionUi`, `SyncUi`, `TransferUi`, `LogsUi`, `MetaUi`; `fun UiSnapshot.toJson(): String` using `org.json.JSONObject`

- [ ] **Step 1: Write the failing test**

```kotlin
package local.kairisei.mserver

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class UiSnapshotTest {
    @Test
    fun encodesRequiredTopLevelKeys() {
        val json = UiSnapshot(
            server = ServerUi(
                phase = "stopped",
                port = 26020,
                uptimeSec = 0,
                addresses = emptyList(),
                error = "",
            ),
            connection = ConnectionUi(
                host = "192.168.1.24",
                port = 26020,
                token = "secret",
                connected = false,
                lastHeartbeatMs = null,
                error = "",
            ),
            sync = SyncUi(phase = "idle", progress = 0, lastSummary = "", error = ""),
            transfer = TransferUi(
                accounts = emptyList(),
                selected = null,
                busy = false,
                choice = null,
                status = "",
            ),
            logs = LogsUi(recent = emptyList(), full = emptyList()),
            meta = MetaUi(appVersion = "1.0", splashDone = false),
        ).toJson()
        val root = JSONObject(json)
        assertTrue(root.has("server"))
        assertTrue(root.has("connection"))
        assertTrue(root.has("sync"))
        assertTrue(root.has("transfer"))
        assertTrue(root.has("logs"))
        assertTrue(root.has("meta"))
        assertEquals("stopped", root.getJSONObject("server").getString("phase"))
        assertEquals(26020, root.getJSONObject("server").getInt("port"))
        assertFalse(root.getJSONObject("meta").getBoolean("splashDone"))
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `mserver\android\gradlew.bat :app:testDebugUnitTest --tests local.kairisei.mserver.UiSnapshotTest`

Expected: FAIL (unresolved `UiSnapshot` / related types)

- [ ] **Step 3: Write minimal implementation**

Create `UiSnapshot.kt`:

```kotlin
package local.kairisei.mserver

import org.json.JSONArray
import org.json.JSONObject

data class ServerUi(
    val phase: String, // stopped | probing | running
    val port: Int,
    val uptimeSec: Long,
    val addresses: List<String>,
    val error: String,
)

data class ConnectionUi(
    val host: String,
    val port: Int,
    val token: String,
    val connected: Boolean,
    val lastHeartbeatMs: Long?,
    val error: String,
)

data class SyncUi(
    val phase: String, // idle | running | done
    val progress: Int,
    val lastSummary: String,
    val error: String,
)

data class TransferAccountUi(
    val userID: Int,
    val name: String,
    val state: String,
    val card: String, // returnable | borrowable | blocked | disabled
)

data class TransferChoiceUi(
    val serverRevision: Int,
    val phoneRevision: Int,
)

data class TransferUi(
    val accounts: List<TransferAccountUi>,
    val selected: Int?,
    val busy: Boolean,
    val choice: TransferChoiceUi?,
    val status: String,
)

data class LogsUi(
    val recent: List<String>,
    val full: List<String>,
)

data class MetaUi(
    val appVersion: String,
    val splashDone: Boolean,
)

data class UiSnapshot(
    val server: ServerUi,
    val connection: ConnectionUi,
    val sync: SyncUi,
    val transfer: TransferUi,
    val logs: LogsUi,
    val meta: MetaUi,
) {
    fun toJson(): String {
        val root = JSONObject()
        root.put(
            "server",
            JSONObject()
                .put("phase", server.phase)
                .put("port", server.port)
                .put("uptimeSec", server.uptimeSec)
                .put("addresses", JSONArray(server.addresses))
                .put("error", server.error),
        )
        root.put(
            "connection",
            JSONObject()
                .put("host", connection.host)
                .put("port", connection.port)
                .put("token", connection.token)
                .put("connected", connection.connected)
                .put("lastHeartbeatMs", connection.lastHeartbeatMs ?: JSONObject.NULL)
                .put("error", connection.error),
        )
        root.put(
            "sync",
            JSONObject()
                .put("phase", sync.phase)
                .put("progress", sync.progress)
                .put("lastSummary", sync.lastSummary)
                .put("error", sync.error),
        )
        val accounts = JSONArray()
        for (a in transfer.accounts) {
            accounts.put(
                JSONObject()
                    .put("userID", a.userID)
                    .put("name", a.name)
                    .put("state", a.state)
                    .put("card", a.card),
            )
        }
        val transferJson = JSONObject()
            .put("accounts", accounts)
            .put("selected", transfer.selected ?: JSONObject.NULL)
            .put("busy", transfer.busy)
            .put("status", transfer.status)
        if (transfer.choice == null) {
            transferJson.put("choice", JSONObject.NULL)
        } else {
            transferJson.put(
                "choice",
                JSONObject()
                    .put("serverRevision", transfer.choice.serverRevision)
                    .put("phoneRevision", transfer.choice.phoneRevision),
            )
        }
        root.put("transfer", transferJson)
        root.put(
            "logs",
            JSONObject()
                .put("recent", JSONArray(logs.recent))
                .put("full", JSONArray(logs.full)),
        )
        root.put(
            "meta",
            JSONObject()
                .put("appVersion", meta.appVersion)
                .put("splashDone", meta.splashDone),
        )
        return root.toString()
    }
}

object AccountCardKind {
    fun of(state: String): String = when (state) {
        "mine" -> "returnable"
        "server" -> "borrowable"
        "other" -> "blocked"
        else -> "disabled"
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `mserver\android\gradlew.bat :app:testDebugUnitTest --tests local.kairisei.mserver.UiSnapshotTest`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/android/app/src/main/kotlin/local/kairisei/mserver/UiSnapshot.kt \
  mserver/android/app/src/test/kotlin/local/kairisei/mserver/UiSnapshotTest.kt
git commit -m "Add UiSnapshot JSON model for the Web UI bridge."
```

---

### Task 2: BridgeDispatcher command gates

**Files:**
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/BridgeDispatcher.kt`
- Create: `mserver/android/app/src/test/kotlin/local/kairisei/mserver/BridgeDispatcherTest.kt`

**Interfaces:**
- Consumes: `PortText.parse`, `SyncTarget`, `TransferRequest.body`
- Produces: `data class BridgeResult(val ok: Boolean, val error: String = "")`; `object BridgeDispatcher` with pure validators used before side effects: `setPort`, `requireServerRunning`, `requireConnectionFields`, `parseConnection`

- [ ] **Step 1: Write the failing test**

```kotlin
package local.kairisei.mserver

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class BridgeDispatcherTest {
    @Test
    fun setPortRejectsInvalid() {
        assertFalse(BridgeDispatcher.setPort("0").ok)
        assertEquals("本机端口无效", BridgeDispatcher.setPort("0").error)
        assertTrue(BridgeDispatcher.setPort("26020").ok)
    }

    @Test
    fun connectRequiresServerAndFields() {
        assertEquals("请先启动本地服务", BridgeDispatcher.requireServerRunning(false).error)
        assertTrue(BridgeDispatcher.requireServerRunning(true).ok)
        assertFalse(BridgeDispatcher.parseConnection("bad/host", "26020", "tok").ok)
        assertTrue(BridgeDispatcher.parseConnection("10.0.0.1", "26020", "tok").ok)
    }

    @Test
    fun syncRequiresConnected() {
        assertEquals("未连接远程服务", BridgeDispatcher.requireConnected(false).error)
        assertTrue(BridgeDispatcher.requireConnected(true).ok)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `mserver\android\gradlew.bat :app:testDebugUnitTest --tests local.kairisei.mserver.BridgeDispatcherTest`

Expected: FAIL (unresolved `BridgeDispatcher`)

- [ ] **Step 3: Write minimal implementation**

```kotlin
package local.kairisei.mserver

data class BridgeResult(val ok: Boolean, val error: String = "") {
    companion object {
        fun ok() = BridgeResult(true, "")
        fun err(message: String) = BridgeResult(false, message)
    }
}

data class ParsedConnection(val host: String, val port: Int, val token: String)

object BridgeDispatcher {
    fun setPort(text: String): BridgeResult {
        if (PortText.parse(text) == null) return BridgeResult.err("本机端口无效")
        return BridgeResult.ok()
    }

    fun requireServerRunning(running: Boolean): BridgeResult {
        if (!running) return BridgeResult.err("请先启动本地服务")
        return BridgeResult.ok()
    }

    fun requireConnected(connected: Boolean): BridgeResult {
        if (!connected) return BridgeResult.err("未连接远程服务")
        return BridgeResult.ok()
    }

    fun parseConnection(host: String, portText: String, token: String): BridgeResult {
        val body = TransferRequest.body(host, portText, token)
            ?: return BridgeResult.err("电脑地址、端口或口令无效")
        return BridgeResult.ok()
    }

    fun connectionOrNull(host: String, portText: String, token: String): ParsedConnection? {
        val body = TransferRequest.body(host, portText, token) ?: return null
        return ParsedConnection(body.host, body.port, body.token)
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `mserver\android\gradlew.bat :app:testDebugUnitTest --tests local.kairisei.mserver.BridgeDispatcherTest`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/android/app/src/main/kotlin/local/kairisei/mserver/BridgeDispatcher.kt \
  mserver/android/app/src/test/kotlin/local/kairisei/mserver/BridgeDispatcherTest.kt
git commit -m "Add pure bridge command gates for Web UI."
```

---

### Task 3: Transfer chrome labels (design copy)

**Files:**
- Modify: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/PageRules.kt`
- Modify: `mserver/android/app/src/test/kotlin/local/kairisei/mserver/PageRulesTest.kt`

**Interfaces:**
- Consumes: `AccountRow`, `AccountCardKind`
- Produces: updated `TransferAction.of` texts; `fun TransferAction.withCount(n: Int): TransferAction` for 「归还到远程（n）」

- [ ] **Step 1: Write the failing test**

Update `buttonFollowsSelection` in `PageRulesTest.kt`:

```kotlin
@Test
fun buttonFollowsSelection() {
    assertEquals(TransferAction("归还到远程", false, ""), TransferAction.of(null))
    assertEquals(TransferAction("借出至本地", true, ""), TransferAction.of(AccountRow(1, "秋", "server")))
    assertEquals(TransferAction("归还到远程", true, ""), TransferAction.of(AccountRow(1, "秋", "mine")))
    assertEquals(
        TransferAction("归还到远程", false, "该账号在别的设备上"),
        TransferAction.of(AccountRow(1, "秋", "other")),
    )
    assertEquals(TransferAction("归还到远程", false, ""), TransferAction.of(AccountRow(1, "秋", "weird")))
    assertEquals(
        TransferAction("归还到远程（1）", true, ""),
        TransferAction.of(AccountRow(1, "秋", "mine")).withCount(1),
    )
    assertEquals("returnable", AccountCardKind.of("mine"))
    assertEquals("borrowable", AccountCardKind.of("server"))
    assertEquals("blocked", AccountCardKind.of("other"))
    assertEquals("disabled", AccountCardKind.of("x"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `mserver\android\gradlew.bat :app:testDebugUnitTest --tests local.kairisei.mserver.PageRulesTest`

Expected: FAIL on assertion text mismatch

- [ ] **Step 3: Write minimal implementation**

In `PageRules.kt`, change `TransferAction`:

```kotlin
data class TransferAction(val text: String, val enabled: Boolean, val note: String) {
    fun withCount(n: Int): TransferAction {
        if (!enabled || !text.startsWith("归还到远程")) return this
        return copy(text = "归还到远程（$n）")
    }

    companion object {
        fun of(row: AccountRow?): TransferAction {
            if (row == null) return TransferAction("归还到远程", false, "")
            return when (row.state) {
                "server" -> TransferAction("借出至本地", true, "")
                "mine" -> TransferAction("归还到远程", true, "")
                "other" -> TransferAction("归还到远程", false, "该账号在别的设备上")
                else -> TransferAction("归还到远程", false, "")
            }
        }
    }
}
```

(`AccountCardKind` already added in Task 1.)

- [ ] **Step 4: Run test to verify it passes**

Run: `mserver\android\gradlew.bat :app:testDebugUnitTest --tests local.kairisei.mserver.PageRulesTest`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mserver/android/app/src/main/kotlin/local/kairisei/mserver/PageRules.kt \
  mserver/android/app/src/test/kotlin/local/kairisei/mserver/PageRulesTest.kt
git commit -m "Align transfer button copy with Scheme A chrome."
```

---

### Task 4: Web asset shell (Splash + TabBar)

**Files:**
- Create: `mserver/android/app/src/main/assets/web/index.html`
- Create: `mserver/android/app/src/main/assets/web/css/tokens.css`
- Create: `mserver/android/app/src/main/assets/web/css/app.css`
- Create: `mserver/android/app/src/main/assets/web/js/bridge.js`
- Create: `mserver/android/app/src/main/assets/web/js/app.js`
- Create: `mserver/android/app/src/main/assets/web/fonts/` (copy woff2 from design)

**Interfaces:**
- Consumes: future `window.onUiState(json)` from Kotlin
- Produces: DOM with `#splash`, `#app` (hidden until splash done), tabs `service|connect|sync|logs`, `window.WebApp` render API

- [ ] **Step 1: Copy fonts**

```powershell
New-Item -ItemType Directory -Force -Path "mserver\android\app\src\main\assets\web\fonts"
Copy-Item "D:\springfield\download\Page 1\assets\fonts\*.woff2" "mserver\android\app\src\main\assets\web\fonts\"
```

Measure which woff2 is Noto Sans SC Bold/Regular from the design CSS `@font-face` blocks; wire those filenames in `tokens.css`. If unsure, map `font_f1.woff2` (largest) as primary SC and adjust after visual check.

- [ ] **Step 2: Add tokens + shell HTML/CSS/JS**

`tokens.css` must define at least:

```css
:root {
  --bg: #0B0D12;
  --cyan: #22D3C5;
  --frame-w: 390px;
  --frame-h: 844px;
  --font-sc: "Noto Sans SC", sans-serif;
}
```

`index.html` structure (single page):

- `#splash` with logo mark 「S」, title, 「本地服务管家」, 「正在初始化本地服务环境…」
- `#app` with `#pane-service`, `#pane-connect`, `#pane-sync`, `#pane-logs`, `#tabbar` (服务/连接/同步/日志)
- Script order: `bridge.js` then `app.js`

`bridge.js`:

```javascript
window.Bridge = {
  call(name, payload) {
    const raw = prompt == null; // placeholder replaced below
    if (window.AndroidBridge && window.AndroidBridge.command) {
      return JSON.parse(window.AndroidBridge.command(name, JSON.stringify(payload || {})));
    }
    return { ok: false, error: "bridge missing" };
  }
};
window.onUiState = function (json) {
  window.WebApp && window.WebApp.applyState(typeof json === "string" ? JSON.parse(json) : json);
};
```

`app.js`: splash auto-hides after short timeout **and** calls `Bridge.call("splashFinished", {})`; tab switching only toggles panes; `applyState` updates `data-phase` attributes for CSS.

Scale: wrap UI in `.frame` width 390px; `html, body` background `#0B0D12`; use `transform: scale(min(100vw/390, 100vh/844))` centered so proportions stay.

- [ ] **Step 3: Manual open check (optional browser)**

Open `index.html` in a browser; confirm splash → tabs switch with empty panes and Scheme A background. No Gradle test required for static markup.

- [ ] **Step 4: Commit**

```bash
git add mserver/android/app/src/main/assets/web
git commit -m "Add Scheme A Web shell with splash and tab bar."
```

---

### Task 5: MainActivity WebView host + snapshot push

**Files:**
- Modify: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt` (replace View UI with WebView)
- Create: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/WebAppInterface.kt`

**Interfaces:**
- Consumes: `UiSnapshot`, `BridgeDispatcher`, existing file keys `server-port`, `server-running`, `server-error`, `server-host`, `remote-port`, `sync-token`
- Produces: `WebAppInterface.command(name, payloadJson): String`; `MainActivity.pushState()` calling `webView.evaluateJavascript("onUiState(...)", null)`

- [ ] **Step 1: Implement WebAppInterface**

```kotlin
package local.kairisei.mserver

import android.webkit.JavascriptInterface
import org.json.JSONObject

class WebAppInterface(private val host: MainActivity) {
    @JavascriptInterface
    fun command(name: String, payloadJson: String): String {
        val payload = runCatching { JSONObject(payloadJson) }.getOrElse { JSONObject() }
        val result = host.handleBridge(name, payload)
        return JSONObject().put("ok", result.ok).put("error", result.error).toString()
    }
}
```

- [ ] **Step 2: Rewrite MainActivity onCreate to WebView**

Replace the LinearLayout UI with:

```kotlin
WebView.setWebContentsDebuggingEnabled(true) // debug builds only if easy; else always ok for local tool
val web = WebView(this)
web.settings.javaScriptEnabled = true
web.settings.domStorageEnabled = true
web.addJavascriptInterface(WebAppInterface(this), "AndroidBridge")
web.loadUrl("file:///android_asset/web/index.html")
setContentView(web)
this.webView = web
```

Keep the 2s `refresh` runnable: build `UiSnapshot` from files + in-memory transfer/sync fields, `pushState()`.

`handleBridge` stub for Task 5:

- `splashFinished` → set `splashDone = true`
- `setPort` → `BridgeDispatcher.setPort` then write `server-port` if ok and not running
- `startServer` / `stopServer` → existing start/stop logic from current MainActivity
- other names → `BridgeResult.err("not implemented")` until later tasks

Server phase mapping: `HomeState` → `stopped` / `probing` / `running` (`未启动`/`监测中`/`已启动`).

- [ ] **Step 3: Manual verify**

Install debug APK, launch: splash appears, then tabs; server pane may be empty but `uiState` should arrive (add temporary `console` via toast or log if needed).

- [ ] **Step 4: Commit**

```bash
git add mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt \
  mserver/android/app/src/main/kotlin/local/kairisei/mserver/WebAppInterface.kt
git commit -m "Host Scheme A Web UI in MainActivity with bridge stub."
```

---

### Task 6: Service tab UI + wiring

**Files:**
- Modify: `mserver/android/app/src/main/assets/web/js/app.js`
- Modify: `mserver/android/app/src/main/assets/web/css/app.css`
- Modify: `mserver/android/app/src/main/assets/web/index.html` (`#pane-service` markup)
- Modify: `mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt` (addresses in snapshot)

**Interfaces:**
- Consumes: `server.phase|port|error|addresses`
- Produces: Web calls `setPort`, `startServer`, `stopServer`; shows notification preview card (static visual per design when `running`)

- [ ] **Step 1: Markup service pane**

Include: title「服务控制」, subtitle「本地后端服务管理」, status card, PORT field, primary button, hint about notification, notification preview block, LogBar strip.

- [ ] **Step 2: Render + actions in app.js**

On `applyState`: set status text from phase (`stopped`→未启动 / `probing`→监测中 / `running`→服务运行中); disable port when not stopped; button label 启动/监测中/停止服务 matching `HomeState` rules (monitoring cannot stop).

- [ ] **Step 3: Ensure snapshot includes listen addresses**

Reuse existing `/local/launcher/addresses` fetch; put formatted `AddressLine.copyText` strings into `server.addresses`.

- [ ] **Step 4: Manual test**

Start/stop service from Web UI; notification stop still works; port validation shows「本机端口无效」.

- [ ] **Step 5: Commit**

```bash
git add mserver/android/app/src/main/assets/web \
  mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt
git commit -m "Wire service tab to ServerService through the Web bridge."
```

---

### Task 7: Connection tab

**Files:**
- Modify: web `index.html` / `app.js` / `app.css` for `#pane-connect`
- Modify: `MainActivity.kt` — persist host/port/token; `connect`/`disconnect`/`ping`/`saveConnection`

**Interfaces:**
- Consumes: files `server-host`, `remote-port`, `sync-token`
- Produces: in-memory `connected` + `lastHeartbeatMs`; `ping` measures RTT via loopback launcher or TCP/HTTP check already available — use `SyncClient.post(localPort, "/local/launcher/accounts", minimalBody)` **or** a lightweight GET if one exists; on failure set `connection.error` and append log line. First version: `connect` validates fields + `requireServerRunning`, sets `connected=true` after successful accounts probe (read-only); `disconnect` clears `connected`; `ping` repeats probe and sets `lastHeartbeatMs`.

- [ ] **Step 1: Bridge handlers**

```kotlin
"saveConnection" -> {
  val host = payload.optString("host")
  val port = payload.optString("port")
  val token = payload.optString("token")
  val parsed = BridgeDispatcher.connectionOrNull(host, port, token)
    ?: return BridgeDispatcher.parseConnection(host, port, token)
  writeText("server-host", parsed.host)
  writeText("remote-port", parsed.port.toString())
  writeText("sync-token", parsed.token)
  BridgeResult.ok()
}
```

`connect` / `ping`: gate with `requireServerRunning` + parse; background thread `SyncClient.post`; on 2xx set connected + heartbeat; on failure `BridgeResult` / snapshot error「没有连上本机服务」or existing messages.

- [ ] **Step 2: Web form**

Fields IP / PORT / TOKEN; buttons 断开连接 / 测试心跳; status card「长连接已建立 · 心跳 Nms」vs empty.

- [ ] **Step 3: Manual test**

With server running and PC reachable: connect succeeds; sync tab leaves empty-state.

- [ ] **Step 4: Commit**

```bash
git add mserver/android/app/src/main/assets/web \
  mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt
git commit -m "Add connection tab with save, connect, and ping."
```

---

### Task 8: Sync tab (data sync + account transfer)

**Files:**
- Modify: web panes for sync segment control「数据同步 | 账号移交」
- Modify: `MainActivity.kt` — port logic from `TransferActivity` (list/borrow/return/choice/banners)

**Interfaces:**
- Consumes: `SyncClient`, `TransferRequest`, `AccountList`, `BannerSyncStatus`, `TransferAction`
- Produces: bridge commands `startSync` (= banner sync `POST /local/launcher/banners`), `cancelSync` (no-op or flag if request in flight — if cancel impossible, return ok and ignore), `refreshTransfer`, `selectAccount`, `transferAction` (`kind`: borrow|return, optional `choice` phone|server)

- [ ] **Step 1: Move TransferActivity networking into MainActivity helpers**

Keep behavior identical:

- List: `POST /local/launcher/accounts` with device body as today
- Borrow/return paths and 409 choice UI fields in snapshot `transfer.choice`
- Banner sync status → `sync.lastSummary` via `BannerSyncStatus.of`

Map accounts through `AccountCardKind.of(state)`.

- [ ] **Step 2: Empty states**

If `!connection.connected`, both sync segments show design empty copy; bridge commands return `未连接远程服务` without HTTP.

- [ ] **Step 3: Web render**

Account cards by `card` class; primary button uses `TransferAction.of(selectedRow).withCount(1)`; choice buttons when `transfer.choice != null`.

- [ ] **Step 4: Manual test**

List accounts, borrow, return, 409 keep phone/server, sync banners.

- [ ] **Step 5: Commit**

```bash
git add mserver/android/app/src/main/assets/web \
  mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt
git commit -m "Wire sync and account transfer into the Web UI."
```

---

### Task 9: Logs tab + LogBar + clear

**Files:**
- Modify: web `#pane-logs`, LogBar on service/connect
- Modify: `MainActivity.kt` — `fetchLogs`, `clearLogs`
- Optionally keep reading `filesDir/server.log`; `LogTail.latest` for recent three lines split into `logs.recent`

**Interfaces:**
- Consumes: `LogTail`, `server.log`
- Produces: `clearLogs` truncates file (same as current start-clear behavior for explicit clear); `fetchLogs` fills `logs.full`

- [ ] **Step 1: Bridge**

```kotlin
"clearLogs" -> {
  File(filesDir, "server.log").writeText("")
  BridgeResult.ok()
}
"fetchLogs" -> {
  // full list loaded on next snapshot from file lines
  BridgeResult.ok()
}
```

Always include `full` lines in snapshot when on logs tab **or** always include last N (cap e.g. 500) to keep simple.

- [ ] **Step 2: Web**

LogBar click /「展开完整日志」→ `WebApp.showTab('logs')`. Clear button calls `clearLogs`.

- [ ] **Step 3: Manual test**

Generate logs via start/connect; clear; LogBar navigation.

- [ ] **Step 4: Commit**

```bash
git add mserver/android/app/src/main/assets/web \
  mserver/android/app/src/main/kotlin/local/kairisei/mserver/MainActivity.kt
git commit -m "Add logs tab and LogBar navigation for Web UI."
```

---

### Task 10: Retire old Activities + visual QA

**Files:**
- Delete: `TransferActivity.kt`, `LogActivity.kt` (and `LogStrip.kt` if unused)
- Modify: `AndroidManifest.xml` — remove those activities
- Modify: any remaining references
- Run full unit tests

**Interfaces:**
- Consumes: completed Web paths
- Produces: single-activity app

- [ ] **Step 1: Remove manifest entries and dead Kotlin UI files**

Ensure no `startActivity` to transfer/log remains.

- [ ] **Step 2: Run all Android unit tests**

Run: `mserver\android\gradlew.bat :app:testDebugUnitTest`

Expected: PASS

- [ ] **Step 3: Visual QA checklist (manual)**

Compare against Page 1 Scheme A frames (390×844):

1. Splash  
2. Service running + notification preview  
3. Connect connected + heartbeat  
4. Sync in progress / empty  
5. Transfer list cards (four card kinds)  
6. Full logs  
7. Tab active cyan states  

Keep screenshots next to design for review.

- [ ] **Step 4: Commit**

```bash
git add -A mserver/android
git commit -m "Retire View-based transfer and log activities."
```

---

## Self-review (author)

1. **Spec coverage:** Splash, four tabs, transfer-in-sync, logs, bridge commands, Scheme A tokens, 390×844, retire old UIs, tests — each has a task. Auto-retry on disconnect is explicitly minimal (error + log) in Task 7, matching spec.  
2. **Placeholders:** None intentional; font file mapping may need one visual adjust after copy.  
3. **Types:** `UiSnapshot` / `BridgeResult` / `AccountCardKind` / `TransferAction.withCount` names are consistent across tasks.

---

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-11-02-mserver-android-ui-web.md`. Two execution options:

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks  
**2. Inline Execution** — execute tasks in this session with checkpoints  

Which approach?

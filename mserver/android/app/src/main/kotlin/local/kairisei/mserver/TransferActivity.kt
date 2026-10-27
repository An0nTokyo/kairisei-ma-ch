package local.kairisei.mserver

import android.app.Activity
import android.graphics.Color
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.text.Editable
import android.text.TextWatcher
import android.view.MotionEvent
import android.view.View
import android.view.ViewGroup
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import org.json.JSONObject
import java.io.File
import java.nio.charset.StandardCharsets

class TransferActivity : Activity() {
    private val handler = Handler(Looper.getMainLooper())
    private lateinit var hostField: EditText
    private lateinit var remotePortField: EditText
    private lateinit var tokenField: EditText
    private lateinit var searchField: EditText
    private lateinit var listScroll: ScrollView
    private lateinit var listRows: LinearLayout
    private lateinit var refreshButton: Button
    private lateinit var syncBannersButton: Button
    private lateinit var actionButton: Button
    private lateinit var noteView: TextView
    private lateinit var choiceLine: TextView
    private lateinit var keepPhone: Button
    private lateinit var keepServer: Button
    private lateinit var statusView: TextView
    private lateinit var logStrip: TextView
    private var writingSync = false
    private var rows: List<AccountRow> = emptyList()
    private var selected: Int? = null
    private var revealed = false
    private var busy = false
    private var pullArmed = false
    private var pullStart = 0f

    private val refresh = object : Runnable {
        override fun run() {
            if (readText("server-running") != "1") {
                finish()
                return
            }
            logStrip.showLatestLog(File(filesDir, "server.log"))
            handler.postDelayed(this, 2000)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        title = "移交"
        hostField = EditText(this)
        remotePortField = EditText(this)
        tokenField = EditText(this)
        searchField = EditText(this).apply { hint = "搜索账号或用户名" }
        listRows = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            minimumHeight = (160 * resources.displayMetrics.density).toInt()
            setOnClickListener {
                if (visibleRows().isEmpty()) {
                    revealed = true
                    renderList()
                }
            }
        }
        listScroll = ScrollView(this).apply {
            addView(listRows)
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                0,
                1f,
            )
            setOnTouchListener { _, event -> trackPull(event) }
        }
        refreshButton = Button(this).apply {
            text = "刷新"
            setOnClickListener { fetchList(false) }
        }
        syncBannersButton = Button(this).apply {
            text = "同步图片"
            setOnClickListener { syncBanners() }
        }
        actionButton = Button(this)
        noteView = TextView(this)
        choiceLine = TextView(this)
        keepPhone = Button(this).apply {
            text = "保留手机进度"
            setOnClickListener { submit("return", "phone") }
        }
        keepServer = Button(this).apply {
            text = "保留电脑进度"
            setOnClickListener { submit("return", "server") }
        }
        statusView = TextView(this)
        logStrip = createLogStrip()
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(32, 32, 32, 0)
            addView(TextView(this@TransferActivity).apply { text = "电脑地址" })
            addView(hostField)
            addView(TextView(this@TransferActivity).apply { text = "电脑端口" })
            addView(remotePortField)
            addView(TextView(this@TransferActivity).apply { text = "同步口令" })
            addView(tokenField)
            addView(syncBannersButton)
            addView(searchField)
            addView(listScroll)
            addView(actionButton)
            addView(noteView)
            addView(choiceLine)
            addView(keepPhone)
            addView(keepServer)
            addView(statusView)
            addView(logStrip)
        }
        setContentView(root)
        actionButton.setOnClickListener {
            val row = selectedVisible() ?: return@setOnClickListener
            val action = TransferAction.of(row)
            if (!action.enabled) return@setOnClickListener
            submit(if (row.state == "mine") "return" else "borrow", "")
        }
        searchField.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) = Unit
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) = Unit
            override fun afterTextChanged(s: Editable?) {
                renderList()
            }
        })
        writingSync = true
        hostField.setText(readText("server-host"))
        remotePortField.setText(readText("remote-port").ifEmpty { "26020" })
        tokenField.setText(readText("sync-token"))
        writingSync = false
        saveSync(hostField, "server-host")
        saveSync(remotePortField, "remote-port")
        saveSync(tokenField, "sync-token")
        hideChoice()
        renderList()
        fetchList(false)
    }

    override fun onStart() {
        super.onStart()
        handler.post(refresh)
    }

    override fun onStop() {
        handler.removeCallbacks(refresh)
        super.onStop()
    }

    private fun trackPull(event: MotionEvent): Boolean {
        val threshold = 48f * resources.displayMetrics.density
        when (event.action) {
            MotionEvent.ACTION_DOWN -> {
                pullArmed = listScroll.scrollY == 0
                pullStart = event.y
            }
            MotionEvent.ACTION_MOVE -> if (listScroll.scrollY != 0) pullArmed = false
            MotionEvent.ACTION_UP -> {
                val pulled = pullArmed && listScroll.scrollY == 0 && event.y - pullStart > threshold
                pullArmed = false
                if (pulled && AccountRefresh.pulls(visibleRows().size) && !busy) fetchList(false)
            }
        }
        return false
    }

    private fun submit(kind: String, choice: String) {
        if (busy) return
        val row = selectedVisible() ?: return
        hideChoice()
        val target = TransferRequest.body(
            hostField.text.toString(),
            remotePortField.text.toString(),
            tokenField.text.toString(),
        )
        if (target == null) {
            failLocal("电脑地址、端口或口令无效")
            return
        }
        val localPort = PortText.parse(readText("server-port")) ?: run {
            failLocal("没有连上本机服务")
            return
        }
        val path = if (kind == "borrow") "/local/launcher/borrow" else "/local/launcher/return"
        val body = JSONObject()
            .put("host", target.host)
            .put("port", target.port)
            .put("token", target.token)
            .put("user_id", row.userID)
            .put("choice", choice)
            .toString()
        busy = true
        renderAction()
        Thread {
            val result = runCatching { SyncClient.post(localPort, path, body) }.getOrNull()
            handler.post { showTransfer(row.userID, result) }
        }.start()
    }

    private fun showTransfer(userID: Int, result: SyncClient.Response?) {
        busy = false
        if (result == null) {
            failLocal("没有连上本机服务")
            renderAction()
            return
        }
        if (result.code == 409 && result.error == "choice required") {
            choiceLine.text = "电脑进度版本 ${result.serverRevision}，手机进度版本 ${result.phoneRevision}"
            choiceLine.visibility = View.VISIBLE
            keepPhone.visibility = View.VISIBLE
            keepServer.visibility = View.VISIBLE
            statusView.text = ""
        } else if (result.code == 200) {
            hideChoice()
            statusView.text = "完成"
        } else {
            hideChoice()
            statusView.text = "失败 $userID: ${result.error}"
        }
        renderAction()
        fetchList(true)
    }

    private fun syncBanners() {
        if (busy) return
        val target = TransferRequest.body(
            hostField.text.toString(),
            remotePortField.text.toString(),
            tokenField.text.toString(),
        )
        if (target == null) {
            failLocal("电脑地址、端口或口令无效")
            return
        }
        val localPort = PortText.parse(readText("server-port")) ?: run {
            failLocal("没有连上本机服务")
            return
        }
        val body = JSONObject()
            .put("host", target.host)
            .put("port", target.port)
            .put("token", target.token)
            .toString()
        busy = true
        renderAction()
        Thread {
            val result = runCatching {
                SyncClient.post(localPort, "/local/launcher/banners", body)
            }.getOrNull()
            handler.post { applyBannerSync(result) }
        }.start()
    }

    private fun applyBannerSync(result: SyncClient.Response?) {
        busy = false
        if (result == null) {
            failLocal("没有连上本机服务")
            return
        }
        val json = runCatching { JSONObject(result.body) }.getOrNull()
        val synced = json?.optInt("synced") ?: 0
        val missing = json?.optInt("missing") ?: 0
        statusView.text = BannerSyncStatus.of(result.code, synced, missing, result.error)
        renderAction()
    }

    private fun fetchList(keepStatus: Boolean) {
        if (busy) return
        val target = TransferRequest.body(
            hostField.text.toString(),
            remotePortField.text.toString(),
            tokenField.text.toString(),
        )
        if (target == null) {
            failLocal("电脑地址、端口或口令无效")
            return
        }
        val localPort = PortText.parse(readText("server-port")) ?: run {
            failLocal("没有连上本机服务")
            return
        }
        val body = JSONObject()
            .put("host", target.host)
            .put("port", target.port)
            .put("token", target.token)
            .toString()
        busy = true
        renderAction()
        Thread {
            val result = runCatching {
                SyncClient.post(localPort, "/local/launcher/accounts", body)
            }.getOrNull()
            handler.post { applyList(result, keepStatus) }
        }.start()
    }

    private fun applyList(result: SyncClient.Response?, keepStatus: Boolean) {
        busy = false
        if (result == null) {
            failLocal("没有连上本机服务")
            renderList()
            return
        }
        if (result.code != 200) {
            val reason = result.error.ifEmpty { "account sync failed" }
            statusView.text = "账号列表失败: $reason"
            val kept = AccountList.applyRefresh(rows, selected, null)
            rows = kept.rows
            selected = kept.selected
            renderList()
            return
        }
        val fetched = parseAccounts(result.body)
        if (fetched == null) {
            statusView.text = "账号列表失败: account sync failed"
            renderList()
            return
        }
        val applied = AccountList.applyRefresh(rows, selected, fetched)
        rows = applied.rows
        selected = applied.selected
        if (visibleRows().isNotEmpty()) revealed = false
        if (!keepStatus) statusView.text = ""
        renderList()
    }

    private fun parseAccounts(body: String): List<AccountRow>? {
        val json = runCatching { JSONObject(body) }.getOrNull() ?: return null
        val array = json.optJSONArray("accounts") ?: return null
        val parsed = mutableListOf<AccountRow>()
        for (index in 0 until array.length()) {
            val row = array.optJSONObject(index) ?: return null
            parsed.add(AccountRow(row.optInt("user_id"), row.optString("name"), row.optString("state")))
        }
        return parsed
    }

    private fun renderList() {
        val visible = visibleRows()
        listRows.removeAllViews()
        if (AccountRefresh.button(visible.size, revealed)) listRows.addView(refreshButton)
        for (row in visible) {
            val chosen = row.userID == selected
            listRows.addView(TextView(this).apply {
                text = row.label()
                setPadding(24, 20, 24, 20)
                setTextColor(if (row.state == "server") Color.BLACK else Color.WHITE)
                background = GradientDrawable().apply {
                    setColor(if (row.state == "server") Color.WHITE else Color.parseColor("#1F7A3A"))
                    cornerRadius = 12f
                    if (chosen) setStroke((2 * resources.displayMetrics.density).toInt(), Color.parseColor("#8AB4FF"))
                }
                setOnClickListener {
                    selected = row.userID
                    hideChoice()
                    renderList()
                }
            })
        }
        renderAction()
    }

    private fun renderAction() {
        syncBannersButton.isEnabled = !busy
        val action = TransferAction.of(selectedVisible())
        actionButton.text = action.text
        actionButton.isEnabled = action.enabled && !busy
        noteView.text = action.note
    }

    private fun visibleRows(): List<AccountRow> = rows.filter { it.matches(searchField.text?.toString().orEmpty()) }

    private fun selectedVisible(): AccountRow? {
        val id = selected ?: return null
        return visibleRows().find { it.userID == id }
    }

    private fun hideChoice() {
        choiceLine.visibility = View.GONE
        keepPhone.visibility = View.GONE
        keepServer.visibility = View.GONE
    }

    private fun failLocal(message: String) {
        statusView.text = message
        File(filesDir, "server.log").appendText(message + "\n", StandardCharsets.UTF_8)
        logStrip.showLatestLog(File(filesDir, "server.log"))
    }

    private fun saveSync(field: EditText, name: String) {
        field.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) = Unit
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) = Unit
            override fun afterTextChanged(s: Editable?) {
                if (writingSync) return
                writeText(name, s?.toString()?.trim().orEmpty())
            }
        })
    }

    private fun readText(name: String): String {
        val file = File(filesDir, name)
        if (!file.exists()) return ""
        return file.readText(StandardCharsets.UTF_8).trim()
    }

    private fun writeText(name: String, value: String) {
        File(filesDir, name).writeText(value, StandardCharsets.UTF_8)
    }
}

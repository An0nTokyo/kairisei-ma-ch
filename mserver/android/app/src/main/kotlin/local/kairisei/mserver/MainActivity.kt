package local.kairisei.mserver

import android.app.Activity
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Intent
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.text.Editable
import android.text.TextWatcher
import android.view.View
import android.view.ViewGroup
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.net.URL
import java.nio.charset.StandardCharsets

class MainActivity : Activity() {
    private val handler = Handler(Looper.getMainLooper())
    private lateinit var portField: EditText
    private lateinit var actionButton: Button
    private lateinit var errorView: TextView
    private lateinit var addressList: LinearLayout
    private lateinit var hostField: EditText
    private lateinit var remotePortField: EditText
    private lateinit var tokenField: EditText
    private lateinit var userField: EditText
    private lateinit var syncStatus: TextView
    private lateinit var keepPhone: Button
    private lateinit var keepServer: Button
    private var writingPort = false
    private var writingSync = false

    private val refresh = object : Runnable {
        override fun run() {
            render()
            handler.postDelayed(this, 2000)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(32, 32, 32, 32)
        }
        portField = EditText(this)
        actionButton = Button(this)
        errorView = TextView(this)
        addressList = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        val logs = Button(this).apply {
            text = "查看日志"
            setOnClickListener { startActivity(Intent(this@MainActivity, LogActivity::class.java)) }
        }
        root.addView(TextView(this).apply { text = "本机端口" })
        root.addView(portField)
        root.addView(actionButton)
        root.addView(errorView)
        root.addView(addressList)
        root.addView(logs)
        hostField = EditText(this)
        remotePortField = EditText(this)
        tokenField = EditText(this)
        userField = EditText(this)
        syncStatus = TextView(this)
        val borrow = Button(this).apply {
            text = "借出"
            setOnClickListener { submitSync("/local/launcher/borrow", "") }
        }
        val giveBack = Button(this).apply {
            text = "交还"
            setOnClickListener { submitSync("/local/launcher/return", "") }
        }
        keepPhone = Button(this).apply {
            text = "保留手机进度"
            visibility = View.GONE
            setOnClickListener { submitSync("/local/launcher/return", "phone") }
        }
        keepServer = Button(this).apply {
            text = "保留电脑进度"
            visibility = View.GONE
            setOnClickListener { submitSync("/local/launcher/return", "server") }
        }
        root.addView(TextView(this).apply { text = "电脑地址" })
        root.addView(hostField)
        root.addView(TextView(this).apply { text = "电脑端口" })
        root.addView(remotePortField)
        root.addView(TextView(this).apply { text = "同步口令" })
        root.addView(tokenField)
        root.addView(TextView(this).apply { text = "账号" })
        root.addView(userField)
        root.addView(borrow)
        root.addView(giveBack)
        root.addView(keepPhone)
        root.addView(keepServer)
        root.addView(syncStatus)
        setContentView(ScrollView(this).apply { addView(root) })

        portField.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) = Unit
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) = Unit
            override fun afterTextChanged(s: Editable?) {
                if (writingPort || running()) return
                val text = s?.toString().orEmpty()
                if (PortText.parse(text) == null) {
                    errorView.text = "本机端口无效"
                    return
                }
                writeText("server-port", text)
                if (errorView.text == "本机端口无效") errorView.text = ""
            }
        })
        actionButton.setOnClickListener {
            if (running()) {
                writeText("server-running", "0")
                stopService(Intent(this, ServerService::class.java))
                render()
                return@setOnClickListener
            }
            val port = PortText.parse(portField.text.toString())
            if (port == null) {
                errorView.text = "本机端口无效"
                return@setOnClickListener
            }
            writeText("server-port", port.toString())
            writeText("server-running", "1")
            File(filesDir, "server.log").writeText("")
            File(filesDir, "server-error").delete()
            startForegroundService(Intent(this, ServerService::class.java))
            render()
        }
        writingPort = true
        portField.setText(readText("server-port").ifEmpty { "26020" })
        writingPort = false
        writingSync = true
        hostField.setText(readText("server-host"))
        remotePortField.setText(readText("remote-port").ifEmpty { "26020" })
        tokenField.setText(readText("sync-token"))
        userField.setText(readText("sync-user").ifEmpty { "1000001" })
        writingSync = false
        saveSyncFields(hostField)
        saveSyncFields(remotePortField)
        saveSyncFields(tokenField)
        saveSyncFields(userField)
    }

    private fun saveSyncFields(field: EditText) {
        field.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) = Unit
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) = Unit
            override fun afterTextChanged(s: Editable?) {
                if (writingSync) return
                writeText("server-host", hostField.text.toString().trim())
                writeText("remote-port", remotePortField.text.toString().trim())
                writeText("sync-token", tokenField.text.toString().trim())
                writeText("sync-user", userField.text.toString().trim())
            }
        })
    }

    private fun submitSync(path: String, choice: String) {
        val host = SyncTarget.host(hostField.text.toString())
        val remotePort = SyncTarget.remotePort(remotePortField.text.toString().trim())
        val userID = SyncTarget.userID(userField.text.toString().trim())
        val token = tokenField.text.toString().trim()
        if (host == null || remotePort == null || userID == null || token.isEmpty()) {
            syncStatus.text = "电脑地址、端口、口令或账号无效"
            return
        }
        if (!running()) {
            syncStatus.text = "请先启动服务"
            return
        }
        val localPort = PortText.parse(portField.text.toString()) ?: return
        keepPhone.visibility = View.GONE
        keepServer.visibility = View.GONE
        syncStatus.text = ""
        val body = JSONObject()
            .put("host", host)
            .put("port", remotePort)
            .put("token", token)
            .put("user_id", userID)
            .put("choice", choice)
            .toString()
        Thread {
            val result = runCatching { SyncClient.post(localPort, path, body) }.getOrNull()
            handler.post { showSync(result) }
        }.start()
    }

    private fun showSync(result: SyncClient.Response?) {
        if (result == null) {
            syncStatus.text = "没有连上本机服务"
            return
        }
        if (result.code == 409 && result.error == "choice required") {
            syncStatus.text = "电脑进度版本 ${result.serverRevision}，手机进度版本 ${result.phoneRevision}"
            keepPhone.visibility = View.VISIBLE
            keepServer.visibility = View.VISIBLE
            return
        }
        syncStatus.text = if (result.code == 200) "完成" else result.error
    }

    override fun onStart() {
        super.onStart()
        handler.post(refresh)
    }

    override fun onStop() {
        handler.removeCallbacks(refresh)
        super.onStop()
    }

    private fun render() {
        val active = running()
        portField.isEnabled = !active
        actionButton.text = if (active) "停止" else "启动"
        errorView.text = readText("server-error")
        addressList.removeAllViews()
        if (!active) return
        val port = PortText.parse(portField.text.toString()) ?: return
        Thread {
            val body = runCatching { URL("http://127.0.0.1:$port/local/launcher/addresses").readText() }.getOrNull()
            handler.post {
                if (!running()) return@post
                showAddresses(body, port)
            }
        }.start()
    }

    private fun showAddresses(body: String?, port: Int) {
        addressList.removeAllViews()
        if (body == null) return
        val rows = runCatching { JSONArray(body) }.getOrNull() ?: return
        for (index in 0 until rows.length()) {
            val row = rows.getJSONObject(index)
            val line = AddressLine.of(row.getString("label"), row.getString("host"), port)
            addressList.addView(Button(this).apply {
                text = line.title + "\n" + line.copyText
                layoutParams = LinearLayout.LayoutParams(
                    ViewGroup.LayoutParams.MATCH_PARENT,
                    ViewGroup.LayoutParams.WRAP_CONTENT,
                )
                setOnClickListener {
                    val clipboard = getSystemService(ClipboardManager::class.java)
                    clipboard.setPrimaryClip(ClipData.newPlainText("server", line.copyText))
                }
            })
        }
    }

    private fun running(): Boolean = readText("server-running") == "1"

    private fun readText(name: String): String {
        val file = File(filesDir, name)
        if (!file.exists()) return ""
        return file.readText(StandardCharsets.UTF_8).trim()
    }

    private fun writeText(name: String, value: String) {
        File(filesDir, name).writeText(value, StandardCharsets.UTF_8)
    }
}

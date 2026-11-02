package local.kairisei.mserver

import android.app.Activity
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color
import android.os.Build
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
import java.io.File
import java.net.NetworkInterface
import java.net.URL
import java.nio.charset.StandardCharsets

class MainActivity : Activity() {
    private companion object {
        private const val NOTIFICATION_REQUEST = 1
    }

    private val handler = Handler(Looper.getMainLooper())
    private lateinit var statusView: TextView
    private lateinit var portField: EditText
    private lateinit var actionButton: Button
    private lateinit var errorView: TextView
    private lateinit var addressLabel: TextView
    private lateinit var addressList: LinearLayout
    private lateinit var transferButton: Button
    private lateinit var logStrip: TextView
    private var writingPort = false
    private var addressesReady = false

    private val refresh = object : Runnable {
        override fun run() {
            render()
            handler.postDelayed(this, 2000)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val content = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        statusView = TextView(this).apply { textSize = 18f }
        portField = EditText(this)
        actionButton = Button(this)
        errorView = TextView(this)
        addressLabel = TextView(this).apply { text = "服务器地址" }
        addressList = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        transferButton = Button(this).apply {
            text = "移交"
            setOnClickListener { startActivity(Intent(this@MainActivity, TransferActivity::class.java)) }
        }
        content.addView(statusView)
        content.addView(TextView(this).apply { text = "本机端口" })
        content.addView(portField)
        content.addView(actionButton)
        content.addView(errorView)
        content.addView(addressLabel)
        content.addView(addressList)
        content.addView(transferButton)
        val scroll = ScrollView(this).apply {
            addView(content)
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                0,
                1f,
            )
        }
        logStrip = createLogStrip()
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(32, 32, 32, 0)
            addView(scroll)
            addView(logStrip)
        }
        setContentView(root)

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
                addressesReady = false
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
            addressesReady = false
            File(filesDir, "server.log").writeText("")
            File(filesDir, "server-error").delete()
            startServer()
            render()
        }
        writingPort = true
        portField.setText(readText("server-port").ifEmpty { "26020" })
        writingPort = false
    }

    override fun onStart() {
        super.onStart()
        val active = running()
        if (active) startServer()
        handler.post(refresh)
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode == NOTIFICATION_REQUEST && running()) startForegroundService(Intent(this, ServerService::class.java))
    }

    private fun startServer() {
        if (Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            requestPermissions(arrayOf(android.Manifest.permission.POST_NOTIFICATIONS), NOTIFICATION_REQUEST)
        }
        startForegroundService(Intent(this, ServerService::class.java))
    }

    override fun onStop() {
        handler.removeCallbacks(refresh)
        super.onStop()
    }

    private fun render() {
        val active = running()
        if (!active) addressesReady = false
        val state = HomeState.of(active, addressesReady)
        statusView.text = state.status
        statusView.setTextColor(statusColor(state.status))
        actionButton.text = state.button
        actionButton.isEnabled = state.buttonEnabled
        portField.isEnabled = state.status == "未启动"
        errorView.text = readText("server-error")
        val show = if (state.showTransfer) View.VISIBLE else View.GONE
        addressLabel.visibility = show
        transferButton.visibility = show
        if (!state.showTransfer) addressList.removeAllViews()
        logStrip.showLatestLog(File(filesDir, "server.log"))
        if (!active) return
        val port = PortText.parse(portField.text.toString()) ?: return
        Thread {
            val body = runCatching { URL("http://127.0.0.1:$port/local/launcher/addresses").readText() }.getOrNull()
            handler.post {
                if (!running()) return@post
                if (body != null && runCatching { JSONArray(body) }.isSuccess) {
                    addressesReady = true
                    val local = runCatching { localAddresses() }.getOrDefault(emptyList())
                    if (local.isNotEmpty()) showAddressRows(local, port) else showAddresses(body, port)
                    val started = HomeState.of(true, true)
                    statusView.text = started.status
                    statusView.setTextColor(statusColor(started.status))
                    actionButton.text = started.button
                    actionButton.isEnabled = started.buttonEnabled
                    addressLabel.visibility = View.VISIBLE
                    transferButton.visibility = View.VISIBLE
                }
            }
        }.start()
    }

    private fun localAddresses(): List<Pair<String, String>> {
        val nics = NetworkInterface.getNetworkInterfaces()?.toList().orEmpty()
        val raw = nics.flatMap { nic ->
            val up = runCatching { nic.isUp }.getOrDefault(false)
            nic.inetAddresses.toList().map { addr -> RawNic(nic.name, up, addr.hostAddress ?: "") }
        }
        return usableAddresses(raw)
    }

    private fun showAddressRows(rows: List<Pair<String, String>>, port: Int) {
        addressList.removeAllViews()
        for ((label, host) in rows) addAddressButton(AddressLine.of(label, host, port))
    }

    private fun showAddresses(body: String, port: Int) {
        addressList.removeAllViews()
        val rows = runCatching { JSONArray(body) }.getOrNull() ?: return
        for (index in 0 until rows.length()) {
            val row = rows.getJSONObject(index)
            addAddressButton(AddressLine.of(row.getString("label"), row.getString("host"), port))
        }
    }

    private fun addAddressButton(line: AddressLine) {
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

    private fun statusColor(status: String): Int {
        return when (status) {
            "监测中" -> Color.parseColor("#FFD60A")
            "已启动" -> Color.parseColor("#30D158")
            else -> Color.parseColor("#FFB4A8")
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

package local.kairisei.mserver

import android.app.Activity
import android.os.Bundle
import android.widget.ScrollView
import android.widget.TextView
import java.io.File
import java.nio.charset.StandardCharsets

class LogActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        title = "查看日志"
        val file = File(filesDir, "server.log")
        val text = if (file.exists()) file.readText(StandardCharsets.UTF_8) else ""
        val body = TextView(this).apply {
            this.text = text
            setTextIsSelectable(true)
            setPadding(32, 32, 32, 32)
        }
        setContentView(ScrollView(this).apply { addView(body) })
    }
}

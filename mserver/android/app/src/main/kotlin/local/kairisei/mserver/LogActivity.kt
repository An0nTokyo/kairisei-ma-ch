package local.kairisei.mserver

import android.app.Activity
import android.os.Bundle
import android.widget.LinearLayout
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
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(32, 32, 32, 32)
        }
        root.addView(TextView(this).apply { this.text = "查看日志" })
        root.addView(TextView(this).apply { this.text = text })
        setContentView(ScrollView(this).apply { addView(root) })
    }
}

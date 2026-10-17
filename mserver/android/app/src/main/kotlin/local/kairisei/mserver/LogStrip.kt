package local.kairisei.mserver

import android.app.Activity
import android.content.Intent
import android.graphics.Color
import android.graphics.Typeface
import android.widget.TextView
import java.io.File

fun Activity.createLogStrip(): TextView {
    return TextView(this).apply {
        setBackgroundColor(Color.parseColor("#111111"))
        setTextColor(Color.parseColor("#C8C8C8"))
        typeface = Typeface.MONOSPACE
        setPadding(24, 16, 24, 16)
        setLines(3)
        setOnClickListener { startActivity(Intent(this@createLogStrip, LogActivity::class.java)) }
    }
}

fun TextView.showLatestLog(file: File) {
    val text = if (file.exists()) file.readText(Charsets.UTF_8) else ""
    this.text = LogTail.latest(text)
}

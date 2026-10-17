package local.kairisei.mserver

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import java.io.File
import java.nio.charset.StandardCharsets

class ServerService : Service() {
    private var process: Process? = null

    @Volatile
    private var stopRequested = false

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (readText("server-running") != "1") {
            stopSelf()
            return START_NOT_STICKY
        }
        startInForeground()
        if (process?.isAlive != true) {
            startProcess()
        }
        return START_STICKY
    }

    override fun onDestroy() {
        stopRequested = true
        process?.destroy()
        process = null
        super.onDestroy()
    }

    private fun startInForeground() {
        val manager = getSystemService(NotificationManager::class.java)
        val channel = NotificationChannel(CHANNEL, "服务", NotificationManager.IMPORTANCE_LOW)
        manager.createNotificationChannel(channel)
        val pending = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE,
        )
        val notification = Notification.Builder(this, CHANNEL)
            .setContentTitle("服务运行中")
            .setSmallIcon(android.R.drawable.stat_notify_sync)
            .setContentIntent(pending)
            .setOngoing(true)
            .build()
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(NOTIFICATION, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else {
            startForeground(NOTIFICATION, notification)
        }
    }

    private fun startProcess() {
        val port = readText("server-port").ifEmpty { "26020" }
        ensureSeed()
        val binary = File(applicationInfo.nativeLibraryDir, "libkairi-mserver.so")
        val log = File(filesDir, "server.log")
        val started = try {
            ProcessBuilder(binary.absolutePath, "-port", port, "-data", filesDir.absolutePath)
                .redirectErrorStream(true)
                .redirectOutput(ProcessBuilder.Redirect.appendTo(log))
                .start()
        } catch (error: Exception) {
            writeText("server-error", error.message ?: "程序无法启动")
            writeText("server-running", "0")
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
            return
        }
        process = started
        Thread {
            started.waitFor()
            if (stopRequested || process !== started) return@Thread
            val last = if (log.exists()) log.readLines().lastOrNull { it.isNotBlank() }.orEmpty() else ""
            if (last.isNotEmpty()) writeText("server-error", last)
            writeText("server-running", "0")
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
        }.start()
    }

    private fun ensureSeed() {
        val destination = File(filesDir, "unused-seed.json")
        if (destination.exists()) return
        assets.open("unused-seed.json").use { input ->
            destination.outputStream().use { output -> input.copyTo(output) }
        }
    }

    private fun readText(name: String): String {
        val file = File(filesDir, name)
        if (!file.exists()) return ""
        return file.readText(StandardCharsets.UTF_8).trim()
    }

    private fun writeText(name: String, value: String) {
        File(filesDir, name).writeText(value, StandardCharsets.UTF_8)
    }

    companion object {
        private const val CHANNEL = "kairi-mserver"
        private const val NOTIFICATION = 1
    }
}

package local.kairisei.mserver

data class HomeState(
    val status: String,
    val button: String,
    val buttonEnabled: Boolean,
    val showTransfer: Boolean,
) {
    companion object {
        fun of(running: Boolean, addressesReady: Boolean): HomeState {
            if (!running) return HomeState("未启动", "启动", true, false)
            if (!addressesReady) return HomeState("监测中", "监测中", false, false)
            return HomeState("已启动", "停止", true, true)
        }
    }
}

data class AccountRow(val userID: Int, val name: String, val state: String) {
    fun label(): String = "$userID · " + if (name.isEmpty()) "未命名" else name

    fun matches(query: String): Boolean {
        val text = query.trim()
        if (text.isEmpty()) return true
        return userID.toString().contains(text, ignoreCase = true) ||
            name.contains(text, ignoreCase = true)
    }

    fun borrowedBackground(): Boolean = state != "server"
}

data class TransferAction(val text: String, val enabled: Boolean, val note: String) {
    companion object {
        fun of(row: AccountRow?): TransferAction {
            if (row == null) return TransferAction("借出/交还", false, "")
            return when (row.state) {
                "server" -> TransferAction("借出", true, "")
                "mine" -> TransferAction("交还", true, "")
                "other" -> TransferAction("借出/交还", false, "该账号在别的设备上")
                else -> TransferAction("借出/交还", false, "")
            }
        }
    }
}

data class AccountListResult(val rows: List<AccountRow>, val selected: Int?)

object AccountList {
    fun applyRefresh(current: List<AccountRow>, selected: Int?, fetched: List<AccountRow>?): AccountListResult {
        if (fetched == null) return AccountListResult(current, selected)
        val still = if (selected != null && fetched.any { it.userID == selected }) selected else null
        return AccountListResult(fetched, still)
    }
}

object AccountRefresh {
    fun pulls(visible: Int): Boolean = visible > 0

    fun button(visible: Int, revealed: Boolean): Boolean = visible == 0 && revealed
}

object LogTail {
    fun latest(text: String): String {
        val lines = text.lineSequence().filter { it.isNotBlank() }.toList()
        if (lines.isEmpty()) return "这次启动还没有输出"
        return lines.takeLast(3).joinToString("\n")
    }
}

data class TransferBody(val host: String, val port: Int, val token: String)

object TransferRequest {
    fun body(host: String, portText: String, token: String): TransferBody? {
        val computer = SyncTarget.host(host) ?: return null
        val port = SyncTarget.remotePort(portText.trim()) ?: return null
        val secret = token.trim()
        if (secret.isEmpty()) return null
        return TransferBody(computer, port, secret)
    }
}

object BannerSyncStatus {
    fun of(code: Int, synced: Int, missing: Int, error: String): String {
        if (code !in 200..299) {
            return if (error.isNotEmpty()) "同步图片失败: $error" else "同步图片失败"
        }
        if (missing > 0) return "已同步 ${synced} 张，缺 ${missing} 张"
        return "已同步 ${synced} 张"
    }
}

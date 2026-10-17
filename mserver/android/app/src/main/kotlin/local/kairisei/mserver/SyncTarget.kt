package local.kairisei.mserver

object SyncTarget {
    fun host(text: String): String? {
        val value = text.trim()
        if (value.isEmpty() || value.any { it.isWhitespace() || it == '/' || it == '?' || it == '#' }) {
            return null
        }
        return value
    }

    fun remotePort(text: String): Int? {
        if (text.isEmpty() || text[0] !in '1'..'9' || text.any { it !in '0'..'9' }) return null
        val value = text.toIntOrNull() ?: return null
        if (value > 65535) return null
        return value
    }

    fun userID(text: String): Int? {
        if (text.isEmpty() || text.any { it !in '0'..'9' } || text.length > 9) return null
        val value = text.toIntOrNull() ?: return null
        if (value < 1000001) return null
        return value
    }
}

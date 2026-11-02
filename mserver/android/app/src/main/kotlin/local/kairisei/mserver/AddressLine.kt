package local.kairisei.mserver

data class AddressLine(val title: String, val copyText: String) {
    companion object {
        fun of(label: String, host: String, port: Int): AddressLine {
            return AddressLine(label, "$host:$port")
        }
    }
}

internal data class RawNic(val name: String, val up: Boolean, val host: String)

internal fun usableAddresses(nics: List<RawNic>): List<Pair<String, String>> {
    val rows = mutableListOf<Triple<Boolean, String, String>>()
    for (nic in nics) {
        if (!nic.up) continue
        val host = ipv4Host(nic.host) ?: continue
        if (host == "0.0.0.0" || host.startsWith("169.254.")) continue
        if (host.startsWith("127.")) {
            if (host != "127.0.0.1") continue
            rows.add(Triple(true, host, "本机 · ${nic.name}"))
            continue
        }
        rows.add(Triple(false, host, nic.name))
    }
    return rows.sortedWith(compareBy<Triple<Boolean, String, String>> { !it.first }.thenBy { it.second })
        .map { it.third to it.second }
}

private fun ipv4Host(text: String): String? {
    val parts = text.split('.')
    if (parts.size != 4) return null
    val nums = parts.map { part ->
        if (part.isEmpty() || part.any { it !in '0'..'9' }) return null
        val value = part.toIntOrNull() ?: return null
        if (value > 255) return null
        value
    }
    return nums.joinToString(".")
}

package local.kairisei.mserver

object PortText {
    fun parse(text: String): Int? {
        if (text.isEmpty() || text[0] !in '1'..'9') {
            return null
        }
        if (text.any { it !in '0'..'9' }) {
            return null
        }
        val value = text.toIntOrNull() ?: return null
        if (value > 65534) {
            return null
        }
        return value
    }
}

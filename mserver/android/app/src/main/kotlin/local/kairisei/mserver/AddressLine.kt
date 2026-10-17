package local.kairisei.mserver

data class AddressLine(val title: String, val copyText: String) {
    companion object {
        fun of(label: String, host: String, port: Int): AddressLine {
            return AddressLine(label, "$host:$port")
        }
    }
}

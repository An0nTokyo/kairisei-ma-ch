package local.kairisei.mserver

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test

class AddressLineTest {
    @Test
    fun buildsCopyTextFromLabelAndHost() {
        val local = AddressLine.of("本机 · lo", "127.0.0.1", 26020)
        assertEquals("本机 · lo", local.title)
        assertEquals("127.0.0.1:26020", local.copyText)

        val wifi = AddressLine.of("wlan0", "192.168.1.23", 26020)
        assertEquals("192.168.1.23:26020", wifi.copyText)

        assertFalse(local.title.contains("复制"))
        assertFalse(local.copyText.contains("复制"))
        assertFalse(wifi.title.contains("复制"))
        assertFalse(wifi.copyText.contains("复制"))
    }
}

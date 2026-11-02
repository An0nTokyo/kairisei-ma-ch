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

    @Test
    fun keepsUsableIpv4FromLocalInterfaces() {
        val rows = usableAddresses(listOf(
            RawNic("lo", true, "127.0.0.1"),
            RawNic("lo", true, "127.0.0.2"),
            RawNic("lo", true, "::1"),
            RawNic("rmnet_data2", true, "10.68.18.118"),
            RawNic("rmnet_data2", true, "fe80::1%rmnet_data2"),
            RawNic("wlan0", true, "0.0.0.0"),
            RawNic("wlan0", true, "169.254.4.5"),
            RawNic("down0", false, "10.0.0.8"),
        ))
        assertEquals(listOf("本机 · lo" to "127.0.0.1", "rmnet_data2" to "10.68.18.118"), rows)
    }
}

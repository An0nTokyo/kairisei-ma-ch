package local.kairisei.mserver

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class PortTextTest {
    @Test
    fun parsesCanonicalPorts() {
        assertEquals(26020, PortText.parse("26020"))
        assertEquals(1, PortText.parse("1"))
        assertEquals(65534, PortText.parse("65534"))
    }

    @Test
    fun rejectsInvalidPorts() {
        assertNull(PortText.parse(""))
        assertNull(PortText.parse("0"))
        assertNull(PortText.parse("01"))
        assertNull(PortText.parse("65535"))
        assertNull(PortText.parse("26020 "))
    }
}

package local.kairisei.mserver

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class SyncTargetTest {
    @Test
    fun acceptsAComputerAddress() {
        assertEquals("10.0.2.2", SyncTarget.host(" 10.0.2.2 "))
        assertEquals(26020, SyncTarget.remotePort("26020"))
        assertEquals(65535, SyncTarget.remotePort("65535"))
        assertEquals(1000001, SyncTarget.userID("1000001"))
    }

    @Test
    fun rejectsAUrlOrASmallUser() {
        assertNull(SyncTarget.host("http://10.0.2.2"))
        assertNull(SyncTarget.remotePort("65536"))
        assertNull(SyncTarget.remotePort("026020"))
        assertNull(SyncTarget.userID("1000000"))
    }
}

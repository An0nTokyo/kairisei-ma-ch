package local.kairisei.mserver

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PageRulesTest {
    @Test
    fun homeHasThreeStates() {
        assertEquals(HomeState("未启动", "启动", true, false), HomeState.of(false, false))
        assertEquals(HomeState("未启动", "启动", true, false), HomeState.of(false, true))
        assertEquals(HomeState("监测中", "监测中", false, false), HomeState.of(true, false))
        assertEquals(HomeState("已启动", "停止", true, true), HomeState.of(true, true))
    }

    @Test
    fun searchMatchesIdOrNameButNotThePlaceholder() {
        val named = AccountRow(1000001, "秋", "server")
        val blank = AccountRow(1000006, "", "server")
        assertEquals("1000001 · 秋", named.label())
        assertEquals("1000006 · 未命名", blank.label())
        assertTrue(named.matches(" 秋 "))
        assertTrue(named.matches("0001"))
        assertFalse(named.matches("另一"))
        assertTrue(blank.matches(""))
        assertFalse(blank.matches("未命名"))
        assertTrue(blank.matches("1000006"))
    }

    @Test
    fun buttonFollowsSelection() {
        assertEquals(TransferAction("借出/交还", false, ""), TransferAction.of(null))
        assertEquals(TransferAction("借出", true, ""), TransferAction.of(AccountRow(1, "秋", "server")))
        assertEquals(TransferAction("交还", true, ""), TransferAction.of(AccountRow(1, "秋", "mine")))
        assertEquals(
            TransferAction("借出/交还", false, "该账号在别的设备上"),
            TransferAction.of(AccountRow(1, "秋", "other")),
        )
        assertEquals(TransferAction("借出/交还", false, ""), TransferAction.of(AccountRow(1, "秋", "weird")))
    }

    @Test
    fun refreshKeepsTheOldListWhenTheFetchFails() {
        val old = listOf(AccountRow(1000001, "秋", "mine"))
        val failed = AccountList.applyRefresh(old, 1000001, null)
        assertEquals(old, failed.rows)
        assertEquals(1000001, failed.selected)
        val gone = AccountList.applyRefresh(old, 1000001, emptyList())
        assertEquals(emptyList<AccountRow>(), gone.rows)
        assertNull(gone.selected)
        val kept = AccountList.applyRefresh(old, 1000001, listOf(AccountRow(1000001, "秋", "server")))
        assertEquals(1000001, kept.selected)
        assertFalse(AccountRefresh.pulls(0))
        assertTrue(AccountRefresh.pulls(2))
        assertFalse(AccountRefresh.button(0, false))
        assertTrue(AccountRefresh.button(0, true))
        assertFalse(AccountRefresh.button(2, true))
    }

    @Test
    fun logTailShowsTheLastThreeNonBlankLines() {
        assertEquals("这次启动还没有输出", LogTail.latest(""))
        assertEquals("这次启动还没有输出", LogTail.latest("\n  \n"))
        assertEquals("one", LogTail.latest("one\n"))
        assertEquals("two\nthree\nfour", LogTail.latest("one\n\ntwo\nthree\nfour\n"))
    }

    @Test
    fun invalidTargetDoesNotBuildARequest() {
        assertNull(TransferRequest.body("http://10.0.0.1", "26020", "secret"))
        assertNull(TransferRequest.body("10.0.0.1", "0", "secret"))
        assertNull(TransferRequest.body("10.0.0.1", "26020", " "))
        val body = TransferRequest.body(" 10.0.0.1 ", "26020", " secret ")
        assertEquals("10.0.0.1", body?.host)
        assertEquals(26020, body?.port)
        assertEquals("secret", body?.token)
    }

    @Test
    fun bannerSyncStatusCopy() {
        assertEquals("同步图片失败: nope", BannerSyncStatus.of(400, 0, 0, "nope"))
        assertEquals("同步图片失败", BannerSyncStatus.of(500, 0, 0, ""))
        assertEquals("已同步 3 张", BannerSyncStatus.of(200, 3, 0, ""))
        assertEquals("已同步 2 张，缺 1 张", BannerSyncStatus.of(200, 2, 1, ""))
    }
}

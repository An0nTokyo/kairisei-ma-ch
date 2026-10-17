package local.kairisei.mserver

import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

object SyncClient {
    data class Response(val code: Int, val error: String, val serverRevision: Int, val phoneRevision: Int)

    fun post(localPort: Int, path: String, body: String): Response {
        val connection = URL("http://127.0.0.1:$localPort$path").openConnection() as HttpURLConnection
        connection.requestMethod = "POST"
        connection.doOutput = true
        connection.setRequestProperty("Content-Type", "application/json; charset=utf-8")
        connection.outputStream.use { it.write(body.toByteArray(Charsets.UTF_8)) }
        val code = connection.responseCode
        val stream = if (code >= 400) connection.errorStream else connection.inputStream
        val text = stream?.bufferedReader()?.use { it.readText() }.orEmpty()
        val json = runCatching { JSONObject(text) }.getOrNull()
        return Response(
            code,
            json?.optString("error").orEmpty(),
            json?.optInt("server_revision") ?: 0,
            json?.optInt("phone_revision") ?: 0,
        )
    }
}

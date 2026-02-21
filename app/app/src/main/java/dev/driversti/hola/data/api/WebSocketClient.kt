package dev.driversti.hola.data.api

import android.util.Log
import dev.driversti.hola.data.model.WsMessage
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import java.util.concurrent.TimeUnit

enum class ConnectionState { DISCONNECTED, CONNECTING, CONNECTED }

class WebSocketClient(
    private val host: String,
    private val port: Int,
    private val token: String,
) {
    private val tag = "WebSocketClient"

    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    private val okHttpClient = OkHttpClient.Builder()
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .pingInterval(30, TimeUnit.SECONDS)
        .build()

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    private var webSocket: WebSocket? = null
    private var reconnectAttempt = 0
    private var shouldReconnect = false

    // Active subscriptions to replay on reconnect.
    private val activeSubscriptions = mutableSetOf<SubscriptionKey>()
    private var metricsIntervalSeconds = 3

    private val _connectionState = MutableStateFlow(ConnectionState.DISCONNECTED)
    val connectionState: StateFlow<ConnectionState> = _connectionState

    private val _messages = MutableSharedFlow<WsMessage>(extraBufferCapacity = 64)
    val messages: SharedFlow<WsMessage> = _messages

    fun connect() {
        if (_connectionState.value != ConnectionState.DISCONNECTED) return
        shouldReconnect = true
        reconnectAttempt = 0
        doConnect()
    }

    fun disconnect() {
        shouldReconnect = false
        // Keep activeSubscriptions intact — they'll be replayed on reconnect.
        webSocket?.close(1000, "client disconnect")
        webSocket = null
        _connectionState.value = ConnectionState.DISCONNECTED
    }

    fun subscribeMetrics(intervalSeconds: Int = 3) {
        val key = SubscriptionKey("metrics")
        if (activeSubscriptions.add(key)) {
            metricsIntervalSeconds = intervalSeconds
            send(WsMessage(
                type = "subscribe",
                payload = buildJsonObject {
                    put("stream", "metrics")
                    put("interval_seconds", intervalSeconds)
                },
            ))
        }
    }

    fun subscribeEvents() {
        val key = SubscriptionKey("events")
        if (activeSubscriptions.add(key)) {
            send(WsMessage(
                type = "subscribe",
                payload = buildJsonObject {
                    put("stream", "events")
                },
            ))
        }
    }

    fun subscribeLogs(containerId: String) {
        val key = SubscriptionKey("logs", containerId)
        if (activeSubscriptions.add(key)) {
            send(WsMessage(
                type = "subscribe",
                payload = buildJsonObject {
                    put("stream", "logs")
                    put("container_id", containerId)
                },
            ))
        }
    }

    fun unsubscribeMetrics() {
        val key = SubscriptionKey("metrics")
        if (activeSubscriptions.remove(key)) {
            send(WsMessage(
                type = "unsubscribe",
                payload = buildJsonObject { put("stream", "metrics") },
            ))
        }
    }

    fun unsubscribeEvents() {
        val key = SubscriptionKey("events")
        if (activeSubscriptions.remove(key)) {
            send(WsMessage(
                type = "unsubscribe",
                payload = buildJsonObject { put("stream", "events") },
            ))
        }
    }

    fun unsubscribeLogs(containerId: String) {
        val key = SubscriptionKey("logs", containerId)
        if (activeSubscriptions.remove(key)) {
            send(WsMessage(
                type = "unsubscribe",
                payload = buildJsonObject {
                    put("stream", "logs")
                    put("container_id", containerId)
                },
            ))
        }
    }

    fun subscribeContainerStats(containerId: String) {
        val key = SubscriptionKey("container_stats", containerId)
        if (activeSubscriptions.add(key)) {
            send(WsMessage(
                type = "subscribe",
                payload = buildJsonObject {
                    put("stream", "container_stats")
                    put("container_id", containerId)
                    put("interval_seconds", 3)
                },
            ))
        }
    }

    fun unsubscribeContainerStats(containerId: String) {
        val key = SubscriptionKey("container_stats", containerId)
        if (activeSubscriptions.remove(key)) {
            send(WsMessage(
                type = "unsubscribe",
                payload = buildJsonObject {
                    put("stream", "container_stats")
                    put("container_id", containerId)
                },
            ))
        }
    }

    fun send(message: WsMessage) {
        val text = json.encodeToString(message)
        webSocket?.send(text) ?: Log.w(tag, "send called but WebSocket not connected")
    }

    private fun doConnect() {
        _connectionState.value = ConnectionState.CONNECTING
        val url = "ws://$host:$port/api/v1/ws"
        val request = Request.Builder()
            .url(url)
            .header("Authorization", "Bearer $token")
            .build()

        webSocket = okHttpClient.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                Log.i(tag, "connected to $url")
                _connectionState.value = ConnectionState.CONNECTED
                reconnectAttempt = 0
                replaySubscriptions()
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                try {
                    val msg = json.decodeFromString<WsMessage>(text)
                    _messages.tryEmit(msg)
                } catch (e: Exception) {
                    Log.w(tag, "failed to parse message: $text", e)
                }
            }

            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                Log.i(tag, "server closing: $code $reason")
                webSocket.close(code, reason)
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                Log.i(tag, "closed: $code $reason")
                _connectionState.value = ConnectionState.DISCONNECTED
                scheduleReconnect()
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                Log.w(tag, "connection failed: ${t.message}")
                _connectionState.value = ConnectionState.DISCONNECTED
                scheduleReconnect()
            }
        })
    }

    private fun replaySubscriptions() {
        for (key in activeSubscriptions.toSet()) {
            val payload = buildJsonObject {
                put("stream", key.stream)
                key.containerId?.let { put("container_id", it) }
                if (key.stream == "metrics") put("interval_seconds", metricsIntervalSeconds)
            }
            send(WsMessage(type = "subscribe", payload = payload))
        }
    }

    private fun scheduleReconnect() {
        if (!shouldReconnect) return
        reconnectAttempt++
        val delayMs = minOf(1000L * (1L shl minOf(reconnectAttempt, 5)), 30_000L)
        Log.i(tag, "reconnecting in ${delayMs}ms (attempt $reconnectAttempt)")
        scope.launch {
            delay(delayMs)
            if (shouldReconnect && _connectionState.value == ConnectionState.DISCONNECTED) {
                doConnect()
            }
        }
    }

    private data class SubscriptionKey(
        val stream: String,
        val containerId: String? = null,
    )
}

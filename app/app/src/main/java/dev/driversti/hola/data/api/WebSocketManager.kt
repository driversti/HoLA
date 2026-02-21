package dev.driversti.hola.data.api

import dev.driversti.hola.data.model.ContainerEvent
import dev.driversti.hola.data.model.ServerConfig
import dev.driversti.hola.data.model.SystemMetrics
import dev.driversti.hola.data.model.WsLogLine
import dev.driversti.hola.data.model.WsMessage
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.launch
import kotlinx.serialization.json.Json

/**
 * Manages WebSocket connections across multiple servers.
 * Provides typed flows for metrics, events, and logs.
 */
class WebSocketManager(
    private val tokenRepository: TokenRepository,
) {
    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val connections = mutableMapOf<String, WebSocketClient>()

    private val _metricsFlow = MutableSharedFlow<Pair<String, SystemMetrics>>(extraBufferCapacity = 16)
    val metricsFlow: SharedFlow<Pair<String, SystemMetrics>> = _metricsFlow

    private val _eventsFlow = MutableSharedFlow<Pair<String, ContainerEvent>>(extraBufferCapacity = 64)
    val eventsFlow: SharedFlow<Pair<String, ContainerEvent>> = _eventsFlow

    private val _logsFlow = MutableSharedFlow<WsLogLine>(extraBufferCapacity = 256)
    val logsFlow: SharedFlow<WsLogLine> = _logsFlow

    fun getOrCreateClient(server: ServerConfig): WebSocketClient? {
        val token = tokenRepository.getToken() ?: return null
        return connections.getOrPut(server.id) {
            val client = WebSocketClient(server.host, server.port, token)
            scope.launch { collectMessages(server.id, client) }
            client
        }
    }

    fun onAppForeground(servers: List<ServerConfig>) {
        for (server in servers) {
            getOrCreateClient(server)?.connect()
        }
    }

    fun onAppBackground() {
        for ((_, client) in connections) {
            client.disconnect()
        }
    }

    fun removeConnection(serverId: String) {
        connections.remove(serverId)?.disconnect()
    }

    private suspend fun collectMessages(serverId: String, client: WebSocketClient) {
        client.messages.collect { msg ->
            dispatchMessage(serverId, msg)
        }
    }

    private fun dispatchMessage(serverId: String, msg: WsMessage) {
        val payload = msg.payload ?: return

        try {
            when (msg.type) {
                "metrics" -> {
                    val metrics = json.decodeFromString<SystemMetrics>(payload.toString())
                    _metricsFlow.tryEmit(serverId to metrics)
                }

                "container_event" -> {
                    val event = json.decodeFromString<ContainerEvent>(payload.toString())
                    _eventsFlow.tryEmit(serverId to event)
                }

                "log_line" -> {
                    val logLine = json.decodeFromString<WsLogLine>(payload.toString())
                    _logsFlow.tryEmit(logLine)
                }
            }
        } catch (_: Exception) {
            // Silently ignore malformed payloads.
        }
    }
}

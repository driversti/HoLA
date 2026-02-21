package dev.driversti.hola.ui.screens.serverlist

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.api.WebSocketManager
import dev.driversti.hola.data.model.ServerConfig
import dev.driversti.hola.data.model.SystemMetrics
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

data class ServerStatus(
    val server: ServerConfig,
    val online: Boolean,
    val metrics: SystemMetrics? = null,
    val stackCount: Int = 0,
    val agentVersion: String? = null,
)

data class ServerListState(
    val servers: List<ServerStatus> = emptyList(),
    val isLoading: Boolean = true,
)

class ServerListViewModel(
    private val serverRepository: ServerRepository,
    private val tokenRepository: TokenRepository,
    private val webSocketManager: WebSocketManager,
) : ViewModel() {

    private val _state = MutableStateFlow(ServerListState())
    val state: StateFlow<ServerListState> = _state

    init {
        observeServers()
        observeMetrics()
    }

    fun refresh() {
        fetchStatuses()
    }

    private fun observeServers() {
        viewModelScope.launch {
            serverRepository.servers.collectLatest { servers ->
                fetchStatuses(servers)
            }
        }
    }

    private fun observeMetrics() {
        viewModelScope.launch {
            webSocketManager.metricsFlow.collect { (serverId, metrics) ->
                val current = _state.value
                val updated = current.servers.map { status ->
                    if (status.server.id == serverId) {
                        status.copy(metrics = metrics, online = true)
                    } else {
                        status
                    }
                }
                _state.value = current.copy(servers = updated)
            }
        }
    }

    private fun fetchStatuses(servers: List<ServerConfig>? = null) {
        viewModelScope.launch {
            _state.value = _state.value.copy(isLoading = true)

            val serverList = servers ?: serverRepository.servers.first()
            val token = tokenRepository.getToken() ?: ""

            val statuses = serverList.map { server ->
                async { fetchServerStatus(server, token) }
            }.awaitAll()

            _state.value = ServerListState(servers = statuses, isLoading = false)

            // Subscribe to live metrics for each online server.
            for (status in statuses) {
                if (status.online) {
                    webSocketManager.getOrCreateClient(status.server)?.let { client ->
                        client.connect()
                        client.subscribeMetrics(intervalSeconds = 1)
                    }
                }
            }
        }
    }

    private suspend fun fetchServerStatus(server: ServerConfig, token: String): ServerStatus {
        return try {
            val api = ApiProvider.forServer(server, token)
            val metrics = api.systemMetrics()
            val stacks = api.listStacks()
            val agentInfo = api.agentInfo()
            ServerStatus(
                server = server,
                online = true,
                metrics = metrics,
                stackCount = stacks.stacks.size,
                agentVersion = agentInfo.version,
            )
        } catch (_: Exception) {
            ServerStatus(server = server, online = false)
        }
    }
}

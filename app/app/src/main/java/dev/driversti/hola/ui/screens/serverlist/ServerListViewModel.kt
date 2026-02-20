package dev.driversti.hola.ui.screens.serverlist

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.model.ServerConfig
import dev.driversti.hola.data.model.SystemMetrics
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

data class ServerStatus(
    val server: ServerConfig,
    val online: Boolean,
    val metrics: SystemMetrics? = null,
    val stackCount: Int = 0,
)

data class ServerListState(
    val servers: List<ServerStatus> = emptyList(),
    val isLoading: Boolean = true,
)

class ServerListViewModel(
    private val serverRepository: ServerRepository,
    private val tokenRepository: TokenRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(ServerListState())
    val state: StateFlow<ServerListState> = _state

    init {
        loadServers()
    }

    fun refresh() {
        loadServers()
    }

    private fun loadServers() {
        viewModelScope.launch {
            _state.value = _state.value.copy(isLoading = true)

            val servers = serverRepository.servers.first()
            val token = tokenRepository.getToken() ?: ""

            val statuses = servers.map { server ->
                async { fetchServerStatus(server, token) }
            }.awaitAll()

            _state.value = ServerListState(servers = statuses, isLoading = false)
        }
    }

    private suspend fun fetchServerStatus(server: ServerConfig, token: String): ServerStatus {
        return try {
            val api = ApiProvider.forServer(server, token)
            val metrics = api.systemMetrics()
            val stacks = api.listStacks()
            ServerStatus(
                server = server,
                online = true,
                metrics = metrics,
                stackCount = stacks.stacks.size,
            )
        } catch (_: Exception) {
            ServerStatus(server = server, online = false)
        }
    }
}

package dev.driversti.hola.ui.screens.serverdetail

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.api.HolaApi
import dev.driversti.hola.data.model.Stack
import dev.driversti.hola.data.model.SystemMetrics
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

data class ServerDetailState(
    val serverName: String = "",
    val metrics: SystemMetrics? = null,
    val stacks: List<Stack> = emptyList(),
    val isLoading: Boolean = true,
    val error: String? = null,
)

class ServerDetailViewModel(
    private val serverId: String,
    private val serverRepository: ServerRepository,
    private val tokenRepository: TokenRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(ServerDetailState())
    val state: StateFlow<ServerDetailState> = _state

    private var api: HolaApi? = null

    init {
        load()
    }

    fun refresh() = load()

    private fun load() {
        viewModelScope.launch {
            _state.value = _state.value.copy(isLoading = true, error = null)
            try {
                val server = serverRepository.servers.first().find { it.id == serverId }
                    ?: throw IllegalStateException("Server not found")
                val token = tokenRepository.getToken() ?: ""
                val client = ApiProvider.forServer(server, token)
                api = client

                val metrics = client.systemMetrics()
                val stacks = client.listStacks()

                _state.value = ServerDetailState(
                    serverName = server.name,
                    metrics = metrics,
                    stacks = stacks.stacks,
                    isLoading = false,
                )
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    isLoading = false,
                    error = e.message ?: "Failed to load server details",
                )
            }
        }
    }
}

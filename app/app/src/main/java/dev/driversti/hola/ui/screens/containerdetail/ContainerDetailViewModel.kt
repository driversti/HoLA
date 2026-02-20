package dev.driversti.hola.ui.screens.containerdetail

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.api.HolaApi
import dev.driversti.hola.data.model.ContainerInfo
import dev.driversti.hola.data.model.LogEntry
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

data class ContainerDetailState(
    val container: ContainerInfo? = null,
    val logs: List<LogEntry> = emptyList(),
    val isLoading: Boolean = true,
    val isLoadingLogs: Boolean = false,
    val actionInProgress: String? = null,
    val message: String? = null,
    val error: String? = null,
)

class ContainerDetailViewModel(
    private val serverId: String,
    private val containerId: String,
    private val serverRepository: ServerRepository,
    private val tokenRepository: TokenRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(ContainerDetailState())
    val state: StateFlow<ContainerDetailState> = _state

    private var api: HolaApi? = null
    private var stackName: String? = null

    init {
        load()
    }

    fun refreshLogs() = loadLogs()

    private fun load() {
        viewModelScope.launch {
            _state.value = _state.value.copy(isLoading = true, error = null)
            try {
                val server = serverRepository.servers.first().find { it.id == serverId }
                    ?: throw IllegalStateException("Server not found")
                val token = tokenRepository.getToken() ?: ""
                val client = ApiProvider.forServer(server, token)
                api = client

                // Find container across all stacks
                val stacks = client.listStacks()
                var foundContainer: ContainerInfo? = null
                for (stack in stacks.stacks) {
                    val detail = client.getStack(stack.name)
                    val ctr = detail.containers.find { it.id == containerId }
                    if (ctr != null) {
                        foundContainer = ctr
                        stackName = stack.name
                        break
                    }
                }

                _state.value = _state.value.copy(
                    container = foundContainer,
                    isLoading = false,
                )

                loadLogs()
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    isLoading = false,
                    error = e.message ?: "Failed to load container",
                )
            }
        }
    }

    private fun loadLogs() {
        val client = api ?: return
        viewModelScope.launch {
            _state.value = _state.value.copy(isLoadingLogs = true)
            try {
                val logsResponse = client.containerLogs(containerId, lines = 100)
                _state.value = _state.value.copy(
                    logs = logsResponse.lines,
                    isLoadingLogs = false,
                )
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    isLoadingLogs = false,
                    error = "Failed to load logs: ${e.message}",
                )
            }
        }
    }

    fun executeAction(action: String) {
        val client = api ?: return
        viewModelScope.launch {
            _state.value = _state.value.copy(actionInProgress = action, message = null, error = null)
            try {
                val result = when (action) {
                    "stop" -> client.stopContainer(containerId)
                    "restart" -> client.restartContainer(containerId)
                    else -> throw IllegalArgumentException("Unknown action: $action")
                }
                _state.value = _state.value.copy(
                    actionInProgress = null,
                    message = result.message ?: result.error,
                )
                if (result.success) load()
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    actionInProgress = null,
                    error = e.message ?: "Action failed",
                )
            }
        }
    }

    fun clearMessage() {
        _state.value = _state.value.copy(message = null, error = null)
    }
}

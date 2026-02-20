package dev.driversti.hola.ui.screens.stackdetail

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.api.HolaApi
import dev.driversti.hola.data.model.ContainerInfo
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

data class StackDetailState(
    val stackName: String = "",
    val status: String = "",
    val containers: List<ContainerInfo> = emptyList(),
    val isLoading: Boolean = true,
    val actionInProgress: String? = null,
    val message: String? = null,
    val error: String? = null,
)

class StackDetailViewModel(
    private val serverId: String,
    private val stackName: String,
    private val serverRepository: ServerRepository,
    private val tokenRepository: TokenRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(StackDetailState(stackName = stackName))
    val state: StateFlow<StackDetailState> = _state

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

                val detail = client.getStack(stackName)
                _state.value = _state.value.copy(
                    status = detail.status,
                    containers = detail.containers,
                    isLoading = false,
                )
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    isLoading = false,
                    error = e.message ?: "Failed to load stack",
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
                    "start" -> client.startStack(stackName)
                    "stop" -> client.stopStack(stackName)
                    "restart" -> client.restartStack(stackName)
                    "down" -> client.downStack(stackName)
                    "pull" -> client.pullStack(stackName)
                    else -> throw IllegalArgumentException("Unknown action: $action")
                }
                _state.value = _state.value.copy(
                    actionInProgress = null,
                    message = result.message ?: result.error,
                )
                if (result.success) refresh()
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

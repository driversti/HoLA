package dev.driversti.hola.ui.screens.containerdetail

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.api.ConnectionState
import dev.driversti.hola.data.api.HolaApi
import dev.driversti.hola.data.api.WebSocketClient
import dev.driversti.hola.data.api.WebSocketManager
import dev.driversti.hola.data.model.ContainerInfo
import dev.driversti.hola.data.model.LogEntry
import dev.driversti.hola.data.model.ServerConfig
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
    val isStreaming: Boolean = false,
    val isFollowing: Boolean = true,
    val connectionState: ConnectionState = ConnectionState.DISCONNECTED,
    val cpuPercent: Float? = null,
    val memUsedBytes: Long? = null,
    val memLimitBytes: Long? = null,
    val memPercent: Float? = null,
)

class ContainerDetailViewModel(
    private val serverId: String,
    private val containerId: String,
    private val serverRepository: ServerRepository,
    private val tokenRepository: TokenRepository,
    private val webSocketManager: WebSocketManager,
) : ViewModel() {

    private val _state = MutableStateFlow(ContainerDetailState())
    val state: StateFlow<ContainerDetailState> = _state

    private var api: HolaApi? = null
    private var stackName: String? = null
    private var wsClient: WebSocketClient? = null
    private var lastRestTimestamp: String? = null

    companion object {
        private const val MAX_LOG_ENTRIES = 1000
    }

    init {
        load()
        observeLogs()
        observeContainerStats()
    }

    fun refreshLogs() = loadLogs()

    fun setFollowing(following: Boolean) {
        _state.value = _state.value.copy(isFollowing = following)
    }

    private fun observeLogs() {
        viewModelScope.launch {
            webSocketManager.logsFlow.collect { logLine ->
                if (logLine.containerId == containerId) {
                    val cutoff = lastRestTimestamp
                    if (cutoff != null && logLine.timestamp <= cutoff) return@collect

                    val entry = LogEntry(
                        timestamp = logLine.timestamp,
                        stream = logLine.stream,
                        message = logLine.message,
                    )
                    val current = _state.value
                    val updated = (current.logs + entry).let {
                        if (it.size > MAX_LOG_ENTRIES) it.drop(it.size - MAX_LOG_ENTRIES) else it
                    }
                    _state.value = current.copy(logs = updated)
                }
            }
        }
    }

    private fun observeContainerStats() {
        viewModelScope.launch {
            webSocketManager.containerStatsFlow.collect { stats ->
                if (stats.containerId == containerId) {
                    _state.value = _state.value.copy(
                        cpuPercent = stats.cpuPercent,
                        memUsedBytes = stats.memUsedBytes,
                        memLimitBytes = stats.memLimitBytes,
                        memPercent = stats.memPercent,
                    )
                }
            }
        }
    }

    private fun observeConnectionState() {
        val client = wsClient ?: return
        viewModelScope.launch {
            client.connectionState.collect { connState ->
                _state.value = _state.value.copy(
                    connectionState = connState,
                    isStreaming = connState == ConnectionState.CONNECTED,
                )
            }
        }
    }

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
                subscribeToLogStream(server)
                observeConnectionState()
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    isLoading = false,
                    error = e.message ?: "Failed to load container",
                )
            }
        }
    }

    private fun subscribeToLogStream(server: ServerConfig) {
        val client = webSocketManager.getOrCreateClient(server) ?: return
        wsClient = client
        client.connect()
        client.subscribeLogs(containerId)
        client.subscribeContainerStats(containerId)
    }

    private fun loadLogs() {
        val client = api ?: return
        viewModelScope.launch {
            _state.value = _state.value.copy(isLoadingLogs = true)
            try {
                val logsResponse = client.containerLogs(containerId, lines = 100)
                lastRestTimestamp = logsResponse.lines.lastOrNull()?.timestamp
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

    override fun onCleared() {
        super.onCleared()
        wsClient?.unsubscribeLogs(containerId)
        wsClient?.unsubscribeContainerStats(containerId)
    }
}

package dev.driversti.hola.ui.screens.addserver

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.model.AgentInfo
import dev.driversti.hola.data.model.ServerConfig
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import java.util.UUID

data class AddServerState(
    val name: String = "",
    val host: String = "",
    val port: String = "8420",
    val isEditing: Boolean = false,
    val editId: String? = null,
    val testResult: TestResult? = null,
    val isTesting: Boolean = false,
    val isSaving: Boolean = false,
    val saved: Boolean = false,
)

sealed class TestResult {
    data class Success(val info: AgentInfo) : TestResult()
    data class Failure(val message: String) : TestResult()
}

class AddServerViewModel(
    private val serverId: String?,
    private val serverRepository: ServerRepository,
    private val tokenRepository: TokenRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(AddServerState())
    val state: StateFlow<AddServerState> = _state

    init {
        if (serverId != null) {
            loadExisting(serverId)
        }
    }

    private fun loadExisting(id: String) {
        viewModelScope.launch {
            val servers = serverRepository.servers.first()
            val server = servers.find { it.id == id } ?: return@launch
            _state.value = _state.value.copy(
                name = server.name,
                host = server.host,
                port = server.port.toString(),
                isEditing = true,
                editId = id,
            )
        }
    }

    fun updateName(value: String) {
        _state.value = _state.value.copy(name = value, testResult = null)
    }

    fun updateHost(value: String) {
        _state.value = _state.value.copy(host = value, testResult = null)
    }

    fun updatePort(value: String) {
        _state.value = _state.value.copy(port = value, testResult = null)
    }

    fun testConnection() {
        val s = _state.value
        val port = s.port.toIntOrNull() ?: 8420
        val token = tokenRepository.getToken() ?: ""

        viewModelScope.launch {
            _state.value = s.copy(isTesting = true, testResult = null)
            try {
                val config = ServerConfig(id = "", name = s.name, host = s.host, port = port)
                val api = ApiProvider.forServer(config, token)
                api.health()
                val info = api.agentInfo()
                _state.value = _state.value.copy(isTesting = false, testResult = TestResult.Success(info))
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    isTesting = false,
                    testResult = TestResult.Failure(e.message ?: "Connection failed"),
                )
            }
        }
    }

    fun save() {
        val s = _state.value
        val port = s.port.toIntOrNull() ?: 8420

        viewModelScope.launch {
            _state.value = s.copy(isSaving = true)
            val server = ServerConfig(
                id = s.editId ?: UUID.randomUUID().toString(),
                name = s.name.trim(),
                host = s.host.trim(),
                port = port,
            )
            if (s.isEditing) {
                serverRepository.updateServer(server)
            } else {
                serverRepository.addServer(server)
            }
            _state.value = _state.value.copy(isSaving = false, saved = true)
        }
    }

    fun isValid(): Boolean {
        val s = _state.value
        return s.name.isNotBlank() && s.host.isNotBlank() && s.port.toIntOrNull() != null
    }
}

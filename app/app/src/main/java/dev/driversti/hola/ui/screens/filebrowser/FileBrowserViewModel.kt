package dev.driversti.hola.ui.screens.filebrowser

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.api.HolaApi
import dev.driversti.hola.data.model.FileWriteRequest
import dev.driversti.hola.data.model.FsEntry
import dev.driversti.hola.data.model.MkdirRequest
import dev.driversti.hola.data.model.RegisterStackRequest
import dev.driversti.hola.data.model.RenameRequest
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

data class FileBrowserState(
    val currentPath: String = "/",
    val parentPath: String? = null,
    val entries: List<FsEntry> = emptyList(),
    val pathSegments: List<Pair<String, String>> = emptyList(),
    val isLoading: Boolean = true,
    val message: String? = null,
    val error: String? = null,
    val showCreateDialog: Boolean = false,
    val showRenameDialog: FsEntry? = null,
    val showDeleteConfirm: FsEntry? = null,
    val operationInProgress: Boolean = false,
)

class FileBrowserViewModel(
    private val serverId: String,
    private val serverRepository: ServerRepository,
    private val tokenRepository: TokenRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(FileBrowserState())
    val state: StateFlow<FileBrowserState> = _state

    private var api: HolaApi? = null

    init {
        viewModelScope.launch {
            try {
                val server = serverRepository.servers.first().find { it.id == serverId }
                    ?: throw IllegalStateException("Server not found")
                val token = tokenRepository.getToken() ?: ""
                api = ApiProvider.forServer(server, token)
                navigateTo("/")
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    isLoading = false,
                    error = e.message ?: "Failed to initialize",
                )
            }
        }
    }

    fun navigateTo(path: String) {
        val client = api ?: return
        viewModelScope.launch {
            _state.value = _state.value.copy(isLoading = true, error = null)
            try {
                val response = client.browsePath(path)
                _state.value = _state.value.copy(
                    currentPath = response.path,
                    parentPath = if (response.path == response.parent) null else response.parent,
                    entries = response.entries,
                    pathSegments = buildBreadcrumbs(response.path),
                    isLoading = false,
                )
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    isLoading = false,
                    error = e.message ?: "Failed to browse path",
                )
            }
        }
    }

    fun navigateUp() {
        _state.value.parentPath?.let { navigateTo(it) }
    }

    fun registerStack(dirPath: String) {
        val client = api ?: return
        viewModelScope.launch {
            try {
                val result = client.registerStack(RegisterStackRequest(dirPath))
                if (result.success) {
                    _state.value = _state.value.copy(
                        message = "Registered stack '${result.message ?: dirPath}'",
                    )
                } else {
                    _state.value = _state.value.copy(
                        error = result.error ?: "Registration failed",
                    )
                }
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    error = e.message ?: "Failed to register stack",
                )
            }
        }
    }

    fun showCreateDialog() {
        _state.value = _state.value.copy(showCreateDialog = true)
    }

    fun dismissCreateDialog() {
        _state.value = _state.value.copy(showCreateDialog = false)
    }

    fun showRenameDialog(entry: FsEntry) {
        _state.value = _state.value.copy(showRenameDialog = entry)
    }

    fun dismissRenameDialog() {
        _state.value = _state.value.copy(showRenameDialog = null)
    }

    fun showDeleteConfirm(entry: FsEntry) {
        _state.value = _state.value.copy(showDeleteConfirm = entry)
    }

    fun dismissDeleteConfirm() {
        _state.value = _state.value.copy(showDeleteConfirm = null)
    }

    fun createFile(name: String) {
        val client = api ?: return
        val path = buildChildPath(name)
        viewModelScope.launch {
            _state.value = _state.value.copy(operationInProgress = true, showCreateDialog = false)
            try {
                val result = client.writeFile(FileWriteRequest(path, ""))
                if (result.success) {
                    _state.value = _state.value.copy(message = "File '$name' created")
                    refresh()
                } else {
                    _state.value = _state.value.copy(error = result.error ?: "Failed to create file")
                }
            } catch (e: Exception) {
                _state.value = _state.value.copy(error = e.message ?: "Failed to create file")
            } finally {
                _state.value = _state.value.copy(operationInProgress = false)
            }
        }
    }

    fun createDirectory(name: String) {
        val client = api ?: return
        val path = buildChildPath(name)
        viewModelScope.launch {
            _state.value = _state.value.copy(operationInProgress = true, showCreateDialog = false)
            try {
                val result = client.mkdir(MkdirRequest(path))
                if (result.success) {
                    _state.value = _state.value.copy(message = "Directory '$name' created")
                    refresh()
                } else {
                    _state.value = _state.value.copy(error = result.error ?: "Failed to create directory")
                }
            } catch (e: Exception) {
                _state.value = _state.value.copy(error = e.message ?: "Failed to create directory")
            } finally {
                _state.value = _state.value.copy(operationInProgress = false)
            }
        }
    }

    fun renameEntry(entry: FsEntry, newName: String) {
        val client = api ?: return
        val parentDir = entry.path.substringBeforeLast("/").ifEmpty { "/" }
        val newPath = if (parentDir == "/") "/$newName" else "$parentDir/$newName"
        viewModelScope.launch {
            _state.value = _state.value.copy(operationInProgress = true, showRenameDialog = null)
            try {
                val result = client.renamePath(RenameRequest(entry.path, newPath))
                if (result.success) {
                    _state.value = _state.value.copy(message = "Renamed to '$newName'")
                    refresh()
                } else {
                    _state.value = _state.value.copy(error = result.error ?: "Failed to rename")
                }
            } catch (e: Exception) {
                _state.value = _state.value.copy(error = e.message ?: "Failed to rename")
            } finally {
                _state.value = _state.value.copy(operationInProgress = false)
            }
        }
    }

    fun deleteEntry(entry: FsEntry) {
        val client = api ?: return
        viewModelScope.launch {
            _state.value = _state.value.copy(operationInProgress = true, showDeleteConfirm = null)
            try {
                val result = client.deletePath(entry.path)
                if (result.success) {
                    _state.value = _state.value.copy(message = "'${entry.name}' deleted")
                    refresh()
                } else {
                    _state.value = _state.value.copy(error = result.error ?: "Failed to delete")
                }
            } catch (e: Exception) {
                _state.value = _state.value.copy(error = e.message ?: "Failed to delete")
            } finally {
                _state.value = _state.value.copy(operationInProgress = false)
            }
        }
    }

    fun clearMessage() {
        _state.value = _state.value.copy(message = null, error = null)
    }

    private fun refresh() {
        navigateTo(_state.value.currentPath)
    }

    private fun buildChildPath(name: String): String {
        // Strip path separators to prevent directory traversal.
        val safeName = name.replace("/", "").replace("\\", "")
        val current = _state.value.currentPath
        return if (current.endsWith("/")) "$current$safeName" else "$current/$safeName"
    }

    private fun buildBreadcrumbs(path: String): List<Pair<String, String>> {
        if (path == "/") return listOf("/" to "/")

        val segments = mutableListOf("/" to "/")
        val parts = path.removePrefix("/").split("/")
        var accumulated = ""
        for (part in parts) {
            accumulated = "$accumulated/$part"
            segments.add(part to accumulated)
        }
        return segments
    }
}

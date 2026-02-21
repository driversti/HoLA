package dev.driversti.hola.ui.screens.resources

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.api.HolaApi
import dev.driversti.hola.data.model.DockerImage
import dev.driversti.hola.data.model.DockerNetwork
import dev.driversti.hola.data.model.DockerVolume
import dev.driversti.hola.data.model.PruneResponse
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

data class ResourceListState(
    val resourceType: String = "",
    val images: List<DockerImage> = emptyList(),
    val volumes: List<DockerVolume> = emptyList(),
    val networks: List<DockerNetwork> = emptyList(),
    val searchQuery: String = "",
    val isLoading: Boolean = true,
    val error: String? = null,
    val message: String? = null,
    val prunePreview: PruneResponse? = null,
    val deleteConfirm: DeleteConfirmState? = null,
)

data class DeleteConfirmState(
    val name: String,
    val id: String,
    val inUse: Boolean,
)

class ResourceListViewModel(
    private val serverId: String,
    private val resourceType: String,
    private val serverRepository: ServerRepository,
    private val tokenRepository: TokenRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(ResourceListState(resourceType = resourceType))
    val state: StateFlow<ResourceListState> = _state

    private var api: HolaApi? = null

    init {
        load()
    }

    fun refresh() = load()

    fun updateSearch(query: String) {
        _state.value = _state.value.copy(searchQuery = query)
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

                when (resourceType) {
                    "images" -> {
                        val resp = client.listImages()
                        _state.value = _state.value.copy(images = resp.images, isLoading = false)
                    }
                    "volumes" -> {
                        val resp = client.listVolumes()
                        _state.value = _state.value.copy(volumes = resp.volumes, isLoading = false)
                    }
                    "networks" -> {
                        val resp = client.listNetworks()
                        _state.value = _state.value.copy(networks = resp.networks, isLoading = false)
                    }
                }
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    isLoading = false,
                    error = e.message ?: "Failed to load $resourceType",
                )
            }
        }
    }

    fun confirmDelete(name: String, id: String, inUse: Boolean) {
        _state.value = _state.value.copy(deleteConfirm = DeleteConfirmState(name, id, inUse))
    }

    fun dismissDelete() {
        _state.value = _state.value.copy(deleteConfirm = null)
    }

    fun executeDelete(force: Boolean = false) {
        val confirm = _state.value.deleteConfirm ?: return
        val client = api ?: return
        viewModelScope.launch {
            _state.value = _state.value.copy(deleteConfirm = null)
            try {
                when (resourceType) {
                    "images" -> {
                        val result = client.removeImage(confirm.id, force)
                        if (!result.success) throw Exception(result.error ?: "Failed to remove image")
                    }
                    "volumes" -> {
                        val result = client.removeVolume(confirm.id, force)
                        if (!result.success) throw Exception(result.error ?: "Failed to remove volume")
                    }
                    "networks" -> {
                        val result = client.removeNetwork(confirm.id)
                        if (!result.success) throw Exception(result.error ?: "Failed to remove network")
                    }
                }
                _state.value = _state.value.copy(message = "${confirm.name} removed")
                refresh()
            } catch (e: Exception) {
                _state.value = _state.value.copy(error = e.message)
            }
        }
    }

    fun previewPrune() {
        val client = api ?: return
        viewModelScope.launch {
            try {
                val preview = when (resourceType) {
                    "images" -> client.pruneImages(dryRun = true)
                    "volumes" -> client.pruneVolumes(dryRun = true)
                    "networks" -> client.pruneNetworks(dryRun = true)
                    else -> return@launch
                }
                _state.value = _state.value.copy(prunePreview = preview)
            } catch (e: Exception) {
                _state.value = _state.value.copy(error = "Failed to preview: ${e.message}")
            }
        }
    }

    fun executePrune() {
        val client = api ?: return
        viewModelScope.launch {
            _state.value = _state.value.copy(prunePreview = null)
            try {
                when (resourceType) {
                    "images" -> client.pruneImages(dryRun = false)
                    "volumes" -> client.pruneVolumes(dryRun = false)
                    "networks" -> client.pruneNetworks(dryRun = false)
                }
                _state.value = _state.value.copy(message = "Unused ${resourceType} pruned")
                refresh()
            } catch (e: Exception) {
                _state.value = _state.value.copy(error = "Prune failed: ${e.message}")
            }
        }
    }

    fun dismissPrunePreview() {
        _state.value = _state.value.copy(prunePreview = null)
    }

    fun clearMessage() {
        _state.value = _state.value.copy(message = null, error = null)
    }
}

package dev.driversti.hola.ui.screens.resources

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.api.HolaApi
import dev.driversti.hola.data.model.DiskUsageResponse
import dev.driversti.hola.data.model.PruneResponse
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

data class ResourcesDashboardState(
    val diskUsage: DiskUsageResponse? = null,
    val isLoading: Boolean = true,
    val error: String? = null,
    val message: String? = null,
    val prunePreview: PrunePreviewState? = null,
    val pruneInProgress: Boolean = false,
)

data class PrunePreviewState(
    val title: String,
    val previews: List<PruneResponse>,
)

class ResourcesDashboardViewModel(
    private val serverId: String,
    private val serverRepository: ServerRepository,
    private val tokenRepository: TokenRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(ResourcesDashboardState())
    val state: StateFlow<ResourcesDashboardState> = _state

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

                val diskUsage = client.diskUsage()
                _state.value = _state.value.copy(
                    diskUsage = diskUsage,
                    isLoading = false,
                )
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    isLoading = false,
                    error = e.message ?: "Failed to load disk usage",
                )
            }
        }
    }

    fun previewPruneBuildCache() {
        val client = api ?: return
        viewModelScope.launch {
            try {
                val preview = client.pruneBuildCache(dryRun = true)
                _state.value = _state.value.copy(
                    prunePreview = PrunePreviewState(
                        title = "Clear Build Cache?",
                        previews = listOf(preview),
                    ),
                )
            } catch (e: Exception) {
                _state.value = _state.value.copy(error = "Failed to preview: ${e.message}")
            }
        }
    }

    fun previewPruneAll() {
        val client = api ?: return
        viewModelScope.launch {
            try {
                val imgPreview = client.pruneImages(dryRun = true)
                val volPreview = client.pruneVolumes(dryRun = true)
                val netPreview = client.pruneNetworks(dryRun = true)
                val cachePreview = client.pruneBuildCache(dryRun = true)

                _state.value = _state.value.copy(
                    prunePreview = PrunePreviewState(
                        title = "Clean All Unused?",
                        previews = listOf(imgPreview, volPreview, netPreview, cachePreview),
                    ),
                )
            } catch (e: Exception) {
                _state.value = _state.value.copy(error = "Failed to preview: ${e.message}")
            }
        }
    }

    fun executePrune() {
        val preview = _state.value.prunePreview ?: return
        val client = api ?: return
        viewModelScope.launch {
            _state.value = _state.value.copy(pruneInProgress = true, prunePreview = null)
            try {
                if (preview.title == "Clear Build Cache?") {
                    client.pruneBuildCache(dryRun = false)
                } else {
                    // Prune all
                    client.pruneImages(dryRun = false)
                    client.pruneVolumes(dryRun = false)
                    client.pruneNetworks(dryRun = false)
                    client.pruneBuildCache(dryRun = false)
                }
                _state.value = _state.value.copy(
                    pruneInProgress = false,
                    message = "Cleanup completed",
                )
                refresh()
            } catch (e: Exception) {
                _state.value = _state.value.copy(
                    pruneInProgress = false,
                    error = "Prune failed: ${e.message}",
                )
            }
        }
    }

    fun dismissPreview() {
        _state.value = _state.value.copy(prunePreview = null)
    }

    fun clearMessage() {
        _state.value = _state.value.copy(message = null, error = null)
    }
}

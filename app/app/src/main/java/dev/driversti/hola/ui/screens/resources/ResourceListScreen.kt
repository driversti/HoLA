package dev.driversti.hola.ui.screens.resources

import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.CleaningServices
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewmodel.compose.viewModel
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import dev.driversti.hola.ui.BiometricHelper
import dev.driversti.hola.ui.screens.resources.components.PrunePreviewDialog
import dev.driversti.hola.ui.screens.resources.components.ResourceItem
import dev.driversti.hola.ui.screens.resources.components.formatBytes

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ResourceListScreen(
    serverId: String,
    resourceType: String,
    serverRepository: ServerRepository,
    tokenRepository: TokenRepository,
    onBack: () -> Unit,
    viewModel: ResourceListViewModel = viewModel(
        factory = object : ViewModelProvider.Factory {
            @Suppress("UNCHECKED_CAST")
            override fun <T : androidx.lifecycle.ViewModel> create(modelClass: Class<T>): T {
                return ResourceListViewModel(serverId, resourceType, serverRepository, tokenRepository) as T
            }
        }
    ),
) {
    val state by viewModel.state.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }
    val context = LocalContext.current

    val title = resourceType.replaceFirstChar { it.uppercase() }

    LaunchedEffect(state.message) {
        state.message?.let {
            snackbarHostState.showSnackbar(it)
            viewModel.clearMessage()
        }
    }

    LaunchedEffect(state.error) {
        state.error?.let {
            snackbarHostState.showSnackbar(it)
            viewModel.clearMessage()
        }
    }

    // Prune preview dialog
    state.prunePreview?.let { preview ->
        PrunePreviewDialog(
            title = "Prune Unused $title?",
            itemCount = preview.count,
            spaceReclaimed = preview.spaceReclaimed,
            items = preview.itemsToRemove,
            onConfirm = {
                val activity = context as? FragmentActivity
                if (activity != null) {
                    BiometricHelper.authenticate(
                        activity = activity,
                        title = "Confirm Prune",
                        subtitle = "Authenticate to prune unused $resourceType",
                        onSuccess = { viewModel.executePrune() },
                        onError = { /* user cancelled */ },
                    )
                } else {
                    viewModel.executePrune()
                }
            },
            onDismiss = viewModel::dismissPrunePreview,
        )
    }

    // Delete confirmation dialog
    state.deleteConfirm?.let { confirm ->
        AlertDialog(
            onDismissRequest = viewModel::dismissDelete,
            title = { Text("Delete ${confirm.name}?") },
            text = {
                if (confirm.inUse) {
                    Text(
                        "This resource is currently in use. Removing it may cause running containers to fail.",
                        color = MaterialTheme.colorScheme.error,
                    )
                } else {
                    Text("This will permanently remove this resource.")
                }
            },
            confirmButton = {
                TextButton(onClick = {
                    val activity = context as? FragmentActivity
                    if (activity != null) {
                        BiometricHelper.authenticate(
                            activity = activity,
                            title = "Confirm Delete",
                            subtitle = "Authenticate to delete ${confirm.name}",
                            onSuccess = { viewModel.executeDelete(force = confirm.inUse) },
                            onError = { /* user cancelled */ },
                        )
                    } else {
                        viewModel.executeDelete(force = confirm.inUse)
                    }
                }) {
                    Text("Delete", color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = {
                TextButton(onClick = viewModel::dismissDelete) { Text("Cancel") }
            },
        )
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(title) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = { viewModel.previewPrune() }) {
                        Icon(
                            Icons.Default.CleaningServices,
                            contentDescription = "Prune unused",
                            tint = MaterialTheme.colorScheme.error,
                        )
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
    ) { padding ->
        PullToRefreshBox(
            isRefreshing = state.isLoading,
            onRefresh = viewModel::refresh,
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
        ) {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                // Search bar
                item {
                    OutlinedTextField(
                        value = state.searchQuery,
                        onValueChange = viewModel::updateSearch,
                        label = { Text("Search") },
                        modifier = Modifier.fillMaxWidth(),
                        singleLine = true,
                    )
                }

                when (resourceType) {
                    "images" -> {
                        val query = state.searchQuery.lowercase()
                        val filtered = if (query.isBlank()) state.images else {
                            state.images.filter { img ->
                                img.tags.any { it.lowercase().contains(query) } ||
                                    img.id.lowercase().contains(query)
                            }
                        }
                        items(filtered, key = { it.id }) { img ->
                            val displayName = img.tags.firstOrNull()?.takeIf { it != "<none>:<none>" }
                                ?: img.id.take(12)
                            ResourceItem(
                                name = displayName,
                                subtitle = if (img.containers.isNotEmpty()) {
                                    "Used by: ${img.containers.joinToString(", ")}"
                                } else null,
                                size = img.size,
                                inUse = img.inUse,
                                modifier = Modifier.combinedClickable(
                                    onClick = {},
                                    onLongClick = {
                                        viewModel.confirmDelete(displayName, img.id, img.inUse)
                                    },
                                ),
                            )
                        }
                    }
                    "volumes" -> {
                        val query = state.searchQuery.lowercase()
                        val filtered = if (query.isBlank()) state.volumes else {
                            state.volumes.filter { it.name.lowercase().contains(query) }
                        }
                        items(filtered, key = { it.name }) { vol ->
                            ResourceItem(
                                name = vol.name,
                                subtitle = if (vol.containers.isNotEmpty()) {
                                    "Used by: ${vol.containers.joinToString(", ")}"
                                } else null,
                                size = vol.size,
                                inUse = vol.inUse,
                                extra = "Driver: ${vol.driver}",
                                modifier = Modifier.combinedClickable(
                                    onClick = {},
                                    onLongClick = {
                                        viewModel.confirmDelete(vol.name, vol.name, vol.inUse)
                                    },
                                ),
                            )
                        }
                    }
                    "networks" -> {
                        val query = state.searchQuery.lowercase()
                        val filtered = if (query.isBlank()) state.networks else {
                            state.networks.filter { it.name.lowercase().contains(query) }
                        }
                        items(filtered, key = { it.id }) { net ->
                            ResourceItem(
                                name = net.name,
                                subtitle = if (net.containers.isNotEmpty()) {
                                    "Used by: ${net.containers.joinToString(", ")}"
                                } else null,
                                inUse = net.inUse,
                                extra = "${net.driver} / ${net.scope}${if (net.internal) " (internal)" else ""}",
                                canDelete = !net.builtin,
                                modifier = if (!net.builtin) {
                                    Modifier.combinedClickable(
                                        onClick = {},
                                        onLongClick = {
                                            viewModel.confirmDelete(net.name, net.id, net.inUse)
                                        },
                                    )
                                } else {
                                    Modifier
                                },
                            )
                        }
                    }
                }

                // Empty state
                val isEmpty = when (resourceType) {
                    "images" -> state.images.isEmpty()
                    "volumes" -> state.volumes.isEmpty()
                    "networks" -> state.networks.isEmpty()
                    else -> true
                }
                if (isEmpty && !state.isLoading) {
                    item {
                        Text(
                            "No $resourceType found",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(16.dp),
                        )
                    }
                }
            }
        }
    }
}

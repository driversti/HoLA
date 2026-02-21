package dev.driversti.hola.ui.screens.resources

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.CleaningServices
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
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
import dev.driversti.hola.ui.screens.resources.components.BuildCacheCard
import dev.driversti.hola.ui.screens.resources.components.DiskUsageCard
import dev.driversti.hola.ui.screens.resources.components.PrunePreviewDialog

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ResourcesDashboardScreen(
    serverId: String,
    serverRepository: ServerRepository,
    tokenRepository: TokenRepository,
    onResourceClick: (String) -> Unit,
    onBack: () -> Unit,
    viewModel: ResourcesDashboardViewModel = viewModel(
        factory = object : ViewModelProvider.Factory {
            @Suppress("UNCHECKED_CAST")
            override fun <T : androidx.lifecycle.ViewModel> create(modelClass: Class<T>): T {
                return ResourcesDashboardViewModel(serverId, serverRepository, tokenRepository) as T
            }
        }
    ),
) {
    val state by viewModel.state.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }
    val context = LocalContext.current

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
        val totalCount = preview.previews.sumOf { it.count }
        val totalSpace = preview.previews.sumOf { it.spaceReclaimed }
        val allItems = preview.previews.flatMap { it.itemsToRemove }

        PrunePreviewDialog(
            title = preview.title,
            itemCount = totalCount,
            spaceReclaimed = totalSpace,
            items = allItems,
            onConfirm = {
                val activity = context as? FragmentActivity
                if (activity != null) {
                    BiometricHelper.authenticate(
                        activity = activity,
                        title = "Confirm Cleanup",
                        subtitle = "Authenticate to prune Docker resources",
                        onSuccess = { viewModel.executePrune() },
                        onError = { /* user cancelled */ },
                    )
                } else {
                    viewModel.executePrune()
                }
            },
            onDismiss = viewModel::dismissPreview,
        )
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Resources") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
        floatingActionButton = {
            FloatingActionButton(
                onClick = { viewModel.previewPruneAll() },
                containerColor = MaterialTheme.colorScheme.errorContainer,
                contentColor = MaterialTheme.colorScheme.onErrorContainer,
            ) {
                Icon(Icons.Default.CleaningServices, contentDescription = "Clean all unused")
            }
        },
    ) { padding ->
        PullToRefreshBox(
            isRefreshing = state.isLoading,
            onRefresh = viewModel::refresh,
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
        ) {
            val du = state.diskUsage

            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                if (du != null) {
                    item {
                        DiskUsageCard(
                            title = "Images",
                            totalCount = du.images.totalCount,
                            inUseCount = du.images.inUseCount,
                            totalSize = du.images.totalSize,
                            reclaimableSize = du.images.reclaimableSize,
                            onClick = { onResourceClick("images") },
                        )
                    }

                    item {
                        DiskUsageCard(
                            title = "Volumes",
                            totalCount = du.volumes.totalCount,
                            inUseCount = du.volumes.inUseCount,
                            totalSize = du.volumes.totalSize,
                            reclaimableSize = du.volumes.reclaimableSize,
                            onClick = { onResourceClick("volumes") },
                        )
                    }

                    item {
                        DiskUsageCard(
                            title = "Networks",
                            totalCount = du.networks.totalCount,
                            inUseCount = du.networks.inUseCount,
                            reclaimableCount = du.networks.reclaimableCount,
                            onClick = { onResourceClick("networks") },
                        )
                    }

                    item {
                        BuildCacheCard(
                            totalSize = du.buildCache.totalSize,
                            onClear = { viewModel.previewPruneBuildCache() },
                        )
                    }
                }
            }
        }
    }
}

package dev.driversti.hola.ui.screens.serverdetail

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.OutlinedButton
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Storage
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewmodel.compose.viewModel
import dev.driversti.hola.data.api.WebSocketManager
import dev.driversti.hola.data.model.Stack
import dev.driversti.hola.data.model.SystemMetrics
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ServerDetailScreen(
    serverId: String,
    serverRepository: ServerRepository,
    tokenRepository: TokenRepository,
    webSocketManager: WebSocketManager,
    onStackClick: (String) -> Unit,
    onAddStack: () -> Unit,
    onResources: () -> Unit,
    onBack: () -> Unit,
    viewModel: ServerDetailViewModel = viewModel(
        factory = object : ViewModelProvider.Factory {
            @Suppress("UNCHECKED_CAST")
            override fun <T : androidx.lifecycle.ViewModel> create(modelClass: Class<T>): T {
                return ServerDetailViewModel(serverId, serverRepository, tokenRepository, webSocketManager) as T
            }
        }
    ),
) {
    val state by viewModel.state.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }

    LaunchedEffect(state.error) {
        state.error?.let { snackbarHostState.showSnackbar(it) }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(state.serverName.ifEmpty { "Server" }) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = onResources) {
                        Icon(Icons.Default.Storage, contentDescription = "Resources")
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
        floatingActionButton = {
            FloatingActionButton(onClick = onAddStack) {
                Icon(Icons.Default.Add, contentDescription = "Add stack")
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
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                state.metrics?.let { metrics ->
                    item { MetricsHeader(metrics) }
                }

                item {
                    Text(
                        "Stacks",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                }

                if (state.stacks.isEmpty() && !state.isLoading) {
                    item {
                        Text(
                            "No stacks discovered",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }

                items(state.stacks, key = { it.name }) { stack ->
                    StackCard(stack = stack, onClick = { onStackClick(stack.name) })
                }
            }
        }
    }
}

@Composable
private fun MetricsHeader(metrics: SystemMetrics) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceVariant,
        ),
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            MetricRow("CPU", metrics.cpu.usagePercent)
            Spacer(modifier = Modifier.height(8.dp))
            MetricRow("RAM", metrics.memory.usagePercent)
            metrics.disk.forEach { disk ->
                Spacer(modifier = Modifier.height(8.dp))
                MetricRow("Disk ${disk.mountPoint}", disk.usagePercent)
            }
        }
    }
}

@Composable
private fun MetricRow(label: String, percent: Double) {
    Column {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text(label, style = MaterialTheme.typography.bodyMedium)
            Text("${percent.toInt()}%", style = MaterialTheme.typography.bodyMedium)
        }
        Spacer(modifier = Modifier.height(4.dp))
        LinearProgressIndicator(
            progress = { (percent / 100).toFloat() },
            modifier = Modifier
                .fillMaxWidth()
                .height(6.dp),
        )
    }
}

@Composable
private fun StackCard(stack: Stack, onClick: () -> Unit) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val dotColor = when (stack.status) {
                "running" -> Color(0xFF4CAF50)
                "partial" -> Color(0xFFFFC107)
                "down" -> Color(0xFF9E9E9E)
                else -> Color(0xFFF44336)
            }
            androidx.compose.foundation.Canvas(modifier = Modifier.size(12.dp)) {
                drawCircle(color = dotColor)
            }
            Spacer(modifier = Modifier.width(12.dp))
            Text(
                stack.name,
                style = MaterialTheme.typography.titleSmall,
                modifier = Modifier.weight(1f),
            )
            Text(
                "${stack.runningCount}/${stack.serviceCount}",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

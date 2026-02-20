package dev.driversti.hola.ui.screens.stackdetail

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Description
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewmodel.compose.viewModel
import dev.driversti.hola.data.model.ContainerInfo
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import dev.driversti.hola.ui.BiometricHelper

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun StackDetailScreen(
    serverId: String,
    stackName: String,
    serverRepository: ServerRepository,
    tokenRepository: TokenRepository,
    onContainerClick: (String) -> Unit,
    onViewCompose: () -> Unit,
    onBack: () -> Unit,
    viewModel: StackDetailViewModel = viewModel(
        factory = object : ViewModelProvider.Factory {
            @Suppress("UNCHECKED_CAST")
            override fun <T : androidx.lifecycle.ViewModel> create(modelClass: Class<T>): T {
                return StackDetailViewModel(serverId, stackName, serverRepository, tokenRepository) as T
            }
        }
    ),
) {
    val state by viewModel.state.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }
    val context = LocalContext.current
    var pullConfirmVisible by remember { mutableStateOf(false) }

    LaunchedEffect(state.message, state.error) {
        val msg = state.message ?: state.error
        if (msg != null) {
            snackbarHostState.showSnackbar(msg)
            viewModel.clearMessage()
        }
    }

    if (pullConfirmVisible) {
        AlertDialog(
            onDismissRequest = { pullConfirmVisible = false },
            title = { Text("Pull Images") },
            text = { Text("This will pull latest images for all services in $stackName. Continue?") },
            confirmButton = {
                TextButton(onClick = {
                    pullConfirmVisible = false
                    viewModel.executeAction("pull")
                }) { Text("Pull") }
            },
            dismissButton = {
                TextButton(onClick = { pullConfirmVisible = false }) { Text("Cancel") }
            },
        )
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(stackName) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
    ) { padding ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            // Action buttons
            item {
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    val inProgress = state.actionInProgress

                    ActionButton("Start", inProgress) {
                        viewModel.executeAction("start")
                    }
                    ActionButton("Stop", inProgress) {
                        withBiometric(context as? FragmentActivity, "Stop Stack", "Confirm to stop $stackName") {
                            viewModel.executeAction("stop")
                        }
                    }
                    ActionButton("Down", inProgress) {
                        withBiometric(context as? FragmentActivity, "Bring Down Stack", "Confirm to bring down $stackName") {
                            viewModel.executeAction("down")
                        }
                    }
                    ActionButton("Pull", inProgress) {
                        pullConfirmVisible = true
                    }
                    ActionButton("Restart", inProgress) {
                        withBiometric(context as? FragmentActivity, "Restart Stack", "Confirm to restart $stackName") {
                            viewModel.executeAction("restart")
                        }
                    }
                }
            }

            // Loading indicator
            if (state.isLoading) {
                item {
                    CircularProgressIndicator(modifier = Modifier.padding(16.dp))
                }
            }

            // Containers header
            item {
                Text(
                    "Containers",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                )
            }

            items(state.containers, key = { it.id }) { container ->
                ContainerCard(
                    container = container,
                    onClick = { onContainerClick(container.id) },
                )
            }

            // View compose file
            item {
                OutlinedButton(
                    onClick = onViewCompose,
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Icon(Icons.Default.Description, contentDescription = null)
                    Spacer(modifier = Modifier.width(8.dp))
                    Text("View Compose File")
                }
            }
        }
    }
}

@Composable
private fun ActionButton(
    label: String,
    actionInProgress: String?,
    onClick: () -> Unit,
) {
    val isThisAction = actionInProgress == label.lowercase()
    val anyInProgress = actionInProgress != null

    Button(
        onClick = onClick,
        enabled = !anyInProgress,
        colors = if (label in listOf("Stop", "Down")) {
            ButtonDefaults.buttonColors(
                containerColor = MaterialTheme.colorScheme.error,
            )
        } else {
            ButtonDefaults.buttonColors()
        },
    ) {
        if (isThisAction) {
            CircularProgressIndicator(
                modifier = Modifier.size(16.dp),
                strokeWidth = 2.dp,
                color = MaterialTheme.colorScheme.onPrimary,
            )
            Spacer(modifier = Modifier.width(8.dp))
        }
        Text(label)
    }
}

@Composable
private fun ContainerCard(container: ContainerInfo, onClick: () -> Unit) {
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
            val dotColor = if (container.state == "running") Color(0xFF4CAF50) else Color(0xFFF44336)
            androidx.compose.foundation.Canvas(modifier = Modifier.size(10.dp)) {
                drawCircle(color = dotColor)
            }
            Spacer(modifier = Modifier.width(12.dp))
            Column {
                Text(container.service, style = MaterialTheme.typography.titleSmall)
                Text(
                    container.image,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}

private fun withBiometric(
    activity: FragmentActivity?,
    title: String,
    subtitle: String,
    onSuccess: () -> Unit,
) {
    if (activity != null) {
        BiometricHelper.authenticate(
            activity = activity,
            title = title,
            subtitle = subtitle,
            onSuccess = onSuccess,
            onError = { /* silently ignore — user cancelled */ },
        )
    } else {
        onSuccess()
    }
}

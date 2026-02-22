package dev.driversti.hola.ui.screens.filebrowser

import android.text.format.Formatter
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.InsertDriveFile
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Code
import androidx.compose.material.icons.filled.Description
import androidx.compose.material.icons.filled.Folder
import androidx.compose.material.icons.filled.Image
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Terminal
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
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
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewmodel.compose.viewModel
import dev.driversti.hola.data.model.FsEntry
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FileBrowserScreen(
    serverId: String,
    serverRepository: ServerRepository,
    tokenRepository: TokenRepository,
    onBack: () -> Unit,
    onOpenFile: (String) -> Unit = {},
    viewModel: FileBrowserViewModel = viewModel(
        factory = object : ViewModelProvider.Factory {
            @Suppress("UNCHECKED_CAST")
            override fun <T : androidx.lifecycle.ViewModel> create(modelClass: Class<T>): T {
                return FileBrowserViewModel(serverId, serverRepository, tokenRepository) as T
            }
        }
    ),
) {
    val state by viewModel.state.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }

    LaunchedEffect(state.message, state.error) {
        val text = state.message ?: state.error
        if (text != null) {
            snackbarHostState.showSnackbar(text)
            viewModel.clearMessage()
        }
    }

    // Create dialog
    if (state.showCreateDialog) {
        CreateDialog(
            onCreateFile = { name -> viewModel.createFile(name) },
            onCreateDirectory = { name -> viewModel.createDirectory(name) },
            onDismiss = { viewModel.dismissCreateDialog() },
        )
    }

    // Rename dialog
    state.showRenameDialog?.let { entry ->
        RenameDialog(
            entry = entry,
            onRename = { newName -> viewModel.renameEntry(entry, newName) },
            onDismiss = { viewModel.dismissRenameDialog() },
        )
    }

    // Delete confirmation dialog
    state.showDeleteConfirm?.let { entry ->
        DeleteConfirmDialog(
            entry = entry,
            onConfirm = { viewModel.deleteEntry(entry) },
            onDismiss = { viewModel.dismissDeleteConfirm() },
        )
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("File Manager") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = { viewModel.showCreateDialog() }) {
                        Icon(Icons.Default.Add, contentDescription = "Create")
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
        ) {
            BreadcrumbRow(
                segments = state.pathSegments,
                onSegmentClick = { path -> viewModel.navigateTo(path) },
            )

            HorizontalDivider()

            if (state.isLoading) {
                Text(
                    "Loading...",
                    modifier = Modifier.padding(16.dp),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            } else if (state.entries.isEmpty()) {
                Text(
                    "Empty directory",
                    modifier = Modifier.padding(16.dp),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            } else {
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(vertical = 4.dp),
                ) {
                    items(state.entries, key = { it.path }) { entry ->
                        FsEntryRow(
                            entry = entry,
                            onNavigate = { viewModel.navigateTo(entry.path) },
                            onOpenFile = { onOpenFile(entry.path) },
                            onRegister = { viewModel.registerStack(entry.path) },
                            onRename = { viewModel.showRenameDialog(entry) },
                            onDelete = { viewModel.showDeleteConfirm(entry) },
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun BreadcrumbRow(
    segments: List<Pair<String, String>>,
    onSegmentClick: (String) -> Unit,
) {
    LazyRow(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 12.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(2.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        itemsIndexed(segments) { index, (label, fullPath) ->
            if (index > 0) {
                Text(
                    ">",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = 2.dp),
                )
            }
            TextButton(onClick = { onSegmentClick(fullPath) }) {
                Text(
                    label,
                    style = MaterialTheme.typography.bodySmall,
                    fontWeight = if (index == segments.lastIndex) FontWeight.Bold else FontWeight.Normal,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}

@Composable
private fun FsEntryRow(
    entry: FsEntry,
    onNavigate: () -> Unit,
    onOpenFile: () -> Unit,
    onRegister: () -> Unit,
    onRename: () -> Unit,
    onDelete: () -> Unit,
) {
    var showMenu by remember { mutableStateOf(false) }
    val context = LocalContext.current

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = if (entry.isDir) onNavigate else onOpenFile)
            .padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            imageVector = fileTypeIcon(entry),
            contentDescription = null,
            modifier = Modifier.size(24.dp),
            tint = if (entry.isDir) {
                MaterialTheme.colorScheme.primary
            } else {
                MaterialTheme.colorScheme.onSurfaceVariant
            },
        )
        Spacer(modifier = Modifier.width(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                entry.name,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (!entry.isDir || entry.modifiedAt > 0) {
                Spacer(modifier = Modifier.height(2.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    if (!entry.isDir && entry.size > 0) {
                        Text(
                            Formatter.formatShortFileSize(context, entry.size),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    if (entry.modifiedAt > 0) {
                        Text(
                            formatDate(entry.modifiedAt),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
        }

        if (entry.isDir && entry.hasComposeFile) {
            IconButton(onClick = onRegister) {
                Icon(
                    Icons.Default.Add,
                    contentDescription = "Register stack",
                    tint = MaterialTheme.colorScheme.primary,
                )
            }
        }

        // Overflow menu
        IconButton(onClick = { showMenu = true }) {
            Icon(
                Icons.Default.MoreVert,
                contentDescription = "More options",
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        DropdownMenu(expanded = showMenu, onDismissRequest = { showMenu = false }) {
            DropdownMenuItem(
                text = { Text("Rename") },
                onClick = {
                    showMenu = false
                    onRename()
                },
            )
            DropdownMenuItem(
                text = { Text("Delete", color = MaterialTheme.colorScheme.error) },
                onClick = {
                    showMenu = false
                    onDelete()
                },
            )
        }
    }
}

@Composable
private fun CreateDialog(
    onCreateFile: (String) -> Unit,
    onCreateDirectory: (String) -> Unit,
    onDismiss: () -> Unit,
) {
    var name by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Create New") },
        text = {
            OutlinedTextField(
                value = name,
                onValueChange = { name = it },
                label = { Text("Name") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
        },
        confirmButton = {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TextButton(
                    onClick = { if (name.isNotBlank()) onCreateFile(name.trim()) },
                    enabled = name.isNotBlank(),
                ) {
                    Text("File")
                }
                TextButton(
                    onClick = { if (name.isNotBlank()) onCreateDirectory(name.trim()) },
                    enabled = name.isNotBlank(),
                ) {
                    Text("Folder")
                }
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("Cancel") }
        },
    )
}

@Composable
private fun RenameDialog(
    entry: FsEntry,
    onRename: (String) -> Unit,
    onDismiss: () -> Unit,
) {
    var newName by remember { mutableStateOf(entry.name) }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Rename") },
        text = {
            OutlinedTextField(
                value = newName,
                onValueChange = { newName = it },
                label = { Text("New name") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
        },
        confirmButton = {
            TextButton(
                onClick = { if (newName.isNotBlank() && newName != entry.name) onRename(newName.trim()) },
                enabled = newName.isNotBlank() && newName != entry.name,
            ) {
                Text("Rename")
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("Cancel") }
        },
    )
}

@Composable
private fun DeleteConfirmDialog(
    entry: FsEntry,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    val typeLabel = if (entry.isDir) "directory" else "file"

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Delete $typeLabel?") },
        text = {
            Text("Are you sure you want to delete '${entry.name}'?${if (entry.isDir) " This will delete all contents." else ""}")
        },
        confirmButton = {
            TextButton(onClick = onConfirm) {
                Text("Delete", color = MaterialTheme.colorScheme.error)
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("Cancel") }
        },
    )
}

private fun fileTypeIcon(entry: FsEntry): ImageVector {
    if (entry.isDir) return Icons.Default.Folder
    return when (entry.fileType.lowercase()) {
        "yml", "yaml", "toml", "ini", "conf", "cfg", "properties" -> Icons.Default.Settings
        "json", "xml", "html", "css", "js", "ts", "kt", "java",
        "go", "py", "rb", "rs", "c", "cpp", "h", "swift" -> Icons.Default.Code
        "sh", "bash", "zsh", "fish", "bat", "ps1" -> Icons.Default.Terminal
        "md", "txt", "log", "csv", "env" -> Icons.Default.Description
        "png", "jpg", "jpeg", "gif", "svg", "ico", "webp" -> Icons.Default.Image
        else -> Icons.AutoMirrored.Filled.InsertDriveFile
    }
}

private fun formatDate(epochSeconds: Long): String {
    val sdf = SimpleDateFormat("MMM d, yyyy", Locale.US)
    return sdf.format(Date(epochSeconds * 1000))
}

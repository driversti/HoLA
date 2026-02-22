package dev.driversti.hola.ui.screens.fileeditor

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.driversti.hola.data.api.ApiProvider
import dev.driversti.hola.data.model.FileWriteRequest
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.TokenRepository
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FileEditorScreen(
    serverId: String,
    filePath: String,
    serverRepository: ServerRepository,
    tokenRepository: TokenRepository,
    onBack: () -> Unit,
) {
    var content by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var isLoading by remember { mutableStateOf(true) }
    var isEditing by remember { mutableStateOf(false) }
    var editedContent by remember { mutableStateOf("") }
    var isSaving by remember { mutableStateOf(false) }
    var showDiscardDialog by remember { mutableStateOf(false) }
    var snackbarMessage by remember { mutableStateOf<String?>(null) }
    var api by remember { mutableStateOf<dev.driversti.hola.data.api.HolaApi?>(null) }
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }

    val fileName = filePath.substringAfterLast("/")

    BackHandler(enabled = isEditing) {
        if (editedContent != content) {
            showDiscardDialog = true
        } else {
            isEditing = false
        }
    }

    LaunchedEffect(snackbarMessage) {
        snackbarMessage?.let {
            snackbarHostState.showSnackbar(it)
            snackbarMessage = null
        }
    }

    LaunchedEffect(Unit) {
        try {
            val server = serverRepository.servers.first().find { it.id == serverId }
                ?: throw IllegalStateException("Server not found")
            val token = tokenRepository.getToken() ?: ""
            api = ApiProvider.forServer(server, token)
            val response = api!!.readFile(filePath)
            content = response.content
        } catch (e: Exception) {
            error = e.message ?: "Failed to load file"
        } finally {
            isLoading = false
        }
    }

    if (showDiscardDialog) {
        AlertDialog(
            onDismissRequest = { showDiscardDialog = false },
            title = { Text("Discard changes?") },
            text = { Text("You have unsaved changes. Discard them?") },
            confirmButton = {
                TextButton(onClick = {
                    showDiscardDialog = false
                    isEditing = false
                    editedContent = ""
                }) {
                    Text("Discard")
                }
            },
            dismissButton = {
                TextButton(onClick = { showDiscardDialog = false }) {
                    Text("Cancel")
                }
            },
        )
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text(
                            if (isEditing) "Editing" else fileName,
                            maxLines = 1,
                        )
                        Text(
                            filePath,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            maxLines = 1,
                        )
                    }
                },
                navigationIcon = {
                    IconButton(onClick = {
                        when {
                            isEditing && editedContent != content -> showDiscardDialog = true
                            isEditing -> {
                                isEditing = false
                                editedContent = ""
                            }
                            else -> onBack()
                        }
                    }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    if (content != null && !isEditing) {
                        IconButton(onClick = {
                            editedContent = content!!
                            isEditing = true
                        }) {
                            Icon(Icons.Default.Edit, contentDescription = "Edit")
                        }
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
        floatingActionButton = {
            if (isEditing && editedContent != content) {
                FloatingActionButton(
                    onClick = {
                        if (!isSaving) {
                            isSaving = true
                            scope.launch {
                                try {
                                    val currentApi = api
                                        ?: throw IllegalStateException("API not initialized")
                                    val response = currentApi.writeFile(
                                        FileWriteRequest(filePath, editedContent),
                                    )
                                    if (response.success) {
                                        content = editedContent
                                        isEditing = false
                                        snackbarMessage = "File saved successfully"
                                    } else {
                                        snackbarMessage =
                                            response.error ?: "Failed to save file"
                                    }
                                } catch (e: Exception) {
                                    snackbarMessage =
                                        e.message ?: "Failed to save file"
                                } finally {
                                    isSaving = false
                                }
                            }
                        }
                    },
                ) {
                    if (isSaving) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(24.dp),
                            color = MaterialTheme.colorScheme.onPrimaryContainer,
                            strokeWidth = 2.dp,
                        )
                    } else {
                        Icon(Icons.Default.Check, contentDescription = "Save")
                    }
                }
            }
        },
    ) { padding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
        ) {
            when {
                isLoading -> {
                    CircularProgressIndicator(
                        modifier = Modifier.align(Alignment.Center),
                    )
                }
                error != null -> {
                    Text(
                        error!!,
                        modifier = Modifier
                            .align(Alignment.Center)
                            .padding(16.dp),
                        color = MaterialTheme.colorScheme.error,
                    )
                }
                content != null -> {
                    val textToDisplay = if (isEditing) editedContent else content!!
                    val lines = textToDisplay.lines()
                    val gutterWidth = (lines.size.toString().length * 10 + 16).dp
                    val verticalScrollState = rememberScrollState()

                    Row(
                        modifier = Modifier
                            .fillMaxSize()
                            .padding(8.dp),
                    ) {
                        // Line number gutter — pinned horizontally, scrolls vertically
                        Column(
                            modifier = Modifier
                                .width(gutterWidth)
                                .verticalScroll(verticalScrollState),
                            horizontalAlignment = Alignment.End,
                        ) {
                            lines.forEachIndexed { index, _ ->
                                Text(
                                    text = "${index + 1}",
                                    style = TextStyle(
                                        fontFamily = FontFamily.Monospace,
                                        fontSize = 13.sp,
                                        lineHeight = 18.sp,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                                    ),
                                    modifier = Modifier.padding(end = 8.dp),
                                )
                            }
                        }

                        // Content area — scrolls both axes
                        Box(
                            modifier = Modifier
                                .weight(1f)
                                .verticalScroll(verticalScrollState)
                                .horizontalScroll(rememberScrollState()),
                        ) {
                            if (isEditing) {
                                BasicTextField(
                                    value = editedContent,
                                    onValueChange = { editedContent = it },
                                    modifier = Modifier.fillMaxWidth(),
                                    textStyle = TextStyle(
                                        fontFamily = FontFamily.Monospace,
                                        fontSize = 13.sp,
                                        lineHeight = 18.sp,
                                        color = MaterialTheme.colorScheme.onSurface,
                                    ),
                                )
                            } else {
                                Text(
                                    text = content!!,
                                    modifier = Modifier.fillMaxWidth(),
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 13.sp,
                                    lineHeight = 18.sp,
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

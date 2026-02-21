package dev.driversti.hola.ui.screens.resources.components

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp

@Composable
fun PrunePreviewDialog(
    title: String,
    itemCount: Int,
    spaceReclaimed: Long,
    items: List<String>,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column {
                if (itemCount == 0) {
                    Text("Nothing to prune.", style = MaterialTheme.typography.bodyMedium)
                } else {
                    Text(
                        "$itemCount item${if (itemCount != 1) "s" else ""} to remove",
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                    if (spaceReclaimed > 0) {
                        Text(
                            "Space to reclaim: ${formatBytes(spaceReclaimed)}",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary,
                        )
                    }
                    Spacer(modifier = Modifier.height(12.dp))
                    LazyColumn(
                        modifier = Modifier
                            .fillMaxWidth()
                            .heightIn(max = 240.dp),
                    ) {
                        items(items) { item ->
                            Text(
                                text = item,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(vertical = 2.dp),
                                maxLines = 1,
                            )
                        }
                    }
                }
            }
        },
        confirmButton = {
            if (itemCount > 0) {
                TextButton(onClick = onConfirm) {
                    Text("Prune", color = MaterialTheme.colorScheme.error)
                }
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(if (itemCount == 0) "Close" else "Cancel")
            }
        },
    )
}

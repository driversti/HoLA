package dev.driversti.hola.data.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement

// --- WebSocket Message Envelope ---

@Serializable
data class WsMessage(
    val type: String,
    val id: String? = null,
    val payload: JsonElement? = null,
)

// --- Container Event (from Docker events stream) ---

@Serializable
data class ContainerEvent(
    val action: String,
    @SerialName("container_id") val containerId: String,
    @SerialName("container_name") val containerName: String,
    val image: String,
    val stack: String,
    val status: String,
    val time: Long,
)

// --- Log Line (from log stream) ---

@Serializable
data class WsLogLine(
    @SerialName("container_id") val containerId: String,
    val timestamp: String,
    val stream: String,
    val message: String,
)

// --- Container Stats (from container_stats stream) ---

@Serializable
data class WsContainerStats(
    @SerialName("container_id") val containerId: String,
    @SerialName("cpu_percent") val cpuPercent: Float,
    @SerialName("mem_used_bytes") val memUsedBytes: Long,
    @SerialName("mem_limit_bytes") val memLimitBytes: Long,
    @SerialName("mem_percent") val memPercent: Float,
)

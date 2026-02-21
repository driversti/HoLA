package dev.driversti.hola.data.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

// --- Health ---

@Serializable
data class HealthResponse(val status: String)

// --- Agent Info ---

@Serializable
data class AgentInfo(
    val version: String,
    val hostname: String,
    val os: String,
    val arch: String,
    @SerialName("docker_version") val dockerVersion: String,
)

// --- System Metrics ---

@Serializable
data class SystemMetrics(
    val hostname: String,
    @SerialName("uptime_seconds") val uptimeSeconds: Long,
    val cpu: CpuMetrics,
    val memory: MemoryMetrics,
    val disk: List<DiskMetric>,
)

@Serializable
data class CpuMetrics(
    @SerialName("usage_percent") val usagePercent: Double,
    val cores: Int,
)

@Serializable
data class MemoryMetrics(
    @SerialName("total_bytes") val totalBytes: Long,
    @SerialName("used_bytes") val usedBytes: Long,
    @SerialName("usage_percent") val usagePercent: Double,
)

@Serializable
data class DiskMetric(
    @SerialName("mount_point") val mountPoint: String,
    @SerialName("total_bytes") val totalBytes: Long,
    @SerialName("used_bytes") val usedBytes: Long,
    @SerialName("usage_percent") val usagePercent: Double,
)

// --- Stacks ---

@Serializable
data class StackListResponse(val stacks: List<Stack>)

@Serializable
data class Stack(
    val name: String,
    val status: String,
    @SerialName("service_count") val serviceCount: Int = 0,
    @SerialName("running_count") val runningCount: Int = 0,
    @SerialName("working_dir") val workingDir: String = "",
    val registered: Boolean = false,
)

@Serializable
data class StackDetail(
    val name: String,
    val status: String,
    @SerialName("working_dir") val workingDir: String,
    val containers: List<ContainerInfo>,
)

@Serializable
data class ContainerInfo(
    val id: String,
    val name: String,
    val service: String,
    val image: String,
    val status: String,
    val state: String,
    @SerialName("created_at") val createdAt: Long,
)

// --- Container Logs ---

@Serializable
data class ContainerLogsResponse(
    @SerialName("container_id") val containerId: String,
    @SerialName("container_name") val containerName: String,
    val lines: List<LogEntry>,
)

@Serializable
data class LogEntry(
    val timestamp: String,
    val stream: String,
    val message: String,
)

// --- Compose File ---

@Serializable
data class ComposeFileResponse(
    val content: String,
    val path: String,
)

// --- Action Response ---

@Serializable
data class ActionResponse(
    val success: Boolean,
    val message: String? = null,
    val error: String? = null,
)

// --- Filesystem Browse ---

@Serializable
data class BrowseResponse(
    val path: String,
    val parent: String,
    val entries: List<FsEntry>,
)

@Serializable
data class FsEntry(
    val name: String,
    val path: String,
    @SerialName("is_dir") val isDir: Boolean,
    @SerialName("has_compose_file") val hasComposeFile: Boolean = false,
)

@Serializable
data class RegisterStackRequest(val path: String)

// --- Error ---

@Serializable
data class ApiError(
    val error: String,
    val code: String,
)

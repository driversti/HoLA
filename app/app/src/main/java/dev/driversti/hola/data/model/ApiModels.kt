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
    @SerialName("temperature_celsius") val temperatureCelsius: Double? = null,
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

@Serializable
data class UpdateComposeRequest(val content: String)

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

// --- Docker Resources ---

@Serializable
data class DiskUsageResponse(
    val images: DiskResourceSummary,
    val volumes: DiskResourceSummary,
    val networks: DiskNetworkSummary,
    @SerialName("build_cache") val buildCache: DiskCacheSummary,
)

@Serializable
data class DiskResourceSummary(
    @SerialName("total_count") val totalCount: Int,
    @SerialName("total_size") val totalSize: Long,
    @SerialName("in_use_count") val inUseCount: Int,
    @SerialName("reclaimable_size") val reclaimableSize: Long,
)

@Serializable
data class DiskNetworkSummary(
    @SerialName("total_count") val totalCount: Int,
    @SerialName("in_use_count") val inUseCount: Int,
    @SerialName("reclaimable_count") val reclaimableCount: Int,
)

@Serializable
data class DiskCacheSummary(
    @SerialName("total_size") val totalSize: Long,
)

@Serializable
data class ImageListResponse(val images: List<DockerImage>)

@Serializable
data class DockerImage(
    val id: String,
    val tags: List<String>,
    val size: Long,
    val created: Long,
    @SerialName("in_use") val inUse: Boolean,
    val containers: List<String>,
)

@Serializable
data class VolumeListResponse(val volumes: List<DockerVolume>)

@Serializable
data class DockerVolume(
    val name: String,
    val driver: String,
    val size: Long,
    val created: String,
    @SerialName("in_use") val inUse: Boolean,
    val containers: List<String>,
)

@Serializable
data class NetworkListResponse(val networks: List<DockerNetwork>)

@Serializable
data class DockerNetwork(
    val id: String,
    val name: String,
    val driver: String,
    val scope: String,
    val internal: Boolean,
    @SerialName("in_use") val inUse: Boolean,
    val containers: List<String>,
    val builtin: Boolean,
)

@Serializable
data class PruneResponse(
    @SerialName("dry_run") val dryRun: Boolean,
    @SerialName("items_to_remove") val itemsToRemove: List<String>,
    val count: Int,
    @SerialName("space_reclaimed") val spaceReclaimed: Long,
)

// --- Error ---

@Serializable
data class ApiError(
    val error: String,
    val code: String,
)

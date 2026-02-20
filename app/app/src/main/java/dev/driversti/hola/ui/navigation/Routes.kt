package dev.driversti.hola.ui.navigation

import kotlinx.serialization.Serializable

@Serializable
object ServerList

@Serializable
data class AddServer(val serverId: String? = null)

@Serializable
data class ServerDetail(val serverId: String)

@Serializable
data class StackDetail(val serverId: String, val stackName: String)

@Serializable
data class ContainerDetail(val serverId: String, val containerId: String)

@Serializable
data class ComposeViewer(val serverId: String, val stackName: String)

@Serializable
object Settings

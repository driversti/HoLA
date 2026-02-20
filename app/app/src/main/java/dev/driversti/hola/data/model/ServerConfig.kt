package dev.driversti.hola.data.model

import kotlinx.serialization.Serializable

@Serializable
data class ServerConfig(
    val id: String,
    val name: String,
    val host: String,
    val port: Int = 8420,
)

package dev.driversti.hola.data.api

import dev.driversti.hola.data.model.ServerConfig

object ApiProvider {

    fun forServer(server: ServerConfig, token: String): HolaApi {
        val baseUrl = "http://${server.host}:${server.port}/"
        return ApiClientFactory.create(baseUrl, token)
    }
}

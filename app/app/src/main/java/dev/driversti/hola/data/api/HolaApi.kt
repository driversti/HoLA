package dev.driversti.hola.data.api

import dev.driversti.hola.data.model.ActionResponse
import dev.driversti.hola.data.model.AgentInfo
import dev.driversti.hola.data.model.BrowseResponse
import dev.driversti.hola.data.model.ComposeFileResponse
import dev.driversti.hola.data.model.ContainerLogsResponse
import dev.driversti.hola.data.model.HealthResponse
import dev.driversti.hola.data.model.RegisterStackRequest
import dev.driversti.hola.data.model.UpdateComposeRequest
import dev.driversti.hola.data.model.StackDetail
import dev.driversti.hola.data.model.StackListResponse
import dev.driversti.hola.data.model.SystemMetrics
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

interface HolaApi {

    @GET("api/v1/health")
    suspend fun health(): HealthResponse

    @GET("api/v1/agent/info")
    suspend fun agentInfo(): AgentInfo

    @GET("api/v1/system/metrics")
    suspend fun systemMetrics(): SystemMetrics

    @GET("api/v1/fs/browse")
    suspend fun browsePath(@Query("path") path: String = "/"): BrowseResponse

    @GET("api/v1/stacks")
    suspend fun listStacks(): StackListResponse

    @GET("api/v1/stacks/{name}")
    suspend fun getStack(@Path("name") name: String): StackDetail

    @GET("api/v1/stacks/{name}/compose")
    suspend fun getComposeFile(@Path("name") name: String): ComposeFileResponse

    @PUT("api/v1/stacks/{name}/compose")
    suspend fun updateComposeFile(
        @Path("name") name: String,
        @Body request: UpdateComposeRequest,
    ): ActionResponse

    @POST("api/v1/stacks/register")
    suspend fun registerStack(@Body request: RegisterStackRequest): ActionResponse

    @DELETE("api/v1/stacks/{name}/unregister")
    suspend fun unregisterStack(@Path("name") name: String): ActionResponse

    @POST("api/v1/stacks/{name}/start")
    suspend fun startStack(@Path("name") name: String): ActionResponse

    @POST("api/v1/stacks/{name}/stop")
    suspend fun stopStack(@Path("name") name: String): ActionResponse

    @POST("api/v1/stacks/{name}/restart")
    suspend fun restartStack(@Path("name") name: String): ActionResponse

    @POST("api/v1/stacks/{name}/down")
    suspend fun downStack(@Path("name") name: String): ActionResponse

    @POST("api/v1/stacks/{name}/pull")
    suspend fun pullStack(@Path("name") name: String): ActionResponse

    @GET("api/v1/containers/{id}/logs")
    suspend fun containerLogs(
        @Path("id") id: String,
        @Query("lines") lines: Int = 100,
        @Query("since") since: String? = null,
    ): ContainerLogsResponse

    @POST("api/v1/containers/{id}/start")
    suspend fun startContainer(@Path("id") id: String): ActionResponse

    @POST("api/v1/containers/{id}/stop")
    suspend fun stopContainer(@Path("id") id: String): ActionResponse

    @POST("api/v1/containers/{id}/restart")
    suspend fun restartContainer(@Path("id") id: String): ActionResponse
}

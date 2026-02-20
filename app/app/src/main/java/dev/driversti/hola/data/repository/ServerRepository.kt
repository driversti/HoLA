package dev.driversti.hola.data.repository

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dev.driversti.hola.data.model.ServerConfig
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

private val Context.serverDataStore: DataStore<Preferences> by preferencesDataStore(name = "servers")

class ServerRepository(private val context: Context) {

    private val serversKey = stringPreferencesKey("server_list")
    private val json = Json { ignoreUnknownKeys = true }

    val servers: Flow<List<ServerConfig>> = context.serverDataStore.data.map { prefs ->
        val raw = prefs[serversKey] ?: "[]"
        json.decodeFromString<List<ServerConfig>>(raw)
    }

    suspend fun addServer(server: ServerConfig) {
        context.serverDataStore.edit { prefs ->
            val current = prefs[serversKey]?.let {
                json.decodeFromString<List<ServerConfig>>(it)
            } ?: emptyList()
            prefs[serversKey] = json.encodeToString(current + server)
        }
    }

    suspend fun updateServer(server: ServerConfig) {
        context.serverDataStore.edit { prefs ->
            val current = prefs[serversKey]?.let {
                json.decodeFromString<List<ServerConfig>>(it)
            } ?: emptyList()
            val updated = current.map { if (it.id == server.id) server else it }
            prefs[serversKey] = json.encodeToString(updated)
        }
    }

    suspend fun deleteServer(serverId: String) {
        context.serverDataStore.edit { prefs ->
            val current = prefs[serversKey]?.let {
                json.decodeFromString<List<ServerConfig>>(it)
            } ?: emptyList()
            prefs[serversKey] = json.encodeToString(current.filter { it.id != serverId })
        }
    }
}

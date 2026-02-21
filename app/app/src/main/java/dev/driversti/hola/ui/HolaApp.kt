package dev.driversti.hola.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.navigation.compose.rememberNavController
import dev.driversti.hola.data.api.WebSocketManager
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.SettingsRepository
import dev.driversti.hola.data.repository.ThemeMode
import dev.driversti.hola.data.repository.TokenRepository
import dev.driversti.hola.ui.navigation.HolaNavGraph
import dev.driversti.hola.ui.theme.HolaTheme
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

@Composable
fun HolaApp(lifecycle: Lifecycle) {
    val context = LocalContext.current
    val serverRepository = remember { ServerRepository(context) }
    val tokenRepository = remember { TokenRepository(context) }
    val settingsRepository = remember { SettingsRepository(context) }
    val webSocketManager = remember { WebSocketManager(tokenRepository) }

    val themeMode by settingsRepository.themeMode.collectAsState(initial = ThemeMode.SYSTEM)

    val scope = rememberCoroutineScope()

    DisposableEffect(lifecycle) {
        val observer = LifecycleEventObserver { _, event ->
            when (event) {
                Lifecycle.Event.ON_START -> {
                    scope.launch {
                        val servers = serverRepository.servers.first()
                        webSocketManager.onAppForeground(servers)
                    }
                }
                Lifecycle.Event.ON_STOP -> {
                    webSocketManager.onAppBackground()
                }
                else -> {}
            }
        }
        lifecycle.addObserver(observer)
        onDispose { lifecycle.removeObserver(observer) }
    }

    HolaTheme(themeMode = themeMode) {
        val navController = rememberNavController()
        HolaNavGraph(
            navController = navController,
            serverRepository = serverRepository,
            tokenRepository = tokenRepository,
            settingsRepository = settingsRepository,
            webSocketManager = webSocketManager,
        )
    }
}

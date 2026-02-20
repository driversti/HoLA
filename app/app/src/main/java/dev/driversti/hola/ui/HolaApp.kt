package dev.driversti.hola.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
import androidx.navigation.compose.rememberNavController
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.SettingsRepository
import dev.driversti.hola.data.repository.ThemeMode
import dev.driversti.hola.data.repository.TokenRepository
import dev.driversti.hola.ui.navigation.HolaNavGraph
import dev.driversti.hola.ui.theme.HolaTheme

@Composable
fun HolaApp() {
    val context = LocalContext.current
    val serverRepository = remember { ServerRepository(context) }
    val tokenRepository = remember { TokenRepository(context) }
    val settingsRepository = remember { SettingsRepository(context) }

    val themeMode by settingsRepository.themeMode.collectAsState(initial = ThemeMode.SYSTEM)

    HolaTheme(themeMode = themeMode) {
        val navController = rememberNavController()
        HolaNavGraph(
            navController = navController,
            serverRepository = serverRepository,
            tokenRepository = tokenRepository,
            settingsRepository = settingsRepository,
        )
    }
}

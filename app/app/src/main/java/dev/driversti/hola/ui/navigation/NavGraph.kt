package dev.driversti.hola.ui.navigation

import androidx.compose.runtime.Composable
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import dev.driversti.hola.data.api.WebSocketManager
import dev.driversti.hola.data.repository.ServerRepository
import dev.driversti.hola.data.repository.SettingsRepository
import dev.driversti.hola.data.repository.TokenRepository
import dev.driversti.hola.ui.screens.addserver.AddServerScreen
import dev.driversti.hola.ui.screens.composeviewer.ComposeViewerScreen
import dev.driversti.hola.ui.screens.containerdetail.ContainerDetailScreen
import dev.driversti.hola.ui.screens.filebrowser.FileBrowserScreen
import dev.driversti.hola.ui.screens.resources.ResourceListScreen
import dev.driversti.hola.ui.screens.resources.ResourcesDashboardScreen
import dev.driversti.hola.ui.screens.serverdetail.ServerDetailScreen
import dev.driversti.hola.ui.screens.serverlist.ServerListScreen
import dev.driversti.hola.ui.screens.settings.SettingsScreen
import dev.driversti.hola.ui.screens.stackdetail.StackDetailScreen

@Composable
fun HolaNavGraph(
    navController: NavHostController,
    serverRepository: ServerRepository,
    tokenRepository: TokenRepository,
    settingsRepository: SettingsRepository,
    webSocketManager: WebSocketManager,
) {
    NavHost(navController = navController, startDestination = ServerList) {
        composable<ServerList> {
            ServerListScreen(
                serverRepository = serverRepository,
                tokenRepository = tokenRepository,
                webSocketManager = webSocketManager,
                onServerClick = { serverId ->
                    navController.navigate(ServerDetail(serverId))
                },
                onAddServer = { navController.navigate(AddServer()) },
                onSettings = { navController.navigate(Settings) },
            )
        }

        composable<AddServer> { backStack ->
            val route = backStack.toRoute<AddServer>()
            AddServerScreen(
                serverId = route.serverId,
                serverRepository = serverRepository,
                tokenRepository = tokenRepository,
                onBack = { navController.popBackStack() },
            )
        }

        composable<ServerDetail> { backStack ->
            val route = backStack.toRoute<ServerDetail>()
            ServerDetailScreen(
                serverId = route.serverId,
                serverRepository = serverRepository,
                tokenRepository = tokenRepository,
                webSocketManager = webSocketManager,
                onStackClick = { stackName ->
                    navController.navigate(StackDetail(route.serverId, stackName))
                },
                onAddStack = {
                    navController.navigate(FileBrowser(route.serverId))
                },
                onResources = {
                    navController.navigate(ResourcesDashboard(route.serverId))
                },
                onBack = { navController.popBackStack() },
            )
        }

        composable<FileBrowser> { backStack ->
            val route = backStack.toRoute<FileBrowser>()
            FileBrowserScreen(
                serverId = route.serverId,
                serverRepository = serverRepository,
                tokenRepository = tokenRepository,
                onBack = { navController.popBackStack() },
            )
        }

        composable<StackDetail> { backStack ->
            val route = backStack.toRoute<StackDetail>()
            StackDetailScreen(
                serverId = route.serverId,
                stackName = route.stackName,
                serverRepository = serverRepository,
                tokenRepository = tokenRepository,
                webSocketManager = webSocketManager,
                onContainerClick = { containerId ->
                    navController.navigate(ContainerDetail(route.serverId, containerId))
                },
                onViewCompose = {
                    navController.navigate(ComposeViewer(route.serverId, route.stackName))
                },
                onBack = { navController.popBackStack() },
            )
        }

        composable<ContainerDetail> { backStack ->
            val route = backStack.toRoute<ContainerDetail>()
            ContainerDetailScreen(
                serverId = route.serverId,
                containerId = route.containerId,
                serverRepository = serverRepository,
                tokenRepository = tokenRepository,
                webSocketManager = webSocketManager,
                onBack = { navController.popBackStack() },
            )
        }

        composable<ComposeViewer> { backStack ->
            val route = backStack.toRoute<ComposeViewer>()
            ComposeViewerScreen(
                serverId = route.serverId,
                stackName = route.stackName,
                serverRepository = serverRepository,
                tokenRepository = tokenRepository,
                onBack = { navController.popBackStack() },
            )
        }

        composable<ResourcesDashboard> { backStack ->
            val route = backStack.toRoute<ResourcesDashboard>()
            ResourcesDashboardScreen(
                serverId = route.serverId,
                serverRepository = serverRepository,
                tokenRepository = tokenRepository,
                onResourceClick = { resourceType ->
                    navController.navigate(ResourceList(route.serverId, resourceType))
                },
                onBack = { navController.popBackStack() },
            )
        }

        composable<ResourceList> { backStack ->
            val route = backStack.toRoute<ResourceList>()
            ResourceListScreen(
                serverId = route.serverId,
                resourceType = route.resourceType,
                serverRepository = serverRepository,
                tokenRepository = tokenRepository,
                onBack = { navController.popBackStack() },
            )
        }

        composable<Settings> {
            SettingsScreen(
                tokenRepository = tokenRepository,
                settingsRepository = settingsRepository,
                onBack = { navController.popBackStack() },
            )
        }
    }
}

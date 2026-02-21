package api

import (
	"net/http"

	"github.com/driversti/hola/internal/auth"
	"github.com/driversti/hola/internal/docker"
	"github.com/driversti/hola/internal/registry"
	"github.com/driversti/hola/internal/update"
	"github.com/driversti/hola/internal/ws"
)

// NewRouter creates the HTTP router with all API routes.
func NewRouter(version string, authMw *auth.Middleware, dockerClient *docker.Client, wsHandler *ws.Handler, registryStore *registry.Store, updater *update.Updater) http.Handler {
	mux := http.NewServeMux()

	h := &handlers{version: version, docker: dockerClient, registry: registryStore, updater: updater}

	// System
	mux.HandleFunc("GET /api/v1/health", h.health)
	mux.HandleFunc("GET /api/v1/agent/info", h.agentInfo)
	mux.HandleFunc("GET /api/v1/system/metrics", h.systemMetrics)
	mux.HandleFunc("GET /api/v1/agent/update", h.checkUpdate)
	mux.HandleFunc("POST /api/v1/agent/update", h.applyUpdate)

	// Filesystem browse
	mux.HandleFunc("GET /api/v1/fs/browse", h.browsePath)

	// Stacks — read
	mux.HandleFunc("GET /api/v1/stacks", h.listStacks)
	mux.HandleFunc("GET /api/v1/stacks/{name}", h.getStack)
	mux.HandleFunc("GET /api/v1/stacks/{name}/compose", h.getComposeFile)

	// Stacks — write
	mux.HandleFunc("PUT /api/v1/stacks/{name}/compose", h.updateComposeFile)
	mux.HandleFunc("POST /api/v1/stacks/register", h.registerStack)
	mux.HandleFunc("POST /api/v1/stacks/{name}/start", h.stackAction)
	mux.HandleFunc("POST /api/v1/stacks/{name}/stop", h.stackAction)
	mux.HandleFunc("POST /api/v1/stacks/{name}/restart", h.stackAction)
	mux.HandleFunc("POST /api/v1/stacks/{name}/down", h.stackAction)
	mux.HandleFunc("POST /api/v1/stacks/{name}/pull", h.stackAction)
	mux.HandleFunc("DELETE /api/v1/stacks/{name}/unregister", h.unregisterStack)

	// Containers
	mux.HandleFunc("GET /api/v1/containers/{id}/logs", h.containerLogs)
	mux.HandleFunc("POST /api/v1/containers/{id}/start", h.containerAction)
	mux.HandleFunc("POST /api/v1/containers/{id}/stop", h.containerAction)
	mux.HandleFunc("POST /api/v1/containers/{id}/restart", h.containerAction)

	// Docker resources
	mux.HandleFunc("GET /api/v1/docker/disk-usage", h.dockerDiskUsage)
	mux.HandleFunc("GET /api/v1/docker/images", h.listImages)
	mux.HandleFunc("DELETE /api/v1/docker/images/{id}", h.removeImage)
	mux.HandleFunc("POST /api/v1/docker/images/prune", h.pruneImages)
	mux.HandleFunc("GET /api/v1/docker/volumes", h.listVolumes)
	mux.HandleFunc("DELETE /api/v1/docker/volumes/{name}", h.removeVolume)
	mux.HandleFunc("POST /api/v1/docker/volumes/prune", h.pruneVolumes)
	mux.HandleFunc("GET /api/v1/docker/networks", h.listNetworks)
	mux.HandleFunc("DELETE /api/v1/docker/networks/{id}", h.removeNetwork)
	mux.HandleFunc("POST /api/v1/docker/networks/prune", h.pruneNetworks)
	mux.HandleFunc("POST /api/v1/docker/buildcache/prune", h.pruneBuildCache)

	// WebSocket
	mux.Handle("GET /api/v1/ws", wsHandler)

	return loggingMiddleware(authMw.Wrap(mux))
}

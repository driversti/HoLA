package api

import (
	"net/http"

	"github.com/driversti/hola/internal/auth"
	"github.com/driversti/hola/internal/docker"
	"github.com/driversti/hola/internal/registry"
	"github.com/driversti/hola/internal/ws"
)

// NewRouter creates the HTTP router with all API routes.
func NewRouter(version string, authMw *auth.Middleware, dockerClient *docker.Client, wsHandler *ws.Handler, registryStore *registry.Store) http.Handler {
	mux := http.NewServeMux()

	h := &handlers{version: version, docker: dockerClient, registry: registryStore}

	// System
	mux.HandleFunc("GET /api/v1/health", h.health)
	mux.HandleFunc("GET /api/v1/agent/info", h.agentInfo)
	mux.HandleFunc("GET /api/v1/system/metrics", h.systemMetrics)

	// Filesystem browse
	mux.HandleFunc("GET /api/v1/fs/browse", h.browsePath)

	// Stacks — read
	mux.HandleFunc("GET /api/v1/stacks", h.listStacks)
	mux.HandleFunc("GET /api/v1/stacks/{name}", h.getStack)
	mux.HandleFunc("GET /api/v1/stacks/{name}/compose", h.getComposeFile)

	// Stacks — write
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

	// WebSocket
	mux.Handle("GET /api/v1/ws", wsHandler)

	return loggingMiddleware(authMw.Wrap(mux))
}

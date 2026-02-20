# HoLA — Homelab Agent

## Overview

HoLA is a lightweight homelab management system consisting of a **Go-based agent** running on each server and a **native Android app** for monitoring and controlling Docker Compose stacks on the go.

**Problem**: Managing a homelab requires SSH access or a browser-based tool like Portainer. When away from a computer, there's no convenient way to check server health, restart a crashed stack, or pull an updated image.

**Solution**: A minimal, stateless Go agent on each server exposes system metrics and Docker operations via REST API. A native Android app connects to agents over the local network or Tailscale, providing quick server health checks and Docker management with a clean Material 3 interface.

---

## Architecture

```
┌─────────────────┐        REST/JSON         ┌──────────────────┐
│   Android App    │ ◄─────────────────────► │   HoLA Agent     │
│  (Kotlin/Compose)│    Tailscale / LAN       │   (Go binary)    │
│                  │                          │                  │
│  - Server list   │                          │  - System metrics│
│  - Stack mgmt    │                          │  - Docker API    │
│  - Log viewer    │                          │  - Log streaming │
└─────────────────┘                          └──────────────────┘
        │                                            │
        │                                     ┌──────┴──────┐
        │                                     │ Docker sock  │
        │                                     │ /var/run/    │
        └─ Tailscale mesh network ────────────│ docker.sock  │
                                              └─────────────┘
```

### Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Agent state | Stateless | No database, no history. Agent is a thin proxy over Docker API + system calls. Easy to deploy, nothing to back up. |
| Connectivity | Tailscale + LAN | Already in use. No port exposure, no relay infra needed. App connects via Tailscale IP when remote, LAN IP when home. |
| Auth | Global pre-shared token | Single user, personal homelab. One token for all agents. Tailscale provides network-level auth already. |
| Container runtime | Docker Compose only | All services managed via compose files. No Swarm, no Podman, no standalone containers. |
| API protocol | REST (JSON) | Simple, debuggable with curl. WebSocket reserved for future real-time features. |
| Notifications | None (pull-only) | Open app to check. No push infra, no FCM, no background services. |

---

## Go Agent (`/agent`)

### Overview

Single static Go binary, managed by systemd. Runs as a non-root user in the `docker` group. Listens on a fixed port.

- **Port**: `8420` (fixed, not configurable in v1)
- **Auth**: `Authorization: Bearer <token>` header on all requests
- **Transport**: HTTP (TLS handled by Tailscale's encrypted tunnels)
- **Docker access**: Unix socket `/var/run/docker.sock` via docker group membership
- **Config**: Minimal — token set via environment variable (`HOLA_TOKEN`) or CLI flag (`--token`)

### Stack Discovery

The agent auto-discovers Docker Compose stacks by querying the Docker API for running containers and grouping them by the `com.docker.compose.project` label. This means:
- Only **running or recently stopped** stacks are visible
- Stacks that have never been started (or were `docker compose down`'d) are **not** discoverable
- The compose file path is extracted from the `com.docker.compose.project.working_dir` label

### API Endpoints

#### System Metrics

```
GET /api/v1/system/metrics
```

Returns current system metrics snapshot:
```json
{
  "hostname": "nas-01",
  "uptime_seconds": 1234567,
  "cpu": {
    "usage_percent": 23.5,
    "cores": 8
  },
  "memory": {
    "total_bytes": 34359738368,
    "used_bytes": 12884901888,
    "usage_percent": 37.5
  },
  "disk": [
    {
      "mount_point": "/",
      "total_bytes": 500107862016,
      "used_bytes": 234554432512,
      "usage_percent": 46.9
    }
  ]
}
```

#### Agent Info

```
GET /api/v1/agent/info
```

Returns agent version and server identity:
```json
{
  "version": "0.1.0",
  "hostname": "nas-01",
  "os": "linux",
  "arch": "amd64",
  "docker_version": "24.0.7"
}
```

#### Health Check

```
GET /api/v1/health
```

Unauthenticated. Returns `200 OK` with `{"status": "ok"}`. Used by the app to check if an agent is reachable.

#### Compose Stacks

```
GET /api/v1/stacks
```

List all discovered compose stacks:
```json
{
  "stacks": [
    {
      "name": "media-stack",
      "status": "running",
      "service_count": 4,
      "running_count": 4,
      "working_dir": "/opt/stacks/media"
    },
    {
      "name": "monitoring",
      "status": "partial",
      "service_count": 3,
      "running_count": 2,
      "working_dir": "/opt/stacks/monitoring"
    }
  ]
}
```

Stack status values: `running` (all services up), `stopped` (all down), `partial` (some up, some down).

```
GET /api/v1/stacks/{name}
```

Get detailed stack info including individual containers:
```json
{
  "name": "media-stack",
  "status": "running",
  "working_dir": "/opt/stacks/media",
  "containers": [
    {
      "id": "abc123",
      "name": "media-stack-plex-1",
      "service": "plex",
      "image": "plexinc/pms-docker:latest",
      "status": "running",
      "state": "running",
      "created_at": "2024-01-15T10:30:00Z",
      "started_at": "2024-01-15T10:30:05Z"
    }
  ]
}
```

```
GET /api/v1/stacks/{name}/compose
```

Returns the raw compose file content (read-only, for reference):
```json
{
  "content": "version: '3.8'\nservices:\n  plex:\n    image: ...",
  "path": "/opt/stacks/media/docker-compose.yml"
}
```

#### Stack Operations

```
POST /api/v1/stacks/{name}/start
POST /api/v1/stacks/{name}/stop
POST /api/v1/stacks/{name}/restart
POST /api/v1/stacks/{name}/down
POST /api/v1/stacks/{name}/pull
```

- `start` → `docker compose up -d` (in the stack's working directory)
- `stop` → `docker compose stop` (stops containers, keeps them)
- `restart` → `docker compose restart`
- `down` → `docker compose down` (stops and removes containers)
- `pull` → `docker compose pull` (pulls latest images, does NOT restart)

All operations return:
```json
{
  "success": true,
  "message": "Stack 'media-stack' stopped successfully"
}
```

On error:
```json
{
  "success": false,
  "error": "failed to stop stack: container plex is in use by..."
}
```

#### Container Operations

```
POST /api/v1/containers/{id}/start
POST /api/v1/containers/{id}/stop
POST /api/v1/containers/{id}/restart
```

Individual container operations (within a compose stack).

#### Container Logs

```
GET /api/v1/containers/{id}/logs?lines=100&since=2024-01-15T10:00:00Z
```

Returns the last N lines of container logs:
```json
{
  "container_id": "abc123",
  "container_name": "media-stack-plex-1",
  "lines": [
    {"timestamp": "2024-01-15T10:30:05Z", "stream": "stdout", "message": "Starting Plex Media Server..."},
    {"timestamp": "2024-01-15T10:30:06Z", "stream": "stderr", "message": "Warning: ..."}
  ]
}
```

Query parameters:
- `lines` — number of lines to return (default: 100, max: 1000)
- `since` — only return logs after this timestamp (ISO 8601)

### Error Handling

All error responses follow:
```json
{
  "error": "human-readable error message",
  "code": "STACK_NOT_FOUND"
}
```

HTTP status codes:
- `400` — bad request (invalid parameters)
- `401` — missing or invalid token
- `404` — stack or container not found
- `500` — internal error (Docker API failure, etc.)
- `503` — Docker daemon unavailable

### Deployment

1. Build: `go build -o hola-agent ./cmd/agent`
2. Copy binary to server
3. Create systemd unit:

```ini
[Unit]
Description=HoLA Agent
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=hola
Group=docker
Environment=HOLA_TOKEN=your-secret-token-here
ExecStart=/usr/local/bin/hola-agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

4. `systemctl enable --now hola-agent`

### Self-Update (v2, deferred)

Future: agent checks GitHub Releases for new versions, downloads and replaces itself. Not in v1 — deploy manually.

---

## Android App (`/app`)

### Overview

Native Android app built with **Kotlin** and **Jetpack Compose**. Material 3 (Material You) design with dynamic colors.

- **Min SDK**: Android 16 (API level 36)
- **UI framework**: Jetpack Compose with Material 3
- **Navigation**: Single-activity, Compose Navigation
- **HTTP client**: Ktor or OkHttp/Retrofit
- **Architecture**: MVVM with Kotlin coroutines and Flow

### Screens

#### 1. Server List (Home)

The main screen. Shows all configured servers with their status.

```
┌─────────────────────────────┐
│  HoLA                    ⚙️ │
│─────────────────────────────│
│  ┌─────────────────────────┐│
│  │ 🟢 nas-01              ││
│  │    CPU 23%  RAM 37%     ││
│  │    4 stacks running     ││
│  └─────────────────────────┘│
│  ┌─────────────────────────┐│
│  │ 🔴 rpi-cluster          ││
│  │    Offline              ││
│  └─────────────────────────┘│
│  ┌─────────────────────────┐│
│  │ 🟢 media-server         ││
│  │    CPU 5%   RAM 62%     ││
│  │    2 stacks running     ││
│  └─────────────────────────┘│
│                             │
│              ＋ Add Server  │
└─────────────────────────────┘
```

- Pull-to-refresh to reload all servers
- Color-coded status indicators (green = online, red = offline)
- Quick stats: CPU, RAM, stack count
- Offline servers show badge only (no stale data)
- Tap server → Server Detail screen
- Settings gear → Settings screen

#### 2. Server Detail

Shows stacks on the selected server.

```
┌─────────────────────────────┐
│ ← nas-01                   │
│   CPU 23%  RAM 37%  Disk 47%│
│─────────────────────────────│
│  Stacks                     │
│  ┌─────────────────────────┐│
│  │ 🟢 media-stack    4/4   ││
│  └─────────────────────────┘│
│  ┌─────────────────────────┐│
│  │ 🟡 monitoring     2/3   ││
│  └─────────────────────────┘│
│  ┌─────────────────────────┐│
│  │ 🟢 reverse-proxy  1/1   ││
│  └─────────────────────────┘│
│  ┌─────────────────────────┐│
│  │ 🔴 gameserver     0/2   ││
│  └─────────────────────────┘│
└─────────────────────────────┘
```

- Shows system metrics at the top (CPU, RAM, disk)
- Stack list with status indicators:
  - 🟢 Green: all services running (`4/4`)
  - 🟡 Yellow: partial (`2/3`)
  - 🔴 Red: all stopped (`0/2`)
- Tap stack → Stack Detail screen

#### 3. Stack Detail

Shows containers in a stack with available actions.

```
┌─────────────────────────────┐
│ ← media-stack          •••  │
│─────────────────────────────│
│  [Start] [Stop] [Down] [Pull]│
│─────────────────────────────│
│  Containers                 │
│  ┌─────────────────────────┐│
│  │ 🟢 plex                 ││
│  │    plexinc/pms:latest   ││
│  └─────────────────────────┘│
│  ┌─────────────────────────┐│
│  │ 🟢 sonarr               ││
│  │    linuxserver/sonarr   ││
│  └─────────────────────────┘│
│  ┌─────────────────────────┐│
│  │ 🟢 radarr               ││
│  │    linuxserver/radarr   ││
│  └─────────────────────────┘│
│                             │
│  📄 View Compose File       │
└─────────────────────────────┘
```

- Stack-level action buttons at the top
- **Stop** and **Down** require biometric confirmation
- **Pull** requires simple confirmation dialog
- **Start** and **Restart** — no confirmation needed
- Container list with individual status
- Tap container → Container Detail
- Overflow menu (`•••`) → Restart
- "View Compose File" → read-only YAML viewer

#### 4. Container Detail

Individual container info and logs.

```
┌─────────────────────────────┐
│ ← plex                     │
│─────────────────────────────│
│  Image: plexinc/pms:latest │
│  Status: Running            │
│  Started: 2h 15m ago        │
│─────────────────────────────│
│  [Stop] [Restart]           │
│─────────────────────────────│
│  Logs (last 100 lines)   🔄│
│  ┌─────────────────────────┐│
│  │ 10:30:05 Starting...    ││
│  │ 10:30:06 Listening on   ││
│  │          port 32400     ││
│  │ 10:31:00 Library scan   ││
│  │          complete       ││
│  │ ...                     ││
│  └─────────────────────────┘│
└─────────────────────────────┘
```

- Container metadata (image, status, uptime)
- Individual container actions (stop/restart) with biometric confirmation
- Log viewer: shows last 100 lines, scrollable
- Refresh button (🔄) to re-fetch logs
- Monospace font for logs

#### 5. Compose File Viewer

Read-only YAML display with syntax highlighting.

#### 6. Add/Edit Server

```
┌─────────────────────────────┐
│ ← Add Server                │
│─────────────────────────────│
│  Name                       │
│  ┌─────────────────────────┐│
│  │ nas-01                  ││
│  └─────────────────────────┘│
│                             │
│  Host                       │
│  ┌─────────────────────────┐│
│  │ 100.64.0.5              ││
│  └─────────────────────────┘│
│                             │
│  Port                       │
│  ┌─────────────────────────┐│
│  │ 8420                    ││
│  └─────────────────────────┘│
│                             │
│  [Test Connection]          │
│                             │
│           [Save]            │
└─────────────────────────────┘
```

- Name, host (IP or hostname), port (default 8420)
- **Token is NOT entered per-server** — it's a global setting
- "Test Connection" button hits `/api/v1/health` then `/api/v1/agent/info` with the token
- Shows success/failure with server info

#### 7. Settings

```
┌─────────────────────────────┐
│ ← Settings                  │
│─────────────────────────────│
│  API Token                  │
│  ┌─────────────────────────┐│
│  │ ••••••••••••••••        ││
│  └─────────────────────────┘│
│                             │
│  Theme                      │
│  [System] [Light] [Dark]    │
│                             │
│  About                      │
│  Version 0.1.0              │
└─────────────────────────────┘
```

- Global API token (masked, tap to reveal/edit)
- Theme selection (follows system by default)
- App version info

### Safety & Confirmation

**Biometric + confirmation required for**:
- Stack: Stop, Down, Restart
- Container: Stop, Restart

**Simple confirmation dialog for**:
- Stack: Pull (shows "This will pull latest images for all services in {stack}. Continue?")

**No confirmation for**:
- Stack: Start (non-destructive, brings things up)
- Navigation, viewing, refreshing

### Data Storage (App-side)

- Server list stored in **DataStore** (Jetpack)
- Token stored in **EncryptedSharedPreferences**
- No caching of server state — always fetched fresh

### Offline / Error Handling

- If agent is unreachable: server shows "Offline" badge with red indicator
- No stale/cached data displayed for offline servers
- Network errors show a snackbar with retry action
- Loading states shown with Material 3 progress indicators

---

## Repository Structure

```
HoLA/
├── agent/                  # Go agent
│   ├── cmd/
│   │   └── agent/
│   │       └── main.go
│   ├── internal/
│   │   ├── api/            # HTTP handlers
│   │   ├── docker/         # Docker client wrapper
│   │   ├── metrics/        # System metrics collection
│   │   └── auth/           # Token authentication middleware
│   ├── go.mod
│   └── go.sum
├── app/                    # Android app
│   ├── app/
│   │   └── src/
│   │       └── main/
│   │           ├── java/   # Kotlin source
│   │           └── res/    # Resources
│   ├── build.gradle.kts
│   └── settings.gradle.kts
├── SPEC.md                 # This file
└── README.md
```

---

## Security Considerations

1. **Transport**: Tailscale provides WireGuard-encrypted tunnels. Agent serves plain HTTP — encryption is handled at the network layer.
2. **Authentication**: Bearer token in Authorization header. Single global token for simplicity.
3. **Token storage**: On agent — environment variable or CLI flag. On app — Android EncryptedSharedPreferences (hardware-backed keystore).
4. **Docker socket**: Agent runs as non-root user in `docker` group. Note: docker group membership is effectively equivalent to root access on the host.
5. **No external exposure**: Agent only listens on Tailscale interface or `0.0.0.0` (but only reachable via Tailscale from outside LAN).
6. **Health endpoint**: `/api/v1/health` is the only unauthenticated endpoint. Returns no sensitive data.

---

## Scope & Phasing

### v1 (MVP)

- Go agent with all API endpoints above
- Android app with all screens above
- System metrics (CPU, RAM, disk)
- Stack discovery, listing, start/stop/down/restart/pull
- Container listing, start/stop/restart
- Container log tailing (polling, last N lines)
- Compose file viewer (read-only)
- Biometric confirmation for destructive ops
- Manual server configuration
- Single global auth token

### v2 (Future)

- Agent self-update (GitHub Releases)
- WebSocket-based real-time log streaming
- Push notifications (container crashes, server offline)
- Per-container resource usage metrics
- Network metrics
- Stack health checks and restart counts
- Per-server tokens (optional)
- Multiple environments/homelabs
- mDNS auto-discovery on LAN
- QR code server onboarding

---

## Non-Goals (Explicitly Out of Scope)

- Compose file editing from the app
- Container creation (outside of compose)
- Image management (building, tagging)
- Docker volume/network management
- User management / multi-user access
- Swarm/Kubernetes orchestration
- Alerting / monitoring history
- CI/CD integration

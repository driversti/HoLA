# HoLA — Homelab Agent

Manage your homelab Docker Compose stacks from your phone.

## Overview

HoLA is a lightweight homelab management system consisting of a **Go agent** running on each server and a **native Android app** for monitoring and controlling Docker Compose stacks on the go.

**Problem:** Managing a homelab requires SSH access or browser-based tools like Portainer. When away from a computer, there's no convenient way to check server health, restart a crashed stack, or pull an updated image.

**Solution:** A minimal, stateless Go agent on each server exposes system metrics and Docker operations via REST + WebSocket API. A native Android app connects to agents over the local network or Tailscale, providing quick health checks and Docker management with a Material 3 interface.

## Architecture

```
┌──────────────────┐       REST / WebSocket        ┌──────────────────┐
│   Android App    │ ◄────────────────────────────► │   HoLA Agent     │
│ (Kotlin/Compose) │      Tailscale / LAN          │   (Go binary)    │
│                  │                                │                  │
│ • Server list    │                                │ • System metrics │
│ • Stack mgmt     │                                │ • Docker API     │
│ • Log viewer     │                                │ • Log streaming  │
│ • File browser   │                                │ • File browser   │
└──────────────────┘                                └──────────────────┘
                                                           │
                                                    ┌──────┴──────┐
                                                    │ Docker sock  │
             Tailscale mesh (WireGuard) ────────────│ /var/run/    │
                                                    │ docker.sock  │
                                                    └─────────────┘
```

## Features

### Go Agent

- **System metrics** — CPU, memory, disk usage, uptime
- **Stack discovery** — auto-discovers running Docker Compose stacks
- **Stack registry** — manually register stacks that aren't running yet
- **Stack operations** — start, stop, restart, down, pull
- **Container operations** — start, stop, restart individual containers
- **Container logs** — fetch last N lines with timestamp filtering
- **File browser** — browse server filesystem with compose file detection
- **WebSocket** — real-time metrics, Docker events, and log streaming
- **Auth** — Bearer token authentication on all endpoints (except health check)

### Android App

- **Server list** — manage multiple servers with live status indicators
- **System dashboard** — CPU, RAM, disk at a glance
- **Stack management** — view, start, stop, restart, pull with status badges
- **Container details** — individual container info and log viewer
- **Compose viewer** — read-only YAML display
- **Biometric confirmation** — fingerprint/face required for destructive operations
- **Material You** — dynamic colors, light/dark theme support

## Project Structure

```
HoLA/
├── agent/                     # Go agent
│   ├── cmd/agent/main.go      # Entry point
│   ├── internal/
│   │   ├── api/               # HTTP handlers & router
│   │   ├── auth/              # Token auth middleware
│   │   ├── docker/            # Docker client wrapper
│   │   ├── metrics/           # System metrics (gopsutil)
│   │   ├── registry/          # Stack registry store
│   │   └── ws/                # WebSocket hub & streams
│   ├── go.mod
│   └── hola-agent.service     # Systemd unit file
├── app/                       # Android app
│   ├── app/src/main/java/dev/driversti/hola/
│   │   ├── ui/screens/        # Compose UI screens
│   │   ├── ui/navigation/     # Navigation graph
│   │   ├── data/              # API client, models
│   │   └── BiometricHelper.kt
│   ├── build.gradle.kts
│   └── gradle/libs.versions.toml
├── SPEC.md                    # Detailed specification
└── README.md
```

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.25+ | Agent build |
| Android SDK | API 36 | App build (Android 16) |
| Docker + Compose | Latest | Required on managed servers |

## Building

### Go Agent

```bash
cd agent

# Build for current platform
go build -o hola-agent ./cmd/agent

# Cross-compile for Linux (typical server target)
GOOS=linux GOARCH=amd64 go build -o hola-agent-linux ./cmd/agent
```

### Android App

```bash
cd app

# Debug build
./gradlew assembleDebug

# Release build
./gradlew assembleRelease
```

The APK will be in `app/app/build/outputs/apk/`.

## Deployment

### 1. Create a dedicated user

```bash
sudo useradd -r -s /bin/false -G docker hola
```

### 2. Copy the binary

```bash
sudo cp hola-agent /usr/local/bin/hola-agent
```

### 3. Install the systemd service

```bash
sudo cp hola-agent.service /etc/systemd/system/
```

Edit the service file to set your token:

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

### 4. Start the agent

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now hola-agent
```

The agent listens on port **8420**.

### 5. Verify

```bash
curl http://localhost:8420/api/v1/health
# {"status":"ok"}
```

## API Overview

All endpoints require `Authorization: Bearer <token>` unless noted otherwise.

### System

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/health` | Health check *(no auth)* |
| `GET` | `/api/v1/agent/info` | Agent version, hostname, OS, arch, Docker version |
| `GET` | `/api/v1/system/metrics` | CPU, memory, disk usage, uptime |

### Stacks

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/stacks` | List all discovered + registered stacks |
| `GET` | `/api/v1/stacks/{name}` | Stack details with containers |
| `GET` | `/api/v1/stacks/{name}/compose` | Raw compose file content |
| `POST` | `/api/v1/stacks/register` | Register a stack by path |
| `DELETE` | `/api/v1/stacks/{name}/unregister` | Unregister a stack |
| `POST` | `/api/v1/stacks/{name}/start` | `docker compose up -d` |
| `POST` | `/api/v1/stacks/{name}/stop` | `docker compose stop` |
| `POST` | `/api/v1/stacks/{name}/restart` | `docker compose restart` |
| `POST` | `/api/v1/stacks/{name}/down` | `docker compose down` |
| `POST` | `/api/v1/stacks/{name}/pull` | `docker compose pull` |

### Containers

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/containers/{id}/logs` | Container logs (`?lines=100&since=<ISO8601>`) |
| `POST` | `/api/v1/containers/{id}/start` | Start container |
| `POST` | `/api/v1/containers/{id}/stop` | Stop container |
| `POST` | `/api/v1/containers/{id}/restart` | Restart container |

### Filesystem

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/fs/browse` | Browse directory (`?path=/`) with compose detection |

### WebSocket

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/ws` | WebSocket connection for real-time updates |

**Available streams:**

- **`metrics`** — system metrics at a configurable interval
- **`events`** — real-time Docker container events (start, stop, die, etc.)
- **`logs`** — live container log streaming (max 3 concurrent per client)

Subscribe by sending:

```json
{"type": "subscribe", "payload": {"stream": "metrics", "interval_seconds": 5}}
{"type": "subscribe", "payload": {"stream": "events"}}
{"type": "subscribe", "payload": {"stream": "logs", "container_id": "abc123"}}
```

## Security

- **Transport:** Tailscale provides WireGuard-encrypted tunnels. The agent serves plain HTTP — encryption is handled at the network layer.
- **Authentication:** Bearer token in `Authorization` header. Single global token shared across all agents.
- **Token storage:** Agent side — environment variable or `--token` CLI flag. App side — Android EncryptedSharedPreferences (hardware-backed keystore).
- **Docker socket:** Agent runs as non-root user in the `docker` group. Note: docker group membership is effectively equivalent to root access on the host.
- **Biometric confirmation:** The Android app requires fingerprint or face authentication for destructive operations (stop, down, restart).
- **Health endpoint:** `/api/v1/health` is the only unauthenticated endpoint and returns no sensitive data.

## License

This project is licensed under the [MIT License](LICENSE).

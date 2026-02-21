# HoLA — Feature Roadmap

> Living document tracking feature ideas, plans, and progress for HoLA.
> Updated as features move through the pipeline.

## Status Legend

| Status | Meaning |
|--------|---------|
| `done` | Shipped and available |
| `in-progress` | Actively being worked on |
| `planned` | Committed to building (from SPEC.md v2) |
| `idea` | Brainstormed, not yet committed |

**Effort estimates:** `S` = small (hours), `M` = medium (1-3 days), `L` = large (3+ days)

---

## 1. Notifications & Alerts

| # | Feature | Status | Effort | Description |
|---|---------|--------|--------|-------------|
| 1 | Ntfy push notifications | `planned` | M | Agent sends alerts via self-hosted Ntfy on container crash, server unreachable, disk threshold exceeded. |
| 2 | Container restart tracking | `planned` | S | Track restart count since agent start. Surface "restarted 5x in last hour" as an early warning signal. |
| 3 | Docker HEALTHCHECK integration | `idea` | S | Surface healthy/unhealthy status from Docker's built-in HEALTHCHECK in the app. Free data — many images already define it. |
| 4 | Disk space threshold alerts | `idea` | S | Configurable thresholds (80%/90%/95%) that trigger Ntfy notifications. Depends on #1. |
| 5 | Cross-server health dashboard | `idea` | M | Single screen showing all servers at a glance with red/yellow/green status — no need to tap into each server. |

## 2. Debugging & Troubleshooting

| # | Feature | Status | Effort | Description |
|---|---------|--------|--------|-------------|
| 6 | Log search & filter | `idea` | S | Client-side grep-like search within fetched container logs in the app. |
| 7 | Multi-container log view | `idea` | M | Interleaved, color-coded logs from all containers in a stack — essential for debugging service interactions. |
| 8 | Per-container resource usage | `planned` | M | CPU/RAM per container via `docker stats`. See which container is consuming resources. |
| 9 | Container detail inspector | `idea` | S | Show ports, mounts, networks, environment variables (secrets masked), labels, and health status. |
| 10 | Real-time log streaming (app) | `done` | M | WebSocket-based live tail in the Android app. Live indicator, pause/follow, buffer cap, dedup. |
| 11 | Log export & share | `idea` | S | Copy logs to clipboard or share via Android share sheet for pasting into chats or issue trackers. |

## 3. Routine Maintenance

| # | Feature | Status | Effort | Description |
|---|---------|--------|--------|-------------|
| 12 | Image update checker | `idea` | L | Compare running image digest vs registry latest. Show "3 stacks have updates available." The #1 maintenance pain in homelabs. |
| 13 | Batch operations | `idea` | M | Select multiple stacks across a server — pull all, restart selected. Reduces repetitive tapping. |
| 14 | Agent self-update | `done` | M | Agent checks GitHub Releases for new versions, downloads and replaces itself, restarts via systemd. |
| 15 | Compose file backup & export | `idea` | S | Download all compose files from a server as a zip — peace of mind before making changes. |
| 16 | Quick actions | `idea` | S | One-tap shortcuts from home screen: "pull all stacks", "health check all servers". |

## 4. Multi-Server Management

| # | Feature | Status | Effort | Description |
|---|---------|--------|--------|-------------|
| 17 | Unified stack list | `idea` | M | See ALL stacks across all servers in one flat list, filterable and sortable by server, status, or name. |
| 18 | Server groups & tags | `idea` | S | Tag servers (e.g., "media", "infra", "dev") for grouping and filtering. Useful at 3-5 servers. |
| 19 | Cross-server search | `idea` | S | "Where is Postgres running?" — search by container or stack name across all connected servers. |
| 20 | QR code onboarding | `planned` | S | Agent displays QR code containing connection details. App scans it → server added instantly. |
| 21 | mDNS auto-discovery | `planned` | M | Agent broadcasts on LAN via mDNS. App discovers agents automatically without manual IP entry. |

## 5. App UX

| # | Feature | Status | Effort | Description |
|---|---------|--------|--------|-------------|
| 22 | Favorites & pinned stacks | `idea` | S | Pin critical stacks to the top of server detail. Quick access to what matters most. |
| 23 | Android home screen widget | `idea` | M | Glanceable widget: all servers green? Any alerts? Check status without opening the app. |
| 24 | App shortcuts | `idea` | S | Long-press app icon → jump directly to a specific server. Standard Android feature. |
| 25 | Swipe actions on stacks | `idea` | S | Swipe to restart, long-press for more options. Polish feature for faster interactions. |
| 26 | Connection diagnostics | `idea` | S | "Test all servers" button showing latency, token validity, and agent version for each server. |

## 6. Observability

| # | Feature | Status | Effort | Description |
|---|---------|--------|--------|-------------|
| 27 | Short-term metrics history | `idea` | M | Last-hour sparkline for CPU/RAM/disk. Agent keeps a ring buffer in memory (still stateless on restart). |
| 28 | Network I/O metrics | `planned` | S | Bandwidth per network interface. Useful for spotting if a server is saturating its link. |
| 29 | GPU metrics | `idea` | M | nvidia-smi / rocm-smi data for homelabs running Plex/Jellyfin with hardware transcoding. |
| 30 | Process list (top N) | `idea` | S | Top 10 CPU/RAM-consuming processes on the server. Quick "what's eating my resources?" without SSH. |

---

## Summary

| Status | Count |
|--------|-------|
| `done` | 1 |
| `in-progress` | 0 |
| `planned` | 7 |
| `idea` | 22 |
| **Total** | **30** |

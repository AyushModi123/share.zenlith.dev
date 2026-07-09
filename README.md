# Share

A high-performance, minimalist WebSocket signaling server designed for P2P WebRTC peer discovery and handshake orchestration.

This server facilitates the initial connection between peers for **share.zenlith.dev**. It acts strictly as a directory and message router; at no point does file data traverse this infrastructure.

## Key Principles

- **Zero Persistence**: All peer state is held in-memory. Connections are ephemeral.
- **Privacy by Design**: The server routes encrypted WebRTC envelopes. It has zero visibility into the P2P data stream.
- **Concurrency**: Built with Go's CSP model (goroutines/channels) for high throughput and low latency.
- **Minimal Footprint**: Deployed as a statically linked binary in a `scratch` Docker container.

## Architecture

The system utilizes a `Hub` to manage thread-safe client registration and message routing.

```
Client A <───WebSocket───> [ Hub ] <───WebSocket───> Client B
   |                          |                        |
   └─────────────────── Direct P2P ────────────────────┘
                     (WebRTC DataChannel)
```

1. **Registration**: Client connects, receives an ID and a randomly generated alias.
2. **Discovery**: Server broadcasts join/leave events and provides initial peer lists — **scoped to the client's room** (see below).
3. **Signaling**: Server routes SDP offers, answers, and ICE candidates between specific peer IDs in the same room.
4. **Handoff**: Once the WebRTC connection is established, the signaling server is no longer involved in the data path.

### Rooms

Peers only discover and signal others in the **same room**, so the peer list stays small and private:

- **Network rooms (default)**: with no room specified, clients are grouped by a hash of their source IP (`net-xxxxxxxx`). Devices on the same network find each other automatically.
- **Private rooms**: connect with `?room=CODE` (surfaced in the UI as a shareable `#room=CODE` link). Anyone who opens that link joins the same room regardless of network — this is how peers on different networks pair. Codes are sanitized to `[A-Za-z0-9_-]` (max 32 chars) and namespaced as `code-CODE`.

Cross-room message delivery is silently dropped, keeping rooms fully isolated.

---

## Technical Specification

### Protocol (JSON over WebSocket)

| Type | Direction | Description |
| :--- | :--- | :--- |
| `welcome` | S → C | Assigned identity (`id`, `name`) and assigned `room` upon connection. |
| `peers` | S → C | List of active `PeerInfo` objects available for connection. |
| `peer-joined` | S → All | Broadcast notification of a new peer connection. |
| `peer-left` | S → All | Broadcast notification of a peer disconnection. |
| `offer` | C ⇄ C | WebRTC SDP Offer (requires `to` field). |
| `answer` | C ⇄ C | WebRTC SDP Answer (requires `to` field). |
| `ice-candidate`| C ⇄ C | WebRTC ICE Candidate (requires `to` field). |
| `file-offer` | C ⇄ C | Metadata for a pending file transfer. |
| `file-accept` | C ⇄ C | Peer acceptance of a file transfer request. |
| `file-decline` | C ⇄ C | Peer rejection of a file transfer request. |

> Once the WebRTC DataChannel is open, transfer control flows peer-to-peer and never touches the server: file bytes travel as binary chunks, and small JSON **control messages** (`{"type":"done"}` on completion, `{"type":"cancel"}` on abort) travel as strings. The receiver distinguishes them by payload type (string vs. `ArrayBuffer`) and only finalizes the file after the explicit `done` signal — after the sender drains its send buffer — so transfers are never truncated.

### Constraints

- **Max Message Size**: 4096 bytes (enforced at the WebSocket layer).
- **Heartbeat**: 60s pong timeout with 54s ping intervals.
- **Buffer**: 64-message outbound channel buffer per client.
- **Rate Limit**: 60 messages/second per connection (bursty ICE traffic accommodated).
- **Room Cap**: 50 peers per room; **20 connections** per source IP.

### Deployment

The server expects a `PORT` environment variable (defaults to `8080`). The web client is **embedded into the binary** (`//go:embed frontend`) and served at `/`, so a single process serves both the app and the signaling endpoint — local development works with zero extra setup:

```bash
# Local development — then open http://localhost:8080
go run .

# Production (Docker)
docker build -t share-server .
docker run -p 8080:8080 share-server
```

The client derives its WebSocket host from the page's own origin (so local dev "just works"), with a production override mapping `share.zenlith.dev` → `share-ws.zenlith.dev`.

The server shuts down gracefully on `SIGINT`/`SIGTERM`, draining in-flight requests within a 5-second window.

### Security

- **Origin Validation**: Currently configured to accept all origins (`*`) to facilitate Cloudflare proxying.
- **Identity**: Peer IDs are server-generated (8-byte random hex) and enforced on all outgoing messages. The `room` is server-assigned and never trusted from the client.
- **Isolation**: Signaling is confined to a room; cross-room routing is dropped.
- **Abuse guards**: per-connection rate limiting, per-room caps, and per-IP connection limits (see Constraints).


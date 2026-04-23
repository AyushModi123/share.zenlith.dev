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
2. **Discovery**: Server broadcasts join/leave events and provides initial peer lists.
3. **Signaling**: Server routes SDP offers, answers, and ICE candidates between specific peer IDs.
4. **Handoff**: Once the WebRTC connection is established, the signaling server is no longer involved in the data path.

---

## Technical Specification

### Protocol (JSON over WebSocket)

| Type | Direction | Description |
| :--- | :--- | :--- |
| `welcome` | S → C | Assigned identity (`id`, `name`) upon connection. |
| `peers` | S → C | List of active `PeerInfo` objects available for connection. |
| `peer-joined` | S → All | Broadcast notification of a new peer connection. |
| `peer-left` | S → All | Broadcast notification of a peer disconnection. |
| `offer` | C ⇄ C | WebRTC SDP Offer (requires `to` field). |
| `answer` | C ⇄ C | WebRTC SDP Answer (requires `to` field). |
| `ice-candidate`| C ⇄ C | WebRTC ICE Candidate (requires `to` field). |
| `file-offer` | C ⇄ C | Metadata for a pending file transfer. |
| `file-accept` | C ⇄ C | Peer acceptance of a file transfer request. |
| `file-decline` | C ⇄ C | Peer rejection of a file transfer request. |

### Constraints

- **Max Message Size**: 4096 bytes (enforced at the WebSocket layer).
- **Heartbeat**: 60s pong timeout with 54s ping intervals.
- **Buffer**: 64-message outbound channel buffer per client.

### Deployment

The server expects a `PORT` environment variable (defaults to `8080`).

```bash
# Local development
go run .

# Production (Docker)
docker build -t share-server .
docker run -p 8080:8080 share-server
```

### Security

- **Origin Validation**: Currently configured to accept all origins (`*`) to facilitate Cloudflare proxying.
- **Identity**: Peer IDs are server-generated (8-byte random hex) and enforced on all outgoing messages.


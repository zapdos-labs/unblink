# UNBLINK Protocol v1

## Overview

UNBLINK v1 defines a minimal, TCP-based bridging protocol with strict separation of concerns:

- **Relay**: Public traffic router and multiplexer
- **Node**: Private TCP forwarder
- **Client**: Handles all protocol, path, and authentication logic

The Relay and Node never interpret application data.

- Paths (e.g., /stream1) live inside application protocols
- Authentication lives inside application protocols
- Relay and Node are unaware of both

## Client Connection Options

- Client connects via HTTPS (or WSS) to sni.relay_domain.com
- The relay can use SNI (Server Name Indication) to know which node or service to connect to.

# Client & Bridge

# Bridge Sharing (Not Yet Implemented)

A future enhancement could allow multiple clients to share a single bridge, reducing resource usage:

- **Subscriber tracking**: The bridge maintains a subscriber count
- **Lifecycle management**:
  - Bridge created when count goes from 0 → 1 (first client connects)
  - Bridge destroyed when count goes from 1 → 0 (last client disconnects)
- **Efficiency**: One TCP connection serves multiple clients for the same service

## Roles

### Relay

- Publicly reachable
- Manages Nodes and Clients
- Creates and multiplexes bridges

### Node

- Runs in private network
- Maintains one persistent TLS connection to Relay
- Opens TCP connections on demand
- Forwards raw bytes

### Client

- Connects to Relay
- Speaks application protocols (RTSP, HTTP, etc.)
- Handles paths, auth, and sessions

## Core Concepts

### Control Connection

The persistent TLS connection between Node and Relay that serves as the communication channel for:

- Control messages (bridge management, registration)
- Multiplexed data streams
- Keep-alive signals

### Bridge

A logical data channel that represents one TCP connection:

- Uniquely identified by `bridge_id`
- Maps: Client ↔ Relay ↔ Node ↔ Target Service
- Currently supports 1:1 client-to-bridge mapping

### Service

A target service that the Node can reach within its private network:

```json
{
  "addr": "192.168.1.100", // Private IP address
  "port": 554 // TCP port number
}
```

The Relay stores services but never interprets them - they're opaque identifiers for the Node.

## Message Encoding

All messages are CBOR-encoded maps with exactly one CBOR object per message.

## Authorization Flow

Before a node can register and start serving traffic, it must be authorized by a user through the dashboard. This ensures that only legitimate nodes can connect to the relay.

### Authorization Messages

#### REQ_AUTHORIZATION_URL (Node → Relay)

Sent by a node that needs authorization (no token).

```json
{
  "msg_id": "msg-001",
  "control": {
    "type": "req_authorization_url",
    "node_id": "node-001"
  }
}
```

#### RES_AUTHORIZATION_URL (Relay → Node)

Response containing the URL the user must visit to authorize the node.

```json
{
  "msg_id": "msg-002",
  "control": {
    "type": "res_authorization_url",
    "auth_url": "http://localhost:3000/authorize?node=node-001"
  }
}
```

#### AUTH_TOKEN (Relay → Node)

Sent when user completes authorization in the dashboard. Contains the token the node must use for future connections.

```json
{
  "msg_id": "msg-003",
  "control": {
    "type": "auth_token",
    "token": "secure-random-token-here"
  }
}
```

#### CONNECTION_READY (Relay → Node)

Sent after successful registration to confirm the node is connected and authorized.

```json
{
  "msg_id": "msg-004",
  "control": {
    "type": "connection_ready",
    "node_id": "node-001",
    "dashboard_url": "" // Empty when already authorized, populated during auth flow
  }
}
```

### Authorization Flow Sequence

#### First-Time Authorization (No Token)

```
1. Node  → Relay : REQ_AUTHORIZATION_URL(node_id)
2. Relay → Node  : ACK
3. Relay → Node  : RES_AUTHORIZATION_URL(auth_url)
4. Node displays auth_url to user
5. User opens browser → Dashboard → Authorizes node
6. Dashboard → Relay API : POST /api/authorize (node_id, user session)
7. Relay → Node  : AUTH_TOKEN(token)
8. Node saves token to config
9. Node  → Relay : REGISTER(node_id, token)
10. Relay → Node  : ACK
11. Relay → Node  : CONNECTION_READY(node_id, "")
12. Node  → Relay : ANNOUNCE(services)
13. Relay → Node  : ACK
```

#### Subsequent Connections (With Token)

```
1. Node  → Relay : REGISTER(node_id, token)
2. Relay → Node  : ACK
3. Relay → Node  : CONNECTION_READY(node_id, "")
4. Node  → Relay : ANNOUNCE(services)
5. Relay → Node  : ACK
```

### Authorization State Management

The relay maintains node authorization state in a database:

- **nodes table**: Stores node_id, token, owner_id (user), and authorization timestamp
- Token validation: The relay checks the token against the database during REGISTER
- Ownership: Each node is owned by one user who authorized it
- Pre-registration: During authorization flow, the node connection is temporarily stored to receive the AUTH_TOKEN

### Important Implementation Notes

1. **Connection Preservation**: During authorization, the node's connection is stored in the relay's nodes map when REQ_AUTHORIZATION_URL is received. This allows the relay to send AUTH_TOKEN on the same connection after user authorization.

2. **Avoiding Self-Disconnect**: When REGISTER is received with a valid token, the relay's `registerNode()` function must check if the connection being registered is already in the map. If it's the same connection, it should NOT close it (this would cause a "connection reset by peer" error).

3. **Token Persistence**: The node saves the received token to its config file automatically. On subsequent runs, it uses this token to authenticate without user interaction.

4. **Security**: Tokens are generated using cryptographically secure random number generation and are unique per node.

## Messages

### REGISTER (Node → Relay)

```json
{
  "msg_id": "msg-001",
  "control": {
    "type": "register",
    "node_id": "node-001",
    "token": "secure-random-token" // Optional: omitted if not yet authorized
  }
}
```

### ANNOUNCE (Node → Relay)

Declares services reachable by the Node.

```json
{
  "msg_id": "msg-002",
  "control": {
    "type": "announce",
    "services": [
      {
        "addr": "192.168.1.100",
        "port": 554
      },
      {
        "addr": "127.0.0.1",
        "port": 443
      }
    ]
  }
}
```

Relay stores announced services but does not interpret them.

### ACK (Bidirectional)

```json
{
  "msg_id": "ack-uuid-001",
  "control": {
    "type": "ack",
    "ack_msg_id": "msg-001"
  }
}
```

The ACK message has its own unique `msg_id`, and the `ack_msg_id` field contains the `msg_id` of the message being acknowledged.

### OPEN_BRIDGE (Relay → Node)

```json
{
  "msg_id": "msg-003",
  "control": {
    "type": "open_bridge",
    "bridge_id": "bridge-123",
    "service": {
      "addr": "192.168.1.100",
      "port": 554
    }
  }
}
```

### CLOSE_BRIDGE (Relay → Node)

```json
{
  "msg_id": "msg-004",
  "control": {
    "type": "close_bridge",
    "bridge_id": "bridge-123"
  }
}
```

### DATA (Bidirectional)

```json
{
  "msg_id": "msg-005",
  "data": {
    "bridge_id": "bridge-123",
    "payload": <byte string>
  }
}
```

All messages in the protocol include a `msg_id` field at the top level for tracking and correlation purposes.

## Data Flow Sequences

### 1. Node Registration Flow (With Existing Token)

```
Node  → Relay : REGISTER(token)
Relay → Node  : ACK
Relay → Node  : CONNECTION_READY
```

(For first-time registration without a token, see the Authorization Flow section above.)

### 2. Service Announcement Flow (Repeatable)

```
Node → Relay : ANNOUNCE(services)
Relay → Node  : ACK
```

### 3. Bridge Creation Flow

```
Client → Relay : Connect
Relay → Node   : OPEN_BRIDGE(service, bridge_id)
Node  → Relay  : ACK
Node opens TCP connection to service.addr:service.port
```

### 4. Data Forwarding Flow

```
Client → Relay : application bytes
Relay → Node   : DATA(bridge_id, payload)
Node  → Target : payload

Target → Node  : payload
Node   → Relay : DATA(bridge_id, payload)
Relay  → Client: payload
```

Payload is never inspected or modified.

### 5. Bridge Teardown Flow

```
Client disconnects OR error
Relay → Node : CLOSE_BRIDGE(bridge_id)
Node closes local TCP socket
```

## Security

### Authorization

- **User Authentication**: The dashboard requires user login (session-based) before allowing node authorization
- **Node Ownership**: Each node is permanently bound to the user who authorized it
- **Token Validation**: Every REGISTER message includes token validation against the database
- **Single Owner**: A node can only be authorized by one user; re-authorization requires logout first

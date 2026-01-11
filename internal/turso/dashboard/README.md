# Dashboard - Real-Time WebSocket Server

The dashboard package provides a WebSocket server for real-time monitoring of task state changes in the beads issue tracker. It enables multiple AI agents to coordinate by broadcasting task updates, dependency changes, and sync events to all connected clients.

## Features

- **WebSocket Server**: Real-time bidirectional communication
- **Message Broadcasting**: Efficient message distribution to all connected clients
- **Event Handling**: Structured event system for task and dependency changes
- **Task Statistics**: Real-time stats tracking (total, by status, blocked count)
- **Health Monitoring**: Built-in health check endpoint
- **Graceful Shutdown**: Clean connection closure on server stop

## Architecture

```
┌─────────────────┐
│  Sync Daemon    │
│  (file watcher) │
└────────┬────────┘
         │ Events
         ▼
┌─────────────────┐      WebSocket     ┌──────────────┐
│    Handler      │─────────────────────│   Client 1   │
│  (event format) │                     └──────────────┘
└────────┬────────┘                     ┌──────────────┐
         │ Messages                     │   Client 2   │
         ▼                              └──────────────┘
┌─────────────────┐                     ┌──────────────┐
│     Server      │─────────────────────│   Client N   │
│  (broadcasting) │      WebSocket      └──────────────┘
└─────────────────┘
```

## Message Types

The dashboard broadcasts JSON messages with the following structure:

```json
{
  "type": "task_update" | "dep_update" | "sync_complete" | "stats" | "blocked_cache",
  "timestamp": "2026-01-10T12:34:56Z",
  "data": { ... }
}
```

### Message Types

- **task_update**: Task created, updated, or deleted
- **dep_update**: Dependency added or removed
- **sync_complete**: Full sync operation completed
- **stats**: Task statistics (total, by status, blocked count)
- **blocked_cache**: Blocked cache refresh completed

## Usage

### Starting the Server

```bash
# Start on default port 8080
bd dashboard

# Start on custom port
bd dashboard --port 9000
```

### Programmatic Usage

```go
import "github.com/steveyegge/beads/internal/turso/dashboard"

// Create server
config := &dashboard.Config{
    Port:   8080,
    Logger: log.Default(),
}
server := dashboard.NewServer(config)

// Start server
if err := server.Start(); err != nil {
    log.Fatal(err)
}

// Create event handler
handler := dashboard.NewHandler(server, log.Default())

// Handle events
handler.OnTaskCreated(task)
handler.OnDepAdded(dep)
handler.OnSyncComplete(100, 50, 2*time.Second)

// Graceful shutdown
server.Stop()
```

### Connecting a Client

WebSocket clients connect to `ws://localhost:8080/ws`:

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log(`Received ${msg.type}:`, msg.data);
};
```

### Health Check

HTTP GET to `/health` returns server status:

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "ok",
  "clients": 3
}
```

## Event Examples

### Task Update

```json
{
  "type": "task_update",
  "timestamp": "2026-01-10T12:34:56Z",
  "data": {
    "task_id": "bd-xyz",
    "action": "created",
    "status": "open",
    "title": "Implement feature X",
    "priority": 1,
    "assignee": "agent-42"
  }
}
```

### Dependency Update

```json
{
  "type": "dep_update",
  "timestamp": "2026-01-10T12:34:56Z",
  "data": {
    "from": "bd-abc",
    "to": "bd-xyz",
    "type": "blocks",
    "action": "added"
  }
}
```

### Statistics

```json
{
  "type": "stats",
  "timestamp": "2026-01-10T12:34:56Z",
  "data": {
    "total": 150,
    "by_status": {
      "open": 50,
      "in_progress": 25,
      "blocked": 30,
      "closed": 45
    },
    "blocked": 30,
    "ready": 95,
    "in_progress": 25
  }
}
```

### Sync Complete

```json
{
  "type": "sync_complete",
  "timestamp": "2026-01-10T12:34:56Z",
  "data": {
    "tasks_processed": 100,
    "deps_processed": 50,
    "duration": 2000000000
  }
}
```

### Blocked Cache Refresh

```json
{
  "type": "blocked_cache",
  "timestamp": "2026-01-10T12:34:56Z",
  "data": {
    "blocked_count": 30,
    "ready_count": 95
  }
}
```

## Configuration

### Server Config

```go
type Config struct {
    Port   int         // Port to listen on (default: 8080)
    Logger *log.Logger // Logger for server activity (default: log.Default())
}
```

### Environment Variables

The dashboard respects standard HTTP server configuration:
- `PORT`: Override default port (alternative to --port flag)

## Testing

The package includes comprehensive tests:

```bash
go test ./internal/turso/dashboard/
```

Tests cover:
- Server lifecycle (start/stop)
- WebSocket connections (single and multiple clients)
- Message broadcasting
- Event handling (task, dep, sync events)
- Statistics tracking
- Health endpoint

## Integration with Turso Daemon

The dashboard is designed to work with the turso sync daemon:

1. **Daemon watches** task and dependency files
2. **Daemon syncs** changes to Turso database
3. **Handler formats** events as dashboard messages
4. **Server broadcasts** messages to all connected clients
5. **Clients update** their UI in real-time

## Security Considerations

### Development Mode

The current implementation accepts WebSocket connections from any origin (`OriginPatterns: ["*"]`). This is suitable for development and local use.

### Production Mode

For production deployment, configure:
- **Origin restrictions**: Whitelist specific origins
- **TLS/SSL**: Use `wss://` instead of `ws://`
- **Authentication**: Add token-based auth for WebSocket connections
- **Rate limiting**: Prevent abuse from malicious clients

## Performance

- **Buffered broadcast channel**: 100 message buffer to handle bursts
- **Concurrent client handling**: Each client runs in its own goroutine
- **Read/write timeouts**: 5-10 second timeouts prevent hung connections
- **Graceful shutdown**: Clients notified with proper WebSocket close frames

## Future Enhancements

- [ ] Authentication and authorization
- [ ] Message filtering (clients subscribe to specific task IDs)
- [ ] Historical event replay on connection
- [ ] Metrics endpoint (Prometheus format)
- [ ] TLS/SSL support
- [ ] Rate limiting per client
- [ ] Compression for large messages
- [ ] Binary protocol option (protobuf/msgpack)

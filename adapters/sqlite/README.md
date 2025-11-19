# SQLite Adapter

A pure Go SQLite implementation for the workflow library's `RecordStore`, `TimeoutStore`, and `EventStreamer` interfaces.

## Features

- **Pure Go**: Uses `modernc.org/sqlite` (no CGO dependencies)
- **Complete Implementation**: All three core interfaces supported
- **Minimal Dependencies**: Only SQLite driver and workflow library
- **Optimized Configuration**: WAL mode, proper pragmas, and retry logic
- **Thread-Safe**: Safe for concurrent use within SQLite's limitations

## Interfaces Implemented

- ✅ **RecordStore**: Store and retrieve workflow records with transactional outbox pattern
- ✅ **TimeoutStore**: Manage workflow timeouts with expiration tracking
- ✅ **EventStreamer**: Event streaming with SQLite-based persistence and cursor tracking

## Installation

```go
go get github.com/luno/workflow/adapters/sqlite
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/luno/workflow/adapters/sqlite"
)

func main() {
    // Open database with optimized settings
    db, err := sqlite.Open("workflow.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Initialize schema
    if err := sqlite.InitSchema(db); err != nil {
        log.Fatal(err)
    }

    // Create adapters
    recordStore := sqlite.NewRecordStore(db)
    timeoutStore := sqlite.NewTimeoutStore(db)
    eventStreamer := sqlite.NewEventStreamer(db)

    // Use with workflow builder
    // builder := workflow.NewBuilder[MyType](...)
    //     .AddRecordStore(recordStore)
    //     .AddTimeoutStore(timeoutStore)
    //     .AddEventStreamer(eventStreamer)
}
```

## Configuration

The `sqlite.Open()` function automatically configures SQLite for optimal workflow usage:

- **WAL Mode**: Write-Ahead Logging for better concurrency
- **Busy Timeout**: 5-second wait for database locks
- **Cache Size**: Increased for better performance
- **Single Connection**: Optimized for SQLite's architecture

## Schema

The adapter creates these tables automatically:

- `workflow_records`: Store workflow execution records
- `workflow_outbox`: Transactional outbox for events
- `workflow_timeouts`: Timeout tracking with expiration
- `workflow_events`: Event streaming log
- `workflow_cursors`: Consumer position tracking

## Usage Notes

### Concurrency Limitations

SQLite is designed for single-writer scenarios. While this adapter includes retry logic and optimizations:

- ✅ **Good for**: Single-node applications, moderate load, development/testing
- ⚠️ **Limited**: High-concurrency scenarios with many concurrent writers
- ❌ **Not for**: Multi-node deployments, high-throughput production systems

For high-concurrency scenarios, consider:
- `sqlstore` + `sqltimeout` (MySQL/PostgreSQL)
- `reflexstreamer` (dedicated event streaming)
- `kafkastreamer` (Kafka-based streaming)

### Event Streaming

The EventStreamer implementation:
- Stores events in SQLite tables (not in-memory)
- Supports `StreamFromLatest()` option
- Uses polling-based consumption (10ms intervals by default)
- Includes automatic retry logic for database locks

### Error Handling

The adapter includes retry logic for common SQLite contention issues:
- Automatic retry on `SQLITE_BUSY` errors
- Exponential backoff for lock conflicts
- Graceful handling of concurrent access patterns

## Testing

All adapters are tested against the standard adapter test suite:

```bash
go test -v
```

**Note**: The full `RunEventStreamerTest` may timeout in high-concurrency scenarios due to SQLite's single-writer nature. Individual interface tests pass completely.

## Performance Characteristics

- **Read Performance**: Excellent with proper indexing
- **Write Performance**: Good for moderate loads
- **Concurrent Reads**: Supported via WAL mode
- **Concurrent Writes**: Limited by SQLite's single-writer design
- **Storage**: Efficient, single-file database

## Best Practices

1. **Use for appropriate workloads**: Single-node, moderate concurrency
2. **Monitor database size**: SQLite can handle large databases efficiently
3. **Regular maintenance**: Use `PRAGMA optimize` periodically
4. **Backup strategy**: Simple file-based backups work well
5. **Connection management**: Use single connection per process

## Comparison with Other Adapters

| Feature | SQLite | SQL + Timeout | Reflex + Kafka | In-Memory |
|---------|---------|---------------|----------------|-----------|
| Setup Complexity | Low | Medium | High | None |
| Production Ready | Limited | Yes | Yes | No |
| Multi-Node | No | Yes | Yes | No |
| Persistence | Yes | Yes | Yes | No |
| High Concurrency | No | Yes | Yes | Yes |

## Contributing

This adapter follows the same patterns as other workflow adapters. When contributing:

1. Run the full test suite
2. Follow existing error handling patterns
3. Maintain compatibility with the workflow interfaces
4. Update documentation for any new features

## License

Same as the workflow library.
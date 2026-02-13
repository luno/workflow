# SQL Example - Complete MariaDB/MySQL Workflow

This example demonstrates a complete workflow setup using MariaDB/MySQL for persistent storage. It shows all the key components working together:

- ✅ **SQL RecordStore** - Persist workflow state in MariaDB
- ✅ **SQL TimeoutStore** - Manage scheduled operations in the database
- ✅ **Transactional Outbox Pattern** - Ensure exactly-once event processing
- ✅ **Event Streaming** - Coordinate workflow steps via events
- ✅ **Real-world Workflow** - Order processing with multiple steps

## What You'll Learn

1. How to set up MariaDB/MySQL with Workflow
2. How to configure connection pooling and timeouts
3. How the transactional outbox pattern works
4. How to persist workflow state, events, and timeouts to SQL
5. How to query workflow data directly from the database

## Prerequisites

- Docker and Docker Compose installed
- Go 1.21 or later

## Quick Start

### 1. Start MariaDB

```bash
docker-compose up -d
```

This starts MariaDB and automatically creates the required schema.

### 2. Run the Example

```bash
go run .
```

You'll see output showing:
- Database connection established
- Schema verification
- Order workflows being processed
- Data being persisted to MariaDB

### 3. Inspect the Database

Connect to MariaDB and query the workflow data:

```bash
# Connect to MariaDB
docker exec -it sqlexample-mariadb-1 mysql -uworkflow_user -pworkflow_pass workflow_db

# Query workflow records
SELECT workflow_name, foreign_id, run_id, run_state, status, created_at 
FROM workflow_records 
ORDER BY created_at DESC;

# Query outbox events
SELECT id, workflow_name, created_at 
FROM workflow_outbox 
ORDER BY created_at DESC;
```

### 4. Clean Up

```bash
docker-compose down -v
```

## Example Overview

### Order Processing Workflow

The example implements a realistic e-commerce order workflow:

```
Order Created → Validate → Process Payment → Fulfill → Completed
     ↓ (invalid)                ↓ (failed)      ↓ (failed)
  Rejected               Payment Failed     Fulfillment Failed
```

**Order States:**
- `OrderCreated` - Initial state when order is placed
- `OrderValidated` - Order passed validation checks
- `PaymentProcessed` - Payment successfully charged
- `OrderFulfilled` - Items shipped to customer
- `OrderCompleted` - Order fully complete
- `OrderRejected` - Validation failed
- `PaymentFailed` - Payment processing failed
- `FulfillmentFailed` - Shipping failed

### Database Tables Used

**workflow_records** - Stores order workflow state:
- `workflow_name`: "order-processor"
- `foreign_id`: Order ID (e.g., "order-1001")
- `run_id`: Unique execution ID (UUID)
- `run_state`: System state (Running, Completed, etc.)
- `status`: Business state (OrderCreated, PaymentProcessed, etc.)
- `object`: Serialised Order data (customer, items, totals)
- `created_at`, `updated_at`: Audit timestamps

**workflow_outbox** - Transactional outbox for events:
- Events created when state changes
- Published to event stream
- Deleted after successful publish
- Ensures exactly-once processing

### Transactional Outbox Pattern

The example demonstrates the transactional outbox pattern:

1. **Step Executes** - Payment processing step runs
2. **Transaction Begins** - Database transaction starts
3. **State Update** - Order state updated to `PaymentProcessed`
4. **Event Written** - Event written to `workflow_outbox`
5. **Transaction Commits** - Both changes committed atomically
6. **Event Published** - Outbox processor publishes event to stream
7. **Event Deleted** - Successfully published event removed from outbox

This guarantees that state changes and events are always in sync.

## Code Structure

```
sqlexample/
├── README.md              # This file
├── docker-compose.yml     # MariaDB setup
├── schema.sql            # Database schema
├── main.go               # Main application
└── main_test.go          # Tests
```

## Configuration

### Connection String

```go
dsn := "workflow_user:workflow_pass@tcp(localhost:3306)/workflow_db?parseTime=true&charset=utf8mb4"
```

**Parameters explained:**
- `parseTime=true` - Required for proper datetime handling
- `charset=utf8mb4` - Full Unicode support (including emojis)
- `collation=utf8mb4_unicode_ci` - Case-insensitive Unicode comparison

### Connection Pool

```go
db.SetMaxOpenConns(25)              // Max concurrent connections
db.SetMaxIdleConns(5)               // Keep 5 connections alive
db.SetConnMaxLifetime(5 * time.Minute)  // Recycle connections every 5min
```

Tune these based on your workload:
- **Low traffic**: `MaxOpenConns=10`, `MaxIdleConns=2`
- **Medium traffic**: `MaxOpenConns=25`, `MaxIdleConns=5` (default in example)
- **High traffic**: `MaxOpenConns=50-100`, `MaxIdleConns=10-25`

## Advanced Usage

### Query Workflow Data

You can query workflow data directly for reporting, debugging, or integration:

```go
// Find all orders in payment processing
rows, err := db.Query(`
    SELECT foreign_id, object, created_at 
    FROM workflow_records 
    WHERE workflow_name = ? AND status = ?
`, "order-processor", OrderPaymentProcessed)

// Find failed orders
rows, err := db.Query(`
    SELECT foreign_id, object 
    FROM workflow_records 
    WHERE workflow_name = ? AND status IN (?, ?, ?)
`, "order-processor", OrderRejected, OrderPaymentFailed, OrderFulfillmentFailed)

// Get order history timeline
rows, err := db.Query(`
    SELECT run_id, status, updated_at 
    FROM workflow_records 
    WHERE workflow_name = ? AND foreign_id = ?
    ORDER BY updated_at
`, "order-processor", "order-1001")
```

### Monitor Outbox Health

Check if outbox events are building up (indicates event publishing issues):

```sql
SELECT 
    workflow_name,
    COUNT(*) as pending_events,
    MIN(created_at) as oldest_event,
    MAX(created_at) as newest_event
FROM workflow_outbox
GROUP BY workflow_name;
```

If events are accumulating:
1. Check event streamer is running
2. Verify event stream (Kafka, Reflex) is available
3. Check network connectivity
4. Review logs for publish errors

### Database Backup and Recovery

**Backup workflow data:**

```bash
# Backup all workflow data
docker exec sqlexample-mariadb-1 mysqldump -uworkflow_user -pworkflow_pass workflow_db > backup.sql

# Backup just workflow tables
docker exec sqlexample-mariadb-1 mysqldump -uworkflow_user -pworkflow_pass workflow_db workflow_records workflow_outbox > backup.sql
```

**Restore:**

```bash
docker exec -i sqlexample-mariadb-1 mysql -uworkflow_user -pworkflow_pass workflow_db < backup.sql
```

## Troubleshooting

### "Failed to connect to database"

Check MariaDB is running:
```bash
docker-compose ps
docker-compose logs mariadb
```

### "Table doesn't exist"

Run schema creation:
```bash
docker exec -i sqlexample-mariadb-1 mysql -uworkflow_user -pworkflow_pass workflow_db < schema.sql
```

### "Too many connections"

Reduce connection pool or increase MariaDB max_connections:
```go
db.SetMaxOpenConns(10)  // Reduce from 25
```

### "Deadlock found when trying to get lock"

This can happen under high concurrency. The workflow library handles this automatically with retries.

## Production Considerations

### Security
- ✅ Use environment variables for credentials (not hardcoded)
- ✅ Use SSL/TLS for database connections (`tls=true` in DSN)
- ✅ Restrict database user permissions to only required tables
- ✅ Use connection timeouts to prevent hanging connections

### Performance
- ✅ Monitor slow queries with `EXPLAIN`
- ✅ Add indexes for your specific query patterns
- ✅ Implement archival strategy for old records
- ✅ Monitor outbox event lag
- ✅ Configure appropriate connection pool sizes

### High Availability
- ✅ Set up MariaDB replication for redundancy
- ✅ Use connection retry logic
- ✅ Monitor database health
- ✅ Plan for backup and recovery

## Next Steps

- **[Database Setup Guide](../../docs/database-setup.md)** - Complete database configuration guide
- **[Adapters](../../docs/adapters.md)** - Learn about all adapter types
- **[Configuration](../../docs/configuration.md)** - Tune workflow performance
- **[Monitoring](../../docs/monitoring.md)** - Set up observability

## Learn More

- [Order Processor Example](../orderprocessor) - Complex order processing workflow
- [Getting Started](../../docs/getting-started.md) - Basic workflow concepts

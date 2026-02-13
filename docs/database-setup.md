# Database Setup Guide

This guide walks you through setting up Workflow with production-ready SQL databases. We'll cover complete setup for MariaDB/MySQL and PostgreSQL, including connection configuration, schema deployment, and performance tuning.

## Quick Start

**Choose your database:**
- [MariaDB/MySQL Setup](#mariadbmysql-setup) - Popular, widely supported, excellent for most workloads
- [PostgreSQL Setup](#postgresql-setup) - Advanced features, strong consistency guarantees

## MariaDB/MySQL Setup

### 1. Installation

**Using Docker (Recommended for development):**

```bash
docker run -d \
  --name workflow-mariadb \
  -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=workflow_password \
  -e MYSQL_DATABASE=workflow_db \
  -e MYSQL_USER=workflow_user \
  -e MYSQL_PASSWORD=workflow_pass \
  mariadb:11
```

**Using Docker Compose:**

```yaml
# docker-compose.yml
version: '3.8'
services:
  mariadb:
    image: mariadb:11
    environment:
      MYSQL_ROOT_PASSWORD: workflow_password
      MYSQL_DATABASE: workflow_db
      MYSQL_USER: workflow_user
      MYSQL_PASSWORD: workflow_pass
    ports:
      - "3306:3306"
    volumes:
      - mariadb_data:/var/lib/mysql
    command: 
      - --character-set-server=utf8mb4
      - --collation-server=utf8mb4_unicode_ci
      - --max_connections=200

volumes:
  mariadb_data:
```

Start with: `docker-compose up -d`

### 2. Create Database Schema

Connect to your database and create the required tables:

```sql
-- Connect to your database
USE workflow_db;

-- Create workflow records table
CREATE TABLE workflow_records (
    workflow_name          VARCHAR(255) NOT NULL,
    foreign_id             VARCHAR(255) NOT NULL,
    run_id                 VARCHAR(255) NOT NULL,
    run_state              INT NOT NULL,
    status                 INT NOT NULL,
    object                 LONGBLOB NOT NULL,
    created_at             DATETIME(3) NOT NULL,
    updated_at             DATETIME(3) NOT NULL,
    meta                   BLOB,

    PRIMARY KEY(run_id),

    INDEX by_workflow_name_foreign_id_status (workflow_name, foreign_id, status),
    INDEX by_run_state (run_state),
    INDEX by_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create workflow outbox table (for transactional outbox pattern)
CREATE TABLE workflow_outbox (
    id                 VARCHAR(255) NOT NULL,
    workflow_name      VARCHAR(255) NOT NULL,
    data               BLOB,
    created_at         DATETIME(3) NOT NULL,

    PRIMARY KEY (id),

    INDEX by_workflow_name (workflow_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**From schema file:**

```bash
# Apply schema from the provided schema.sql file
mysql -u workflow_user -p workflow_db < adapters/sqlstore/schema.sql
```

### 3. Go Application Setup

Install the required Go packages:

```bash
go get github.com/luno/workflow
go get github.com/luno/workflow/adapters/sqlstore
go get github.com/go-sql-driver/mysql
```

**Complete working example:**

```go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "time"

    _ "github.com/go-sql-driver/mysql"
    "github.com/luno/workflow"
    "github.com/luno/workflow/adapters/memstreamer"
    "github.com/luno/workflow/adapters/memrolescheduler"
    "github.com/luno/workflow/adapters/sqlstore"
)

type TaskStatus int

const (
    TaskStatusUnknown   TaskStatus = 0
    TaskStatusCreated   TaskStatus = 1
    TaskStatusProcessed TaskStatus = 2
    TaskStatusCompleted TaskStatus = 3
)

type Task struct {
    ID   string
    Name string
}

func main() {
    // 1. Configure database connection
    dsn := "workflow_user:workflow_pass@tcp(localhost:3306)/workflow_db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci"
    
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer db.Close()

    // 2. Configure connection pool
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)

    // 3. Test connection
    if err := db.Ping(); err != nil {
        log.Fatalf("Database ping failed: %v", err)
    }

    // 4. Create sqlstore adapter
    store := sqlstore.New(db, "workflow_records", "workflow_outbox")

    // 5. Build workflow with SQL record store
    b := workflow.NewBuilder[Task, TaskStatus]("task-processor")

    b.AddStep(TaskStatusCreated, func(ctx context.Context, r *workflow.Run[Task, TaskStatus]) (TaskStatus, error) {
        fmt.Printf("Processing: %s\n", r.Object.Name)
        return TaskStatusProcessed, nil
    }, TaskStatusProcessed)

    b.AddStep(TaskStatusProcessed, func(ctx context.Context, r *workflow.Run[Task, TaskStatus]) (TaskStatus, error) {
        fmt.Printf("Completed: %s\n", r.Object.Name)
        return TaskStatusCompleted, nil
    }, TaskStatusCompleted)

    wf := b.Build(
        memstreamer.New(),     // Can use kafkastreamer for production
        store,                 // SQL record store
        memrolescheduler.New(), // Can use rinkrolescheduler for production
    )

    // 6. Run workflow
    ctx := context.Background()
    wf.Run(ctx)
    defer wf.Stop()

    // 7. Trigger a workflow - data is now persisted in MariaDB!
    runID, err := wf.Trigger(ctx, "task-1", workflow.WithInitialValue(&Task{
        ID:   "task-1",
        Name: "Process Invoice",
    }))
    if err != nil {
        log.Fatalf("Failed to trigger workflow: %v", err)
    }

    // 8. Wait for completion
    _, err = wf.Await(ctx, "task-1", runID, TaskStatusCompleted)
    if err != nil {
        log.Fatalf("Workflow failed: %v", err)
    }

    fmt.Println("✅ Workflow completed and persisted to MariaDB!")
}
```

### 4. Connection String Format

**DSN (Data Source Name) format:**

```
[username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
```

**Common examples:**

```go
// Local development
dsn := "user:pass@tcp(localhost:3306)/workflow_db?parseTime=true"

// Docker container
dsn := "user:pass@tcp(mariadb:3306)/workflow_db?parseTime=true"

// Remote server with SSL
dsn := "user:pass@tcp(db.example.com:3306)/workflow_db?parseTime=true&tls=true"

// With timezone
dsn := "user:pass@tcp(localhost:3306)/workflow_db?parseTime=true&loc=UTC"

// Full production example
dsn := "user:pass@tcp(localhost:3306)/workflow_db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&timeout=10s&readTimeout=30s&writeTimeout=30s"
```

**Important parameters:**

| Parameter | Value | Why |
|-----------|-------|-----|
| `parseTime` | `true` | Required for proper DATETIME handling |
| `charset` | `utf8mb4` | Full Unicode support including emojis |
| `collation` | `utf8mb4_unicode_ci` | Case-insensitive Unicode comparison |
| `timeout` | `10s` | Connection timeout |
| `readTimeout` | `30s` | Query read timeout |
| `writeTimeout` | `30s` | Query write timeout |

### 5. Performance Tuning

**Connection Pool Settings:**

```go
// For high-throughput applications
db.SetMaxOpenConns(50)        // Max concurrent connections
db.SetMaxIdleConns(10)        // Connections kept alive in pool
db.SetConnMaxLifetime(5 * time.Minute)  // Recycle connections
db.SetConnMaxIdleTime(1 * time.Minute)  // Close idle connections
```

**MariaDB Server Configuration** (`my.cnf`):

```ini
[mysqld]
# Connection settings
max_connections = 200

# InnoDB settings for workflow
innodb_buffer_pool_size = 1G
innodb_log_file_size = 256M
innodb_flush_log_at_trx_commit = 1  # For durability (required)
innodb_flush_method = O_DIRECT

# Query cache (disable for write-heavy workloads)
query_cache_type = 0
query_cache_size = 0

# Character set
character_set_server = utf8mb4
collation_server = utf8mb4_unicode_ci
```

**Index Optimization:**

The schema includes essential indexes. Monitor and add more as needed:

```sql
-- Check index usage
SHOW INDEX FROM workflow_records;

-- Add custom index if filtering by specific columns frequently
CREATE INDEX idx_custom ON workflow_records(workflow_name, created_at);
```

### 6. Monitoring and Maintenance

**Check table sizes:**

```sql
SELECT 
    table_name AS 'Table',
    ROUND(((data_length + index_length) / 1024 / 1024), 2) AS 'Size (MB)'
FROM information_schema.TABLES
WHERE table_schema = 'workflow_db'
ORDER BY (data_length + index_length) DESC;
```

**Archive old records:**

```sql
-- Archive completed workflows older than 30 days
DELETE FROM workflow_records 
WHERE run_state = 3  -- Completed
  AND created_at < NOW() - INTERVAL 30 DAY
LIMIT 1000;

-- Clean outbox events older than 7 days
DELETE FROM workflow_outbox
WHERE created_at < NOW() - INTERVAL 7 DAY
LIMIT 1000;
```

### 7. Common Issues and Solutions

**Problem: "Error 1040: Too many connections"**

Solution: Increase `max_connections` in MariaDB config or reduce `SetMaxOpenConns()` in Go.

**Problem: "Error 2006: MySQL server has gone away"**

Solution: Check `wait_timeout` in MariaDB and set `SetConnMaxLifetime()` lower than server's timeout.

```go
db.SetConnMaxLifetime(4 * time.Minute)  // Server timeout is 5min
```

**Problem: Slow queries**

Solution: Check indexes are being used:

```sql
EXPLAIN SELECT * FROM workflow_records 
WHERE workflow_name = 'order-processor' 
  AND foreign_id = 'order-123' 
  AND status = 2;
```

## PostgreSQL Setup

### 1. Installation

**Using Docker:**

```bash
docker run -d \
  --name workflow-postgres \
  -p 5432:5432 \
  -e POSTGRES_PASSWORD=workflow_password \
  -e POSTGRES_DB=workflow_db \
  -e POSTGRES_USER=workflow_user \
  postgres:16
```

**Using Docker Compose:**

```yaml
# docker-compose.yml
version: '3.8'
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: workflow_password
      POSTGRES_DB: workflow_db
      POSTGRES_USER: workflow_user
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    command:
      - "postgres"
      - "-c"
      - "max_connections=200"

volumes:
  postgres_data:
```

### 2. Create Database Schema

```sql
-- Connect to your database
\c workflow_db

-- Create workflow records table
CREATE TABLE workflow_records (
    workflow_name          VARCHAR(255) NOT NULL,
    foreign_id             VARCHAR(255) NOT NULL,
    run_id                 VARCHAR(255) NOT NULL,
    run_state              INTEGER NOT NULL,
    status                 INTEGER NOT NULL,
    object                 BYTEA NOT NULL,
    created_at             TIMESTAMP(3) NOT NULL,
    updated_at             TIMESTAMP(3) NOT NULL,
    meta                   BYTEA,

    PRIMARY KEY(run_id)
);

CREATE INDEX idx_workflow_name_foreign_id_status ON workflow_records(workflow_name, foreign_id, status);
CREATE INDEX idx_run_state ON workflow_records(run_state);
CREATE INDEX idx_created_at ON workflow_records(created_at);

-- Create workflow outbox table
CREATE TABLE workflow_outbox (
    id                 VARCHAR(255) NOT NULL,
    workflow_name      VARCHAR(255) NOT NULL,
    data               BYTEA,
    created_at         TIMESTAMP(3) NOT NULL,

    PRIMARY KEY (id)
);

CREATE INDEX idx_outbox_workflow_name ON workflow_outbox(workflow_name);
```

### 3. Go Application Setup

```bash
go get github.com/luno/workflow
go get github.com/luno/workflow/adapters/sqlstore
go get github.com/lib/pq
```

**Example code:**

```go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"

    _ "github.com/lib/pq"
    "github.com/luno/workflow"
    "github.com/luno/workflow/adapters/memstreamer"
    "github.com/luno/workflow/adapters/memrolescheduler"
    "github.com/luno/workflow/adapters/sqlstore"
)

func main() {
    // PostgreSQL connection string
    dsn := "postgres://workflow_user:workflow_password@localhost:5432/workflow_db?sslmode=disable"
    
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer db.Close()

    store := sqlstore.New(db, "workflow_records", "workflow_outbox")

    // ... rest of workflow setup
}
```

### 4. PostgreSQL Connection Strings

**Format:**

```
postgres://username:password@host:port/database?param=value
```

**Examples:**

```go
// Local development
dsn := "postgres://user:pass@localhost:5432/workflow_db?sslmode=disable"

// With SSL
dsn := "postgres://user:pass@db.example.com:5432/workflow_db?sslmode=require"

// With connection timeout
dsn := "postgres://user:pass@localhost:5432/workflow_db?sslmode=disable&connect_timeout=10"

// Full production example
dsn := "postgres://user:pass@localhost:5432/workflow_db?sslmode=require&connect_timeout=10&application_name=workflow"
```

### 5. Performance Tuning

**PostgreSQL Configuration** (`postgresql.conf`):

```ini
# Connection settings
max_connections = 200

# Memory settings
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 16MB
maintenance_work_mem = 128MB

# Write-ahead log
wal_level = replica
max_wal_size = 1GB
min_wal_size = 80MB

# Query planning
random_page_cost = 1.1  # For SSD storage
effective_io_concurrency = 200
```

**Connection pool:**

```go
db.SetMaxOpenConns(50)
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(5 * time.Minute)
```

## Database Comparison

| Feature | MariaDB/MySQL | PostgreSQL |
|---------|---------------|------------|
| **Performance** | Excellent for read-heavy | Better for write-heavy |
| **ACID Compliance** | ✅ InnoDB | ✅ Full ACID |
| **Setup Complexity** | Simple | Moderate |
| **JSON Support** | Good (JSON type) | Excellent (JSONB) |
| **Replication** | Master-slave | Master-slave + logical |
| **Best For** | General workflows, high read | Complex workflows, heavy writes |

## Schema Evolution

When you need to modify the schema:

```sql
-- Example: Add a new column
ALTER TABLE workflow_records 
ADD COLUMN priority INT DEFAULT 0;

-- Add index for the new column
CREATE INDEX idx_priority ON workflow_records(priority);

-- Example: Add a composite index
CREATE INDEX idx_workflow_status_created 
ON workflow_records(workflow_name, status, created_at);
```

**Migration best practices:**

1. Test migrations on a copy of production data
2. Use transactions where possible
3. Add new columns as nullable or with defaults
4. Build indexes with `CONCURRENTLY` in PostgreSQL to avoid locking
5. Consider downtime windows for major changes

## Testing Your Setup

Verify your database is working correctly:

```go
package main

import (
    "context"
    "database/sql"
    "testing"
    "time"

    _ "github.com/go-sql-driver/mysql"
    "github.com/luno/workflow"
    "github.com/luno/workflow/adapters/memstreamer"
    "github.com/luno/workflow/adapters/memrolescheduler"
    "github.com/luno/workflow/adapters/sqlstore"
)

func TestDatabaseSetup(t *testing.T) {
    dsn := "workflow_user:workflow_pass@tcp(localhost:3306)/workflow_db?parseTime=true"
    
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        t.Fatalf("Connection failed: %v", err)
    }
    defer db.Close()

    // Test connection
    if err := db.Ping(); err != nil {
        t.Fatalf("Ping failed: %v", err)
    }

    // Test schema exists
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'workflow_db' AND table_name = 'workflow_records'").Scan(&count)
    if err != nil {
        t.Fatalf("Schema check failed: %v", err)
    }
    if count != 1 {
        t.Fatalf("workflow_records table not found")
    }

    // Test workflow can write to database
    store := sqlstore.New(db, "workflow_records", "workflow_outbox")
    
    b := workflow.NewBuilder[string, int]("test-workflow")
    b.AddStep(1, func(ctx context.Context, r *workflow.Run[string, int]) (int, error) {
        return 2, nil
    }, 2)

    wf := b.Build(memstreamer.New(), store, memrolescheduler.New())
    
    ctx := context.Background()
    wf.Run(ctx)
    defer wf.Stop()

    runID, err := wf.Trigger(ctx, "test-1", workflow.WithInitialValue("test data"))
    if err != nil {
        t.Fatalf("Trigger failed: %v", err)
    }

    // Verify record was persisted
    time.Sleep(100 * time.Millisecond)
    var exists int
    err = db.QueryRow("SELECT COUNT(*) FROM workflow_records WHERE run_id = ?", runID).Scan(&exists)
    if err != nil {
        t.Fatalf("Query failed: %v", err)
    }
    if exists != 1 {
        t.Fatalf("Workflow record not persisted to database")
    }

    t.Log("✅ Database setup verified successfully!")
}
```

## Next Steps

- **[Adapters Guide](adapters.md)** - Learn about all adapter types
- **[Configuration](configuration.md)** - Tune workflow performance
- **[Monitoring](monitoring.md)** - Set up observability
- **[Examples](../examples/sqlexample)** - See complete SQL workflow example

## Need Help?

- Check the [troubleshooting section](#7-common-issues-and-solutions) above
- Review [GitHub Issues](https://github.com/luno/workflow/issues) for similar problems
- Ask questions in [GitHub Discussions](https://github.com/luno/workflow/discussions)

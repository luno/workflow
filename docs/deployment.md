# Production Deployment

This guide covers deploying Workflow in production environments, from simple single-instance setups to large-scale distributed deployments across multiple regions.

## Deployment Patterns

### Development & Testing

For local development and testing, use in-memory adapters:

```go
func NewDevelopmentWorkflow() *workflow.Workflow[Order, OrderStatus] {
    b := workflow.NewBuilder[Order, OrderStatus]("order-processing")
    // ... define steps ...

    return b.Build(
        memstreamer.New(),
        memrecordstore.New(),
        memrolescheduler.New(),
    )
}
```

**Characteristics:**
- ✅ Fast startup and teardown
- ✅ No external dependencies
- ✅ Perfect for unit tests
- ❌ No durability (data lost on restart)
- ❌ Single instance only

### Single Instance Production

For smaller workloads, run everything on one instance:

```go
func NewSingleInstanceWorkflow() *workflow.Workflow[Order, OrderStatus] {
    db := setupDatabase()

    return b.Build(
        sqlstreamer.New(db), // Use database as event store
        sqlstore.New(db, "workflow_records", "workflow_outbox"),
        memrolescheduler.New(), // Single instance scheduler
        workflow.WithTimeoutStore(sqltimeout.New(db)),
    )
}
```

**Setup Requirements:**
```yaml
# docker-compose.yml
version: '3.8'
services:
  app:
    build: .
    environment:
      - DATABASE_URL=postgres://user:pass@db:5432/workflow
    depends_on:
      - db

  db:
    image: postgres:15
    environment:
      - POSTGRES_DB=workflow
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
```

**Characteristics:**
- ✅ Simple deployment
- ✅ Persistent storage
- ✅ Good for moderate workloads
- ❌ Single point of failure
- ❌ Limited scalability

### Horizontally Scaled Production

For high-throughput workloads, distribute across multiple instances:

```go
func NewScaledWorkflow() *workflow.Workflow[Order, OrderStatus] {
    return b.Build(
        kafkastreamer.New(kafkaBrokers, kafkaConfig),
        sqlstore.New(database, "workflow_records", "workflow_outbox"),
        rinkrolescheduler.New(rinkConfig),
        workflow.WithTimeoutStore(sqltimeout.New(database)),
        workflow.WithDefaultOptions(
            workflow.ParallelCount(5), // 5 parallel consumers per step
        ),
    )
}
```

**Infrastructure Requirements:**
- **Database**: PostgreSQL or MySQL cluster
- **Message Queue**: Kafka cluster
- **Coordination**: Rink cluster or etcd
- **Load Balancer**: For web UI and API endpoints
- **Monitoring**: Prometheus and Grafana

### Multi-Region Deployment

For global applications with regional processing:

```go
func NewMultiRegionWorkflow(region string) *workflow.Workflow[Order, OrderStatus] {
    return b.Build(
        kafkastreamer.New(regionalKafkaBrokers[region], kafkaConfig),
        sqlstore.New(globalDatabase, "workflow_records", "workflow_outbox"),
        rinkrolescheduler.New(regionalRinkConfig[region]),
        workflow.WithTimeoutStore(sqltimeout.New(globalDatabase)),
    )
}
```

**Architecture:**
- **Global**: Shared database for workflow state
- **Regional**: Kafka and role schedulers per region
- **Benefits**: Reduced latency, regulatory compliance
- **Complexity**: Cross-region coordination

## Infrastructure Components

### Database Setup

#### PostgreSQL Schema

```sql
-- Workflow records table
CREATE TABLE workflow_records (
    id BIGSERIAL PRIMARY KEY,
    workflow_name VARCHAR(255) NOT NULL,
    foreign_id VARCHAR(255) NOT NULL,
    run_id VARCHAR(255) NOT NULL UNIQUE,
    run_state INTEGER NOT NULL,
    status INTEGER NOT NULL,
    object JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    meta JSONB
);

-- Indexes for performance
CREATE INDEX idx_workflow_records_workflow_foreign ON workflow_records(workflow_name, foreign_id);
CREATE INDEX idx_workflow_records_status ON workflow_records(workflow_name, status);
CREATE INDEX idx_workflow_records_run_state ON workflow_records(run_state);
CREATE INDEX idx_workflow_records_updated_at ON workflow_records(updated_at);

-- Outbox table for transactional event publishing
CREATE TABLE workflow_outbox (
    id VARCHAR(255) PRIMARY KEY,
    workflow_name VARCHAR(255) NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_outbox_workflow_created ON workflow_outbox(workflow_name, created_at);

-- Timeout table (if using sqltimeout adapter)
CREATE TABLE workflow_timeouts (
    id BIGSERIAL PRIMARY KEY,
    workflow_name VARCHAR(255) NOT NULL,
    foreign_id VARCHAR(255) NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    status INTEGER NOT NULL,
    expire_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_timeouts_expire ON workflow_timeouts(workflow_name, status, expire_at)
    WHERE NOT completed;
```

#### Database Configuration

```yaml
# PostgreSQL optimization for workflow workloads
postgresql.conf:
  # Connection settings
  max_connections: 200
  shared_buffers: '256MB'

  # Write performance
  wal_level: 'replica'
  max_wal_senders: 3
  checkpoint_completion_target: 0.9

  # Query performance
  effective_cache_size: '1GB'
  random_page_cost: 1.1

  # Monitoring
  log_statement: 'mod'
  log_min_duration_statement: 1000
```

### Kafka Setup

#### Topic Configuration

```bash
# Create workflow topics
kafka-topics.sh --create \
  --topic workflow-events \
  --partitions 12 \
  --replication-factor 3 \
  --config cleanup.policy=delete \
  --config retention.ms=604800000  # 7 days

# Create per-workflow topics (optional for better isolation)
kafka-topics.sh --create \
  --topic workflow-order-processing \
  --partitions 6 \
  --replication-factor 3
```

#### Producer Configuration

```go
kafkaConfig := &sarama.Config{
    // Reliability
    Producer.RequiredAcks: sarama.WaitForAll,
    Producer.Retry.Max: 5,
    Producer.Return.Successes: true,

    // Performance
    Producer.Flush.Frequency: 100 * time.Millisecond,
    Producer.Flush.Messages: 100,

    // Compression
    Producer.Compression: sarama.CompressionSnappy,
}
```

#### Consumer Configuration

```go
kafkaConfig := &sarama.Config{
    // Reliability
    Consumer.Offsets.Initial: sarama.OffsetOldest,
    Consumer.Group.Rebalance.Strategy: sarama.BalanceStrategyRoundRobin,

    // Performance
    Consumer.Fetch.Min: 1024,
    Consumer.Fetch.Max: 1024 * 1024,

    // Session management
    Consumer.Group.Session.Timeout: 10 * time.Second,
    Consumer.Group.Heartbeat.Interval: 3 * time.Second,
}
```

## Container Deployment

### Docker Image

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o workflow-app .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/workflow-app .

EXPOSE 8080
CMD ["./workflow-app"]
```

### Kubernetes Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: workflow-app
  labels:
    app: workflow-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: workflow-app
  template:
    metadata:
      labels:
        app: workflow-app
    spec:
      containers:
      - name: workflow-app
        image: myregistry/workflow-app:v1.0.0
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: workflow-secrets
              key: database-url
        - name: KAFKA_BROKERS
          value: "kafka-0:9092,kafka-1:9092,kafka-2:9092"
        - name: RINK_ENDPOINTS
          value: "rink-0:8080,rink-1:8080,rink-2:8080"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"

---
apiVersion: v1
kind: Service
metadata:
  name: workflow-app-service
spec:
  selector:
    app: workflow-app
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer

---
apiVersion: v1
kind: Secret
metadata:
  name: workflow-secrets
type: Opaque
data:
  database-url: cG9zdGdyZXM6Ly91c2VyOnBhc3NAZGItaG9zdDo1NDMyL3dvcmtmbG93
```

### Health Checks

Implement health endpoints for Kubernetes:

```go
func main() {
    wf := NewWorkflow()

    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        // Check if workflow is running
        states := wf.States()
        for _, state := range states {
            if state == workflow.StateShutdown {
                w.WriteHeader(http.StatusServiceUnavailable)
                return
            }
        }
        w.WriteHeader(http.StatusOK)
    })

    http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
        // Check if all dependencies are available
        if err := checkDatabaseHealth(); err != nil {
            w.WriteHeader(http.StatusServiceUnavailable)
            return
        }
        if err := checkKafkaHealth(); err != nil {
            w.WriteHeader(http.StatusServiceUnavailable)
            return
        }
        w.WriteHeader(http.StatusOK)
    })

    go wf.Run(context.Background())

    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## Configuration Management

### Environment-Based Configuration

```go
type Config struct {
    DatabaseURL    string `env:"DATABASE_URL" required:"true"`
    KafkaBrokers   string `env:"KAFKA_BROKERS" default:"localhost:9092"`
    RinkEndpoints  string `env:"RINK_ENDPOINTS"`
    LogLevel       string `env:"LOG_LEVEL" default:"info"`
    MetricsPort    int    `env:"METRICS_PORT" default:"9090"`
    WebUIPort      int    `env:"WEBUI_PORT" default:"8080"`
}

func LoadConfig() (*Config, error) {
    var cfg Config
    if err := env.Parse(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

### ConfigMap for Kubernetes

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: workflow-config
data:
  LOG_LEVEL: "info"
  METRICS_PORT: "9090"
  WEBUI_PORT: "8080"
  KAFKA_CONSUMER_GROUP: "workflow-processors"
  PARALLEL_COUNT: "5"
```

## Monitoring & Observability

### Metrics Collection

```go
func setupMetrics(wf *workflow.Workflow[Order, OrderStatus]) {
    // Expose Prometheus metrics
    http.Handle("/metrics", promhttp.Handler())

    // Custom business metrics
    orderProcessingDuration := prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "order_processing_duration_seconds",
            Help: "Time taken to process orders",
        },
        []string{"status"},
    )
    prometheus.MustRegister(orderProcessingDuration)

    // Hook to track order processing time
    wf.OnComplete(func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) error {
        duration := time.Since(r.CreatedAt)
        orderProcessingDuration.WithLabelValues(r.Status.String()).Observe(duration.Seconds())
        return nil
    })
}
```

### Log Aggregation

```yaml
# Fluent Bit configuration for Kubernetes
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluent-bit-config
data:
  fluent-bit.conf: |
    [SERVICE]
        Flush         1
        Log_Level     info
        Daemon        off
        Parsers_File  parsers.conf

    [INPUT]
        Name              tail
        Path              /var/log/containers/*workflow-app*.log
        Parser            docker
        Tag               workflow.*
        Refresh_Interval  5
        Mem_Buf_Limit     50MB

    [FILTER]
        Name                kubernetes
        Match               workflow.*
        Kube_URL            https://kubernetes.default.svc:443
        Kube_CA_File        /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        Kube_Token_File     /var/run/secrets/kubernetes.io/serviceaccount/token

    [OUTPUT]
        Name  es
        Match *
        Host  elasticsearch.logging.svc.cluster.local
        Port  9200
        Index workflow-logs
```

## Security Considerations

### TLS Configuration

```go
func setupTLS() *tls.Config {
    return &tls.Config{
        MinVersion: tls.VersionTLS12,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
        },
    }
}
```

### Network Policies

```yaml
# Kubernetes NetworkPolicy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: workflow-network-policy
spec:
  podSelector:
    matchLabels:
      app: workflow-app
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: database
    ports:
    - protocol: TCP
      port: 5432
  - to:
    - namespaceSelector:
        matchLabels:
          name: kafka
    ports:
    - protocol: TCP
      port: 9092
```

## Backup & Disaster Recovery

### Database Backups

```bash
#!/bin/bash
# backup-workflow-db.sh

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="workflow_backup_${TIMESTAMP}.sql"

# Create backup
pg_dump -h $DB_HOST -U $DB_USER -d workflow > $BACKUP_FILE

# Upload to S3
aws s3 cp $BACKUP_FILE s3://workflow-backups/

# Cleanup local file
rm $BACKUP_FILE

# Keep only last 30 days of backups
aws s3 ls s3://workflow-backups/ | sort | head -n -30 | awk '{print $4}' | xargs -I {} aws s3 rm s3://workflow-backups/{}
```

### Point-in-Time Recovery

```sql
-- Restore workflow database to specific point in time
CREATE DATABASE workflow_restored;

-- Restore from backup
psql -h $DB_HOST -U $DB_USER -d workflow_restored < workflow_backup_20240115_143000.sql

-- For PostgreSQL with WAL-E/WAL-G
wal-g backup-fetch /var/lib/postgresql/12/main LATEST
```

## Performance Tuning

### Database Optimization

```sql
-- Partition large tables by date
CREATE TABLE workflow_records_2024 PARTITION OF workflow_records
FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');

-- Archive old completed workflows
DELETE FROM workflow_records
WHERE run_state IN (4, 5, 6) -- Completed, Cancelled, DataDeleted
AND updated_at < NOW() - INTERVAL '90 days';
```

### Application Tuning

```go
func NewOptimizedWorkflow() *workflow.Workflow[Order, OrderStatus] {
    return b.Build(
        // ... adapters ...
        workflow.WithDefaultOptions(
            workflow.ParallelCount(10),                    // Scale based on load
            workflow.PollingFrequency(100*time.Millisecond), // Balance latency vs CPU
            workflow.ErrBackOff(time.Second*5),            // Reduce retry storms
            workflow.LagAlert(time.Minute*5),              // Early warning
        ),
    )
}
```

## Migration Strategies

### Zero-Downtime Deployments

```bash
#!/bin/bash
# rolling-deployment.sh

# Deploy new version
kubectl set image deployment/workflow-app workflow-app=myregistry/workflow-app:v2.0.0

# Wait for rollout
kubectl rollout status deployment/workflow-app

# Run database migrations (if needed)
kubectl run migration-job --image=myregistry/workflow-migrations:v2.0.0 --restart=Never

# Verify health
kubectl get pods -l app=workflow-app
```

### Schema Migrations

```sql
-- Migration: Add new column for feature flag
ALTER TABLE workflow_records ADD COLUMN feature_flags JSONB DEFAULT '{}';

-- Migration: Add index for new query pattern
CREATE INDEX CONCURRENTLY idx_workflow_records_feature_flags
ON workflow_records USING GIN (feature_flags);
```

This covers the essential aspects of deploying Workflow in production. The key is starting simple and scaling complexity as your requirements grow.

## Next Steps

- **[Monitoring](monitoring.md)** - Set up comprehensive observability
- **[Configuration](configuration.md)** - Fine-tune performance settings
- **[Troubleshooting](advanced/troubleshooting.md)** - Handle common production issues
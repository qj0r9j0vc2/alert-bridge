# Alert Bridge

A unified alert management system that bridges Alertmanager with Slack and PagerDuty, enabling bidirectional acknowledgment synchronization and persistent alert storage.

## Features

- **Alert Processing**: Receive and process alerts from Alertmanager webhooks
- **Bidirectional Sync**: Synchronize acknowledgments between Slack and PagerDuty
- **Persistent Storage**: SQLite and MySQL-based persistence for alerts, ack events, and silence rules
- **Silence Management**: Create and manage alert silences across platforms
- **Audit Trail**: Complete history of all acknowledgment events
- **High Performance**: Sub-millisecond read/write operations

## Quick Start

### Prerequisites

- Go 1.24 or later
- Slack workspace (optional)
- PagerDuty account (optional)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/qj0r9j0vc2/alert-bridge.git
cd alert-bridge
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application:
```bash
go build -o alert-bridge ./cmd/alert-bridge
```

### Configuration

Create a `config/config.yaml` file based on the example:

```bash
cp config/config.example.yaml config/config.yaml
```

Edit `config/config.yaml` with your settings:

```yaml
server:
  port: 8080
  read_timeout: 5s
  write_timeout: 10s
  shutdown_timeout: 30s

# Storage Configuration
storage:
  type: sqlite  # Options: "memory" or "sqlite"
  sqlite:
    path: ./data/alert-bridge.db  # Path to SQLite database file

slack:
  enabled: true
  bot_token: ${SLACK_BOT_TOKEN}
  signing_secret: ${SLACK_SIGNING_SECRET}
  channel_id: ${SLACK_CHANNEL_ID}

pagerduty:
  enabled: true
  api_token: ${PAGERDUTY_API_TOKEN}
  routing_key: ${PAGERDUTY_ROUTING_KEY}
  service_id: ${PAGERDUTY_SERVICE_ID}
  webhook_secret: ${PAGERDUTY_WEBHOOK_SECRET}
  from_email: ${PAGERDUTY_FROM_EMAIL}

alerting:
  deduplication_window: 5m
  resend_interval: 30m
  silence_durations: [15m, 1h, 4h, 24h]

logging:
  level: info
  format: json
```

### Storage Options

Alert Bridge supports two storage backends:

#### In-Memory Storage (Default)
Fast but ephemeral - data is lost on restart.

```yaml
storage:
  type: memory
```

**Use cases:**
- Development and testing
- Stateless deployments
- When persistence is not required

#### SQLite Storage (Recommended for Production)
Persistent storage with excellent performance.

```yaml
storage:
  type: sqlite
  sqlite:
    path: ./data/alert-bridge.db
```

**Features:**
- Data persists across restarts
- Sub-millisecond read operations (15.8µs average)
- Concurrent read support via WAL mode
- Automatic schema migrations
- Foreign key constraints and data integrity
- Graceful shutdown with WAL checkpoint

**Performance:**
- Read operations: ~15.8µs (0.0158ms)
- Write operations: Sub-50ms
- Concurrent operations: 100+ simultaneous reads

**Production considerations:**
- Ensure the data directory exists and is writable
- Regular backups recommended for production
- Database file will grow with alert volume
- Consider log rotation for WAL files

#### MySQL Storage (Production Multi-Instance)
Scalable persistent storage with multi-instance support for high availability.

```yaml
storage:
  type: mysql
  mysql:
    primary:
      host: mysql.example.com
      port: 3306
      database: alert_bridge
      username: alert_bridge_user
      password: ${MYSQL_PASSWORD}
    replica:  # Optional: for read scaling
      host: mysql-replica.example.com
      port: 3306
      database: alert_bridge
      username: alert_bridge_reader
      password: ${MYSQL_REPLICA_PASSWORD}
    pool:
      max_open_conns: 25      # Maximum number of open connections
      max_idle_conns: 5       # Maximum number of idle connections
      conn_max_lifetime: 3m   # Maximum connection lifetime
      conn_max_idle_time: 1m  # Maximum idle time before closing
    timeout: 5s               # Query timeout
    parse_time: true          # Parse time values to time.Time
    charset: utf8mb4          # Character set
```

**Features:**
- Multi-instance deployment support (3+ concurrent instances)
- Optimistic locking prevents concurrent update conflicts
- Primary-replica support for read scaling
- Connection pool with configurable limits
- Automatic schema migrations
- Foreign key constraints and referential integrity
- JSON columns for flexible label/annotation storage

**Performance:**
- Read operations: < 100ms target (10K alerts)
- Write operations: < 200ms target
- Cross-instance visibility: < 1 second
- Concurrent instances: 3+ supported

**Production considerations:**
- Use MySQL 8.0+ or MariaDB 10.5+ with InnoDB engine
- Configure connection pool based on expected load
- Set up read replicas for scaling read operations
- Regular backups using mysqldump or physical backups
- Monitor connection pool metrics (wait count, in-use connections)
- Use separate credentials for primary (read-write) and replica (read-only)
- Enable slow query logging for queries > 100ms
- Consider partitioning for very large alert volumes

**Multi-instance deployment:**
```yaml
# Example Kubernetes deployment with 3 replicas
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alert-bridge
spec:
  replicas: 3  # Multiple instances supported with MySQL
  selector:
    matchLabels:
      app: alert-bridge
  template:
    metadata:
      labels:
        app: alert-bridge
    spec:
      containers:
      - name: alert-bridge
        image: alert-bridge:latest
        env:
        - name: MYSQL_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-credentials
              key: password
```

**Migration from SQLite to MySQL:**
1. Export data from SQLite using `.dump` command
2. Create MySQL database and user
3. Update configuration to use MySQL storage
4. Restart application (migrations run automatically)
5. Verify data integrity and performance

### Running the Application

1. Start the server:
```bash
./alert-bridge
```

Or with custom config path:
```bash
CONFIG_PATH=/path/to/config.yaml ./alert-bridge
```

2. The server will start and:
   - Initialize the storage backend (SQLite or memory)
   - Run database migrations (for SQLite)
   - Start the HTTP server on the configured port

3. Verify it's running:
```bash
curl http://localhost:8080/health
```

## API Endpoints

### Health Check
```bash
GET /health
```

### Alertmanager Webhook
```bash
POST /alertmanager/webhook
Content-Type: application/json

# Receives alerts from Alertmanager
```

### Slack Interaction
```bash
POST /slack/interaction
Content-Type: application/x-www-form-urlencoded

# Handles button clicks and actions from Slack
```

### Slack Events
```bash
POST /slack/events
Content-Type: application/json

# Receives Slack events
```

### PagerDuty Webhook
```bash
POST /pagerduty/webhook
Content-Type: application/json

# Receives incident updates from PagerDuty
```

## Development

### Project Structure

```
alert-bridge/
├── cmd/alert-bridge/          # Application entry point
├── config/                    # Configuration files
├── internal/
│   ├── adapter/              # HTTP handlers, DTOs
│   ├── domain/               # Business entities and interfaces
│   ├── infrastructure/       # External integrations
│   │   ├── config/          # Configuration loading
│   │   ├── persistence/     # Storage implementations
│   │   │   ├── memory/      # In-memory storage
│   │   │   └── sqlite/      # SQLite storage
│   │   ├── slack/           # Slack client
│   │   └── pagerduty/       # PagerDuty client
│   └── usecase/             # Business logic
└── specs/                    # Feature specifications
```

### Running Tests

Run all tests:
```bash
go test ./...
```

Run SQLite-specific tests:
```bash
go test -v ./internal/infrastructure/persistence/sqlite/...
```

Run integration tests:
```bash
go test -v ./internal/infrastructure/persistence/sqlite/... -run Integration
```

Run benchmarks:
```bash
go test -bench=. ./internal/infrastructure/persistence/sqlite/...
```

### SQLite Database Management

#### View Database Schema
```bash
sqlite3 ./data/alert-bridge.db ".schema"
```

#### Query Alerts
```bash
sqlite3 ./data/alert-bridge.db "SELECT id, name, state FROM alerts;"
```

#### Backup Database
```bash
# Create a backup
sqlite3 ./data/alert-bridge.db ".backup ./data/alert-bridge-backup.db"

# Or use standard file copy (when app is stopped)
cp ./data/alert-bridge.db ./data/alert-bridge-backup.db
```

#### Compact Database
```bash
sqlite3 ./data/alert-bridge.db "VACUUM;"
```

### MySQL Database Management

#### Create Database and User
```bash
# Connect to MySQL
mysql -u root -p

# Create database
CREATE DATABASE alert_bridge CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

# Create user with appropriate privileges
CREATE USER 'alert_bridge_user'@'%' IDENTIFIED BY 'secure_password';
GRANT ALL PRIVILEGES ON alert_bridge.* TO 'alert_bridge_user'@'%';

# Optional: Create read-only user for replicas
CREATE USER 'alert_bridge_reader'@'%' IDENTIFIED BY 'reader_password';
GRANT SELECT ON alert_bridge.* TO 'alert_bridge_reader'@'%';

FLUSH PRIVILEGES;
```

#### View Database Schema
```bash
mysql -u alert_bridge_user -p alert_bridge -e "SHOW TABLES;"
mysql -u alert_bridge_user -p alert_bridge -e "DESCRIBE alerts;"
mysql -u alert_bridge_user -p alert_bridge -e "DESCRIBE ack_events;"
mysql -u alert_bridge_user -p alert_bridge -e "DESCRIBE silences;"
```

#### Query Alerts
```bash
# View all active alerts
mysql -u alert_bridge_user -p alert_bridge -e \
  "SELECT id, name, state, severity FROM alerts WHERE state = 'firing';"

# View recent acknowledgments
mysql -u alert_bridge_user -p alert_bridge -e \
  "SELECT alert_id, source, acknowledged_by_name, acknowledged_at
   FROM ack_events ORDER BY acknowledged_at DESC LIMIT 10;"

# View active silences
mysql -u alert_bridge_user -p alert_bridge -e \
  "SELECT id, instance, fingerprint, start_at, end_at
   FROM silences WHERE start_at <= NOW() AND end_at > NOW();"
```

#### Backup Database
```bash
# Create a backup using mysqldump
mysqldump -u alert_bridge_user -p alert_bridge > alert-bridge-backup.sql

# Create compressed backup
mysqldump -u alert_bridge_user -p alert_bridge | gzip > alert-bridge-backup-$(date +%Y%m%d).sql.gz

# Backup specific tables
mysqldump -u alert_bridge_user -p alert_bridge alerts ack_events silences > backup.sql
```

#### Restore Database
```bash
# Restore from backup
mysql -u alert_bridge_user -p alert_bridge < alert-bridge-backup.sql

# Restore from compressed backup
gunzip < alert-bridge-backup-20250101.sql.gz | mysql -u alert_bridge_user -p alert_bridge
```

#### Monitor Connection Pool
```bash
# View current connections
mysql -u root -p -e "SHOW PROCESSLIST;"

# View connection statistics
mysql -u root -p -e "SHOW STATUS LIKE 'Threads%';"
mysql -u root -p -e "SHOW STATUS LIKE 'Connections';"
mysql -u root -p -e "SHOW STATUS LIKE 'Max_used_connections';"

# View table statistics
mysql -u alert_bridge_user -p alert_bridge -e \
  "SELECT COUNT(*) as total_alerts FROM alerts;"
mysql -u alert_bridge_user -p alert_bridge -e \
  "SELECT COUNT(*) as total_acks FROM ack_events;"
```

#### Clean Up Old Data
```bash
# Delete old resolved alerts (older than 30 days)
mysql -u alert_bridge_user -p alert_bridge -e \
  "DELETE FROM alerts WHERE state = 'resolved'
   AND updated_at < DATE_SUB(NOW(), INTERVAL 30 DAY);"

# Delete expired silences
mysql -u alert_bridge_user -p alert_bridge -e \
  "DELETE FROM silences WHERE end_at < NOW();"

# Optimize tables after cleanup
mysql -u alert_bridge_user -p alert_bridge -e "OPTIMIZE TABLE alerts;"
mysql -u alert_bridge_user -p alert_bridge -e "OPTIMIZE TABLE ack_events;"
mysql -u alert_bridge_user -p alert_bridge -e "OPTIMIZE TABLE silences;"
```

#### Troubleshooting MySQL

**"too many connections" error**
- Increase max_connections in MySQL configuration
- Reduce max_open_conns in application config
- Check for connection leaks using SHOW PROCESSLIST

**"Lock wait timeout exceeded" error**
- Long-running transactions blocking updates
- Check for deadlocks: `SHOW ENGINE INNODB STATUS;`
- Reduce transaction size or duration

**Slow queries**
- Enable slow query log: `SET GLOBAL slow_query_log = 'ON';`
- Set threshold: `SET GLOBAL long_query_time = 0.1;` (100ms)
- Analyze: `mysqldumpslow /var/log/mysql/slow-query.log`
- Add indexes if needed (migrations handle this automatically)

**Replication lag** (if using replicas)
- Check replication status: `SHOW SLAVE STATUS\G`
- Monitor Seconds_Behind_Master
- Increase replica resources if consistently lagging

## Architecture

Alert Bridge follows Clean Architecture principles:

- **Domain Layer**: Core business entities and repository interfaces
- **Use Case Layer**: Business logic for alert processing, ack sync, silence management
- **Infrastructure Layer**: External integrations (Slack, PagerDuty, SQLite)
- **Adapter Layer**: HTTP handlers, request/response mapping

## Deployment

### Docker

Build the Docker image:
```bash
docker build -t alert-bridge .
```

Run with Docker:
```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/config:/app/config \
  --name alert-bridge \
  alert-bridge
```

### Kubernetes

#### SQLite Deployment (Single Instance)

Example deployment with persistent volume for SQLite:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: alert-bridge-data
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alert-bridge
spec:
  replicas: 1  # SQLite supports single instance only
  selector:
    matchLabels:
      app: alert-bridge
  template:
    metadata:
      labels:
        app: alert-bridge
    spec:
      containers:
      - name: alert-bridge
        image: alert-bridge:latest
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: data
          mountPath: /app/data
        - name: config
          mountPath: /app/config
        env:
        - name: CONFIG_PATH
          value: /app/config/config.yaml
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: alert-bridge-data
      - name: config
        configMap:
          name: alert-bridge-config
```

**Important**: SQLite uses file-based locking, so only run **one instance** when using SQLite storage.

#### MySQL Deployment (Multi-Instance)

Example high-availability deployment with MySQL:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: mysql-credentials
type: Opaque
stringData:
  password: your-mysql-password-here
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: alert-bridge-config
data:
  config.yaml: |
    server:
      port: 8080
    storage:
      type: mysql
      mysql:
        primary:
          host: mysql-primary.database.svc.cluster.local
          port: 3306
          database: alert_bridge
          username: alert_bridge_user
          password: ${MYSQL_PASSWORD}
        pool:
          max_open_conns: 25
          max_idle_conns: 5
          conn_max_lifetime: 3m
    slack:
      enabled: true
      bot_token: ${SLACK_BOT_TOKEN}
      signing_secret: ${SLACK_SIGNING_SECRET}
      channel_id: ${SLACK_CHANNEL_ID}
    # ... other config ...
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alert-bridge
spec:
  replicas: 3  # Multiple instances supported with MySQL
  selector:
    matchLabels:
      app: alert-bridge
  template:
    metadata:
      labels:
        app: alert-bridge
    spec:
      containers:
      - name: alert-bridge
        image: alert-bridge:latest
        ports:
        - containerPort: 8080
        env:
        - name: CONFIG_PATH
          value: /app/config/config.yaml
        - name: MYSQL_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-credentials
              key: password
        - name: SLACK_BOT_TOKEN
          valueFrom:
            secretKeyRef:
              name: slack-credentials
              key: bot_token
        - name: SLACK_SIGNING_SECRET
          valueFrom:
            secretKeyRef:
              name: slack-credentials
              key: signing_secret
        - name: SLACK_CHANNEL_ID
          value: "C1234567890"
        volumeMounts:
        - name: config
          mountPath: /app/config
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: config
        configMap:
          name: alert-bridge-config
---
apiVersion: v1
kind: Service
metadata:
  name: alert-bridge
spec:
  selector:
    app: alert-bridge
  ports:
  - port: 8080
    targetPort: 8080
  type: LoadBalancer
```

**Production tips for MySQL deployment:**
- Use 3+ replicas for high availability
- Configure resource limits based on alert volume
- Use external MySQL service (RDS, Cloud SQL, etc.) for production
- Enable pod disruption budgets for controlled rollouts
- Monitor connection pool metrics via /metrics endpoint (if enabled)
- Use HorizontalPodAutoscaler based on CPU/memory for auto-scaling

## Environment Variables

You can use environment variables in the config file:

```yaml
slack:
  bot_token: ${SLACK_BOT_TOKEN}
  signing_secret: ${SLACK_SIGNING_SECRET}
```

Or override config path:
```bash
export CONFIG_PATH=/path/to/config.yaml
./alert-bridge
```

## Troubleshooting

### SQLite Database Issues

**"database is locked" error**
- Ensure only one instance is running (SQLite doesn't support multi-instance)
- Check for stale lock files in data directory
- Verify no other process is accessing the database
- Consider switching to MySQL for multi-instance deployments

**"no such table" error**
- Migrations failed to run on startup
- Check database file permissions (needs write access)
- Verify path in configuration exists
- Check application logs for migration errors

**Performance degradation**
- Run VACUUM to compact database: `sqlite3 ./data/alert-bridge.db "VACUUM;"`
- Check database file size (large files slow down operations)
- Consider archiving old resolved alerts
- Monitor WAL file size and checkpoint frequency

### MySQL Database Issues

**"too many connections" error**
- Increase max_connections in MySQL: `SET GLOBAL max_connections = 200;`
- Reduce max_open_conns in application config (default: 25)
- Check for connection leaks: `SHOW PROCESSLIST;`
- Monitor connection pool metrics

**"Lock wait timeout exceeded" error**
- Long-running transactions blocking updates
- Check for deadlocks: `SHOW ENGINE INNODB STATUS;`
- Verify READ COMMITTED isolation level is set
- Reduce transaction scope or duration

**"connection refused" or "can't connect" error**
- Verify MySQL server is running and accessible
- Check host/port in configuration
- Verify network connectivity from application to MySQL
- Check MySQL user permissions: `SHOW GRANTS FOR 'alert_bridge_user'@'%';`
- Verify firewall rules allow connection

**Slow query performance**
- Enable slow query log: `SET GLOBAL slow_query_log = 'ON';`
- Set threshold: `SET GLOBAL long_query_time = 0.1;` (100ms)
- Analyze slow queries: `mysqldumpslow /var/log/mysql/slow-query.log`
- Check if indexes exist (migrations create them automatically)
- Monitor table sizes and consider partitioning

**Replication lag** (if using primary-replica)
- Check replication status: `SHOW SLAVE STATUS\G`
- Monitor Seconds_Behind_Master metric
- Increase replica server resources
- Check network latency between primary and replica
- Verify binary log size and retention

**"Deadlock found when trying to get lock" error**
- InnoDB detected a deadlock and rolled back transaction
- Application will retry automatically
- If frequent, review transaction patterns
- Consider breaking large transactions into smaller ones

### Application Issues

**Cannot connect to Slack/PagerDuty**
- Verify API tokens are correct
- Check network connectivity
- Review application logs

**Alerts not persisting**
- Verify storage type is set to "sqlite"
- Check database path exists and is writable
- Review logs for migration errors

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Write tests
5. Submit a pull request

## License

[Add your license information here]

## Support

For issues and questions:
- GitHub Issues: [Add your issue tracker URL]
- Documentation: See `specs/` directory for detailed specifications

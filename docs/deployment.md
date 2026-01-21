# Deployment Guide

## Releases

Alert Bridge uses automated releases via GoReleaser. When a version tag (e.g., `v1.0.0`) is pushed, the CI/CD pipeline automatically:

- Builds binaries for Linux, macOS, and Windows (amd64/arm64)
- Creates multi-arch Docker images on ghcr.io
- Generates changelog and release notes
- Publishes to GitHub Releases

### Creating a Release

```bash
# Create and push a version tag
git tag v1.0.0
git push origin v1.0.0
```

### Downloading Binaries

Download pre-built binaries from [GitHub Releases](https://github.com/qj0r9j0vc2/alert-bridge/releases):

```bash
# Linux (amd64)
curl -LO https://github.com/qj0r9j0vc2/alert-bridge/releases/latest/download/alert-bridge_linux_amd64.tar.gz
tar xzf alert-bridge_linux_amd64.tar.gz

# macOS (Apple Silicon)
curl -LO https://github.com/qj0r9j0vc2/alert-bridge/releases/latest/download/alert-bridge_darwin_arm64.tar.gz
tar xzf alert-bridge_darwin_arm64.tar.gz

# Verify checksum
sha256sum -c checksums.txt
```

### Using Docker Images

```bash
# Pull the latest image
docker pull ghcr.io/qj0r9j0vc2/alert-bridge:latest

# Pull a specific version
docker pull ghcr.io/qj0r9j0vc2/alert-bridge:v1.0.0

# Run with config
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config:/app/config \
  ghcr.io/qj0r9j0vc2/alert-bridge:latest
```

## Docker

### Build Image

```bash
docker build -t alert-bridge .
```

### Run Container

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/config:/app/config \
  --name alert-bridge \
  alert-bridge
```

### With Environment Variables

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/config:/app/config \
  -e SLACK_BOT_TOKEN=xoxb-your-token \
  -e SLACK_SIGNING_SECRET=your-secret \
  -e SLACK_CHANNEL_ID=C1234567890 \
  --name alert-bridge \
  alert-bridge
```

## Kubernetes

### SQLite Deployment (Single Instance)

For single-instance deployments with SQLite storage.

**Important:** SQLite uses file-based locking, so only run **one instance** when using SQLite storage.

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
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: alert-bridge-data
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

### MySQL Deployment (Multi-Instance)

For high-availability deployments with MySQL storage.

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
kind: Secret
metadata:
  name: slack-credentials
type: Opaque
stringData:
  bot_token: xoxb-your-bot-token
  signing_secret: your-signing-secret
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
    pagerduty:
      enabled: false
    alerting:
      deduplication_window: 5m
      resend_interval: 30m
    logging:
      level: info
      format: json
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
            path: /ready
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

### Production Tips for MySQL Deployment

- Use 3+ replicas for high availability
- Configure resource limits based on alert volume
- Use external MySQL service (RDS, Cloud SQL, etc.) for production
- Enable pod disruption budgets for controlled rollouts
- Use HorizontalPodAutoscaler based on CPU/memory for auto-scaling

### ConfigMap for Configuration

Create a ConfigMap for your configuration:

```bash
kubectl create configmap alert-bridge-config \
  --from-file=config.yaml=./config/config.yaml
```

### Secrets for Sensitive Data

Create secrets for sensitive credentials:

```bash
# MySQL password
kubectl create secret generic mysql-credentials \
  --from-literal=password='your-mysql-password'

# Slack credentials
kubectl create secret generic slack-credentials \
  --from-literal=bot_token='xoxb-your-token' \
  --from-literal=signing_secret='your-secret'

# PagerDuty credentials (if using)
kubectl create secret generic pagerduty-credentials \
  --from-literal=api_token='your-api-token' \
  --from-literal=routing_key='your-routing-key'
```

## Production Considerations

### Resource Planning

- **SQLite deployments:** 64-128MB memory, 0.1-0.2 CPU cores
- **MySQL deployments:** 128-256MB memory per instance, 0.1-0.5 CPU cores
- Scale based on alert volume and query frequency

### High Availability

- Use MySQL storage for multi-instance deployments
- Deploy 3+ replicas across different availability zones
- Configure load balancer health checks
- Set up monitoring and alerting for the alert-bridge instances

### Backup Strategy

- **SQLite:** Regular file backups of database
- **MySQL:** Automated mysqldump or physical backups
- Test restore procedures regularly

### Monitoring

- Monitor application logs for errors
- Track HTTP endpoint response times
- Monitor database connection pool metrics
- Set up alerts for failed health checks

## Next Steps

- [Storage Options](storage.md) - Configure persistent storage
- [Troubleshooting](troubleshooting.md) - Common deployment issues
- [API Reference](api.md) - Configure webhooks

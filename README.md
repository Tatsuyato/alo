# ALO Backend API - Complete Guide

Enterprise-grade backend API for Kubernetes operations with comprehensive authentication, RBAC, manifest validation, queue processing, build/deploy pipeline, logging, and system monitoring.

## Table of Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Setup & Configuration](#setup--configuration)
- [Environment Variables](#environment-variables)
- [API Endpoints](#api-endpoints)
- [Quick Start Examples](#quick-start-examples)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)

## Features

### Security & Access Control
- **Authentication**: API Key and JWT Bearer Token support
- **RBAC**: Fine-grained role-based access control per principal
- **Namespace Isolation**: Per-user namespace restrictions
- **Rate Limiting**: Configurable per-user rate limiting
- **Auto-Initialization**: First-run setup with secure key generation

### Manifest Validation
- Block privileged containers
- Block hostPath volume mounts
- Enforce CPU/Memory resource limits
- Per-project policy enforcement

### Operations
- **Kubernetes Commands**: Create, edit, delete resources
- **Build & Deploy Pipeline**: Git clone → Docker build → Registry push → kubectl deploy
- **Queue System**: Asynchronous job processing with retries and rollback
- **Status Monitoring**: Pod metrics, logs streaming, cluster info
- **Database**: SQLite for projects, services, and metadata

### Data Model
- **Projects**: Git repos with namespace/resource policies
- **Services**: Image/replica configuration linked to projects
- **Jobs**: Async task queue with retry/rollback support

## Requirements

- Go 1.26+ (for building from source)
- kubectl configured and accessible
- Docker (for build/deploy jobs)
- SQLite (included)

## Installation

### Option 1: Build from Source

```bash
cd /path/to/alo
go build -o alo ./src
./alo
```

### Option 2: Using Make

```bash
make build
make run-dev
```

### Option 3: Docker

```bash
docker build -t alo-backend:latest .
docker run -p 8080:8080 -v ~/.kube:/root/.kube alo-backend:latest
```

## Setup & Configuration

### First-Run Auto-Initialization

On first startup, the system automatically:
1. Generates secure API keys
2. Creates JWT secret
3. Saves credentials to `.setup_credentials` file (600 permissions)
4. Makes keys immediately available

**Output:**
```
╔════════════════════════════════════════════════════════════╗
║          SYSTEM AUTO-INITIALIZED SUCCESSFULLY             ║
╠════════════════════════════════════════════════════════════╣
║ Credentials saved to: .setup_credentials                   ║
╠════════════════════════════════════════════════════════════╣
║ Admin API Key:     admin-XXXXXXXXXXXXXXXX                  ║
║ Deployer API Key:  deployer-XXXXXXXXXXXXXXXX               ║
╚════════════════════════════════════════════════════════════╝
```

### Manual Setup via API

If you want to customize setup:

```bash
# 1. Check setup status
curl -X GET http://localhost:8080/api/v1/setup/status

# Response includes setupKey for one-time configuration
{
  "ok": true,
  "isInitialized": false,
  "setupKey": "PazQ5RhyaUYVY1FkEJ-1VM-5-lyBKDbc313falYTus0=",
  "message": "system ready for initialization"
}

# 2. Configure system using setupKey
curl -X POST http://localhost:8080/api/v1/setup/config \
  -H "Content-Type: application/json" \
  -d '{
    "setupKey": "PazQ5RhyaUYVY1FkEJ-1VM-5-lyBKDbc313falYTus0=",
    "adminApiKey": "my-secure-admin-key-123",
    "deployerApiKey": "my-secure-deployer-key-456",
    "jwtSecret": "my-jwt-secret-789",
    "defaultNamespace": "default",
    "defaultCpuLimit": "1000m",
    "defaultMemoryLimit": "1024Mi",
    "defaultServiceAccount": "default",
    "enableInitialAdmin": true,
    "initialAdminProjectName": "demo-project",
    "initialAdminProjectRepo": "https://github.com/example/repo.git"
  }'
```

### Reset Setup

To reset and re-initialize:

```bash
rm -f .setup_credentials .setup_key alo.db
./alo  # Will auto-initialize again
```

## Environment Variables

### Startup Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `PORT` | 8080 | HTTP server port |
| `DB_PATH` | ./alo.db | SQLite database path |
| `API_KEY_ADMIN` | local-admin-key | Admin API key (override auto-gen) |
| `API_KEY_DEPLOYER` | local-deployer-key | Deployer API key (override auto-gen) |
| `JWT_SECRET` | (empty) | JWT signing secret (override auto-gen) |

### Initial Setup

| Variable | Default | Purpose |
|----------|---------|---------|
| `DEFAULT_NAMESPACE` | default | Default Kubernetes namespace |
| `DEFAULT_CPU_LIMIT` | 1000m | Default CPU resource limit |
| `DEFAULT_MEMORY_LIMIT` | 1024Mi | Default memory resource limit |
| `DEFAULT_SERVICE_ACCOUNT` | default | Default Kubernetes service account |
| `INITIAL_REPO_URL` | (none) | Repo to create initial project from |
| `INITIAL_PROJECT_NAME` | initial-project | Name of initial auto-created project |

### Examples

**Development Setup**
```bash
export PORT=8081
export DB_PATH=./dev.db
export DEFAULT_NAMESPACE=dev
export DEFAULT_CPU_LIMIT=500m
export DEFAULT_MEMORY_LIMIT=512Mi

go run ./src
```

**Production with Custom Keys**
```bash
export PORT=8080
export DB_PATH=/data/alo.db
export API_KEY_ADMIN=$(openssl rand -base64 32)
export API_KEY_DEPLOYER=$(openssl rand -base64 32)
export JWT_SECRET=$(openssl rand -base64 64)

./alo
```

**Using .env File**
```bash
# Create .env
cat > .env << EOF
PORT=8080
DB_PATH=./alo.db
DEFAULT_NAMESPACE=prod
DEFAULT_CPU_LIMIT=2000m
DEFAULT_MEMORY_LIMIT=2048Mi
EOF

# Load and run
set -a && source .env && set +a && go run ./src
```

## API Endpoints

### Health & Setup

#### GET /healthz
System health check including environment, Kubernetes, and database status.

**Response:**
```json
{
  "ok": true,
  "service": "alo-backend",
  "timestamp": "2024-01-01T12:00:00Z",
  "environment": {
    "PORT": "8080",
    "DB_PATH": "./alo.db",
    "API_KEY_ADMIN": "set",
    "API_KEY_DEPLOYER": "set",
    "JWT_SECRET": "set"
  },
  "kubernetes": {
    "kubectl_available": true,
    "kubectl_version": "Client Version: v1.28.0",
    "cluster": {
      "available": true,
      "cluster_info": "Kubernetes control plane is running...",
      "current_context": "docker-desktop",
      "node_count": 1
    }
  },
  "database": {
    "healthy": true
  }
}
```

#### GET /api/v1/setup/status
Check system initialization status and get setup key.

**Response (not initialized):**
```json
{
  "ok": true,
  "isInitialized": false,
  "setupKey": "PazQ5RhyaUYVY1FkEJ-1VM-5-lyBKDbc313falYTus0=",
  "message": "system ready for initialization",
  "createdAt": "2024-01-01T12:00:00Z"
}
```

#### POST /api/v1/setup/config
Initialize or reconfigure the system. Requires valid setupKey.

**Request:**
```json
{
  "setupKey": "PazQ5RhyaUYVY1FkEJ-1VM-5-lyBKDbc313falYTus0=",
  "adminApiKey": "my-admin-key",
  "deployerApiKey": "my-deployer-key",
  "jwtSecret": "my-jwt-secret",
  "defaultNamespace": "prod",
  "defaultCpuLimit": "2000m",
  "defaultMemoryLimit": "2048Mi",
  "defaultServiceAccount": "deployer-sa",
  "enableInitialAdmin": true,
  "initialAdminProjectName": "main-app",
  "initialAdminProjectRepo": "https://github.com/org/repo.git"
}
```

### Projects

#### POST /api/v1/projects
Create a new project with namespace and resource policies.

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/projects \
  -H "X-API-Key: admin-XXXXXXXX" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "backend-api",
    "repo": "https://github.com/org/backend.git",
    "environment": "prod",
    "namespace": "prod",
    "cpuLimit": "2000m",
    "memoryLimit": "2048Mi",
    "serviceAccount": "deployer-sa"
  }'
```

**Response:**
```json
{
  "ok": true,
  "project": {
    "id": 1,
    "name": "backend-api",
    "repo": "https://github.com/org/backend.git",
    "environment": "prod",
    "namespace": "prod",
    "cpuLimit": "2000m",
    "memoryLimit": "2048Mi",
    "serviceAccount": "deployer-sa",
    "createdAt": "2024-01-01T12:00:00Z"
  }
}
```

#### GET /api/v1/projects
List all projects.

**Request:**
```bash
curl -X GET http://localhost:8080/api/v1/projects \
  -H "X-API-Key: admin-XXXXXXXX"
```

### Services

#### POST /api/v1/services
Create a service linked to a project.

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/services \
  -H "X-API-Key: admin-XXXXXXXX" \
  -H "Content-Type: application/json" \
  -d '{
    "projectId": 1,
    "name": "api-server",
    "image": "registry.example.com/org/api-server",
    "replicas": 3
  }'
```

#### GET /api/v1/services?projectId=1
List services for a specific project.

### Commands

#### POST /api/v1/commands
Execute kubectl commands asynchronously (create/edit/delete).

**Create Deployment:**
```bash
curl -X POST http://localhost:8080/api/v1/commands \
  -H "X-API-Key: admin-XXXXXXXX" \
  -H "Content-Type: application/json" \
  -d '{
    "projectId": 1,
    "action": "create",
    "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api-server\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app: api-server\n  template:\n    metadata:\n      labels:\n        app: api-server\n    spec:\n      containers:\n      - name: api-server\n        image: nginx:1.27\n        resources:\n          limits:\n            cpu: 500m\n            memory: 256Mi"
  }'
```

**Response:**
```json
{
  "ok": true,
  "jobId": "job-1704110400000000000",
  "message": "job queued"
}
```

**Delete Resource:**
```bash
curl -X POST http://localhost:8080/api/v1/commands \
  -H "X-API-Key: admin-XXXXXXXX" \
  -H "Content-Type: application/json" \
  -d '{
    "projectId": 1,
    "action": "delete",
    "kind": "deployment",
    "name": "api-server",
    "namespace": "prod"
  }'
```

**Dry Run (test without executing):**
```bash
curl -X POST http://localhost:8080/api/v1/commands \
  -H "X-API-Key: admin-XXXXXXXX" \
  -H "Content-Type: application/json" \
  -d '{
    "projectId": 1,
    "action": "delete",
    "kind": "deployment",
    "name": "api-server",
    "dryRun": true
  }'
```

### Jobs

#### GET /api/v1/jobs/{jobId}
Check job status.

**Request:**
```bash
curl -X GET "http://localhost:8080/api/v1/jobs/job-1704110400000000000" \
  -H "X-API-Key: admin-XXXXXXXX"
```

**Response:**
```json
{
  "ok": true,
  "job": {
    "id": "job-1704110400000000000",
    "type": "k8s_command",
    "status": "done",
    "attempts": 1,
    "maxRetries": 2,
    "output": "deployment.apps/api-server configured",
    "createdAt": "2024-01-01T12:00:00Z",
    "updatedAt": "2024-01-01T12:00:05Z"
  }
}
```

### Build & Deploy

#### POST /api/v1/builds
Trigger build and deploy pipeline.

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/builds \
  -H "X-API-Key: admin-XXXXXXXX" \
  -H "Content-Type: application/json" \
  -d '{
    "projectId": 1,
    "serviceId": 1,
    "gitRef": "main",
    "tag": "v1.2.3"
  }'
```

**Response:**
```json
{
  "ok": true,
  "jobId": "job-1704110400000000001",
  "message": "build/deploy queued"
}
```

Pipeline steps:
1. Git clone with specified ref
2. Docker build (tag: registry/image:tag)
3. Docker push to registry
4. kubectl set image deployment/service
5. Auto-rollback on failure

### Logs

#### GET /api/v1/logs?namespace=prod&service=api-server&tail=100
Stream pod logs for a service.

**Request:**
```bash
curl -X GET "http://localhost:8080/api/v1/logs?namespace=prod&service=api-server&tail=100" \
  -H "X-API-Key: admin-XXXXXXXX"
```

**Response:**
```json
{
  "ok": true,
  "namespace": "prod",
  "service": "api-server",
  "logs": "2024-01-01T12:00:00Z Started server on port 8080..."
}
```

### Status

#### GET /api/v1/status?namespace=prod&service=api-server
Get pod status and metrics for a service.

**Request:**
```bash
curl -X GET "http://localhost:8080/api/v1/status?namespace=prod&service=api-server" \
  -H "X-API-Key: admin-XXXXXXXX"
```

### Machine Status

#### GET /api/v1/machine/status
Get backend machine information.

**Request:**
```bash
curl -X GET http://localhost:8080/api/v1/machine/status \
  -H "X-API-Key: admin-XXXXXXXX"
```

**Response:**
```json
{
  "ok": true,
  "hostname": "prod-server-01",
  "os": "linux",
  "arch": "amd64",
  "cpuCount": 8,
  "goVersion": "go1.26.2",
  "loadAvg": "1.40 1.67 1.89 8/1420 31561",
  "uptime": "up 47 minutes",
  "dockerInfo": "Docker version 29.3.1, build c2be9cc",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

## Quick Start Examples

### 1. Initialize System
```bash
# Start server (auto-initializes)
./alo

# Check generated credentials
cat .setup_credentials
```

### 2. Create Project
```bash
ADMIN_KEY=$(grep "ADMIN_API_KEY=" .setup_credentials | cut -d= -f2)

curl -X POST http://localhost:8080/api/v1/projects \
  -H "X-API-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "repo": "https://github.com/myorg/myapp.git",
    "environment": "dev",
    "namespace": "default",
    "cpuLimit": "1000m",
    "memoryLimit": "1024Mi"
  }'
```

### 3. Create Service
```bash
curl -X POST http://localhost:8080/api/v1/services \
  -H "X-API-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "projectId": 1,
    "name": "web",
    "image": "myrepo/myapp",
    "replicas": 1
  }'
```

### 4. Deploy Manifest
```bash
curl -X POST http://localhost:8080/api/v1/commands \
  -H "X-API-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d @deployment.json
```

### 5. Check Job Status
```bash
curl -X GET "http://localhost:8080/api/v1/jobs/job-XXXXX" \
  -H "X-API-Key: $ADMIN_KEY"
```

## Security Considerations

### API Keys
- Store in secure vault (not git)
- Rotate regularly
- Use different keys per environment
- Disable unused keys

### RBAC
- Admin key has all permissions
- Deployer key restricted to specific namespace
- Use narrowest permissions needed
- Audit access via logs

### Manifest Validation
- Always enforces CPU/Memory limits
- Blocks privileged containers
- Blocks hostPath volumes
- Define per-project policies

### JWT
- Optional but recommended for production
- Use strong secrets (min 32 bytes)
- Short expiration times
- Rotate frequently

### Database
- Keep database file secure (600 permissions)
- Regular backups
- Encrypt at rest in production

## Troubleshooting

### Port Already in Use
```bash
# Find process
lsof -i :8080

# Use different port
PORT=9000 ./alo
```

### kubectl Not Found
```bash
# Verify kubectl
which kubectl
kubectl version --client

# Update PATH if needed
export PATH=$PATH:/usr/local/bin
```

### Manifest Validation Failures
```bash
# Check manifest compliance
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - resources:
          limits:
            cpu: "500m"          # Required and under policy max
            memory: "256Mi"      # Required and under policy max
```

### Rate Limit Exceeded
- Wait before retrying
- Check configured limits
- Use different credentials for parallel requests

### Database Locked
```bash
# Remove lock file
rm -f alo.db-*

# Restart
./alo
```

### Credentials Lost
```bash
# Regenerate
rm -f .setup_credentials .setup_key alo.db
./alo  # Auto-initialize again
```

## Support & Contributing

For issues, questions, or contributions, please open an issue in the repository.
- RBAC permissions per principal
- Namespace-level access control per principal
- Manifest policy validation:
  - block privileged containers
  - block hostPath volumes
  - enforce cpu/memory limit ceilings by project
- Rate limiting per authenticated subject

## Data model (SQLite)

- `projects`: repo, env, namespace, resource limits, service account
- `services`: image and replica settings per project

## Run

```bash
go run ./src
```

Or use make (see Makefile for convenient commands).

Environment variables:

- `PORT` (default `8080`)
- `DB_PATH` (default `./alo.db`)
- `API_KEY_ADMIN` (default `local-admin-key`)
- `API_KEY_DEPLOYER` (default `local-deployer-key`)
- `JWT_SECRET` (optional, enables JWT validation)

## API endpoints

- `GET /healthz`
- `GET /api/v1/machine/status`
- `POST /api/v1/projects`
- `GET /api/v1/projects`
- `POST /api/v1/services`
- `GET /api/v1/services?projectId=1`
- `POST /api/v1/commands`
- `POST /api/v1/builds`
- `GET /api/v1/jobs/{jobId}`
- `GET /api/v1/logs?namespace=ns-a&service=api&tail=100`
- `GET /api/v1/status?namespace=ns-a&service=api`

## Example auth header

```bash
-H "X-API-Key: local-admin-key"
```

## Example flow

1. Create project with namespace/resource policy
2. Create service linked to project
3. Submit command/build request (queued)
4. Poll job endpoint for status

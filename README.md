# ALO Backend API

Backend API for Kubernetes operations with authentication, RBAC, manifest validation, queue worker, build/deploy pipeline, logs, and status endpoints.

## Security features

- Authentication via `X-API-Key` or JWT bearer token
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
go run .
```

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

# Deployment Guide

## Prerequisites

- **Docker** 24+ and **Docker Compose** 2+
- **Go** 1.26+ (for local development)
- **Node.js** 22+ and **pnpm** (for frontend development)
- **PostgreSQL** 16+ (if running without Docker)

## Local Development (Docker Compose)

```bash
# Clone
git clone <repo-url>
cd Power-Distribution-Fault-Detection-System

# Start all services
docker compose up --build

# Access
# Frontend: http://localhost:8080
# API: http://localhost:8080/api/tickets
# Simulator: http://localhost:8080/sim/faults
```

### Services

| Service | Port | Health Check |
|---------|------|--------------|
| Nginx (Frontend + Proxy) | 8080 | `GET /healthz` → API |
| API (Go) | 8080 (internal) | `GET /healthz` |
| Simulator | 8081 (internal) | `GET /healthz` |
| PostgreSQL | 5432 | `pg_isready` |
| Seed | - | Runs once, exits |

### Stopping

```bash
docker compose down          # Stop, keep volumes
docker compose down -v       # Stop, remove volumes (clean slate)
```

## Environment Variables

Create `.env` from `.env.example`:

```bash
cp .env.example .env
```

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | `postgres://postgres:postgres@postgres:5432/kspdb?sslmode=disable` | PostgreSQL connection |
| `API_PORT` | No | `8080` | API listen port |
| `SIMULATOR_URL` | No | `http://simulator:8081` | Simulator base URL |
| `SIM_PORT` | No | `8081` | Simulator listen port |
| `CLOCK_MULTIPLIER` | No | `30` | Sim time / wall time ratio |
| `API_URL` | No | `http://api:8080` | API URL for simulator |

### `.env.example`

```env
DATABASE_URL=postgres://postgres:postgres@postgres:5432/kspdb?sslmode=disable
API_PORT=8080
SIMULATOR_URL=http://simulator:8081
SIM_PORT=8081
CLOCK_MULTIPLIER=30
API_URL=http://api:8080
```

## Railway Deployment

Railway does **not** use `docker-compose.yml` directly. Deploy as **two separate services** sharing a PostgreSQL database.

### Option 1: Single Container (Recommended for Simplicity)

Build a combined image that runs API + Simulator + Nginx via supervisor, but Railway prefers single-process containers. **Better: deploy as two services.**

### Option 2: Two Services (Recommended)

#### 1. Create Railway Project

1. Go to [railway.app](https://railway.app) → New Project
2. Add **PostgreSQL** database service
3. Note the `DATABASE_URL` from PostgreSQL variables

#### 2. Deploy API Service

1. New Service → **GitHub Repo** → Select this repo
2. Configure:
   - **Build Command**: `docker build --target api-final -t api .`
   - **Start Command**: `./api`
   - **Port**: `8080`
3. Add Environment Variables:
   ```
   DATABASE_URL=${{Postgres.DATABASE_URL}}
   SIMULATOR_URL=https://<simulator-service>.railway.app
   API_PORT=8080
   ```
4. Deploy → Get API URL (e.g., `https://api-production-xxxx.up.railway.app`)

#### 3. Deploy Simulator Service

1. New Service → **GitHub Repo** → Same repo
2. Configure:
   - **Build Command**: `docker build --target simulator-final -t simulator .`
   - **Start Command**: `./simulator`
   - **Port**: `8081`
3. Add Environment Variables:
   ```
   DATABASE_URL=${{Postgres.DATABASE_URL}}
   API_URL=https://<api-service>.railway.app
   SIM_PORT=8081
   CLOCK_MULTIPLIER=30
   ```
4. Deploy → Get Simulator URL (e.g., `https://simulator-production-xxxx.up.railway.app`)

#### 4. Update API's SIMULATOR_URL

Go to API service variables → Update `SIMULATOR_URL` to the Simulator service URL → Redeploy API.

#### 5. Add Nginx (Optional)

For single-domain access with proxying, add a third Nginx service or use Railway's built-in routing. Or access API/Simulator directly via their Railway URLs.

### Railway Dockerfile Targets

The `Dockerfile` has multi-stage targets:

```dockerfile
# For API service
docker build --target api-final -t api .

# For Simulator service
docker build --target simulator-final -t simulator .

# For Seed (one-time)
docker build --target seed-final -t seed .
```

## Frontend on Railway

The frontend is **built into the API image** (`--target api-final` copies `frontend/dist` to `/app/static`). Nginx is not needed separately - the API serves static files with SPA fallback.

Access frontend at: `https://<api-service>.railway.app`

## Verification Checklist

After deployment:

- [ ] `GET https://<api-url>/healthz` → `{"status":"ok"}`
- [ ] `GET https://<api-url>/api/tickets` → `[]` (or tickets)
- [ ] `GET https://<sim-url>/sim/topology/tree` → network data
- [ ] Open `https://<api-url>/` → Dashboard loads
- [ ] Inject fault via `POST /sim/faults` → ticket appears in `/api/tickets`
- [ ] Open `https://<api-url>/map?fault=T-XXX` → Map centers on fault

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `docker compose up` fails on `pnpm install` | Node version mismatch | Use `node:22-alpine` in Dockerfile |
| API can't connect to DB | `DATABASE_URL` wrong | Check Railway Postgres variables, use internal hostname |
| Simulator can't reach API | `API_URL` wrong | Use Railway internal URL (`http://api.railway.internal:8080`) or public URL |
| Clock multiplier not applied | API didn't fetch clock | Check `SIMULATOR_URL` accessible from API |
| Frontend shows "Failed to load topology" | Nginx proxy misconfig | Check `/sim/` and `/clock` proxy to simulator |
| Tickets not appearing | Detection window too long | At 30x, 60 sim secs = 2 wall secs. Wait 10-15s after fault |
| `pnpm run build` fails in Docker | Missing `CI=true` | Added to Dockerfile frontend-builder stage |
| Seed exits with code 1 | DB not ready | Seed depends on `postgres:service_healthy` |
| Port 8080 already in use | Local conflict | `docker compose down` first, or change port in `.env` |

## Clean Reset

```bash
# Local
docker compose down -v
docker compose up --build

# Railway
# Delete services and recreate, or use Railway CLI:
railway service delete <service-name>
```

## Resource Requirements (Railway)

| Service | Memory | CPU |
|---------|--------|-----|
| PostgreSQL | 512 MB | 0.5 vCPU |
| API | 256 MB | 0.25 vCPU |
| Simulator | 256 MB | 0.25 vCPU |
| **Total** | **~1 GB** | **1 vCPU** |

Fits comfortably in Railway's **Hobby** plan ($5/month).

## Local Development Without Docker

```bash
# Terminal 1: PostgreSQL
docker run -d --name pg -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:16-alpine

# Terminal 2: Seed
DATABASE_URL=postgres://postgres:postgres@localhost:5432/kspdb?sslmode=disable \
go run ./cmd/generator --seed-db --export-csv --pole-count 3000

# Terminal 3: API
DATABASE_URL=postgres://postgres:postgres@localhost:5432/kspdb?sslmode=disable \
SIMULATOR_URL=http://localhost:8081 \
go run ./cmd/api

# Terminal 4: Simulator
DATABASE_URL=postgres://postgres:postgres@localhost:5432/kspdb?sslmode=disable \
API_URL=http://localhost:8080 \
go run ./cmd/simulator

# Terminal 5: Frontend
cd frontend && pnpm install && pnpm run dev
# Opens http://localhost:5173 (proxies /sim, /clock to :8081)
```
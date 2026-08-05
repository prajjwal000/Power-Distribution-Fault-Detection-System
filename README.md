# Power Distribution Fault Detection System

A real-time fault detection and localization system for power distribution networks. Built for the KSPDB assignment.

## Quick Start

```bash
# Clone and start
git clone <repo-url>
cd Power-Distribution-Fault-Detection-System
docker compose up --build

# Open in browser
open http://localhost:8080
```

**One command start** - `docker compose up --build` brings up the entire stack:
- **PostgreSQL** - Database with synthetic Bangalore network (3000+ poles, 42 DTs, 6 feeders)
- **Seed** - Generates network topology and seeds database
- **API (Go)** - Ingestion, detection engine, ticket management, SSE
- **Simulator (Go)** - Virtual clock, fault injection, realistic telemetry
- **Nginx** - Reverse proxy serving frontend + routing to API/Simulator

## Public Demo

- **Live URL**: https://your-deployment-url.com (see DEPLOYMENT.md)
- **Demo Video**: [5-min walkthrough](https://your-video-link.com)

## System Overview

```
┌─────────────┐    HTTP POST     ┌──────────┐    SSE/REST    ┌─────────────┐
│  Simulator  │ ───────────────▶ │   API    │ ────────────▶  │  Frontend   │
│ (Port 8081) │   /ingest        │ (Port    │   /tickets     │ (Nginx)     │
│             │    + /sim/*      │  8080)   │   /tickets/    │ (Port 8080) │
└─────────────┘                  └──────────┘   stream        └─────────────┘
                                      │
                                      ▼
                              ┌──────────────┐
                              │  PostgreSQL  │
                              │  (topology,  │
                              │   tickets)   │
                              └──────────────┘
```

## Pages

| Route | Page | Purpose |
|-------|------|---------|
| `/` | **Dashboard** | Real-time ticket table, SSE updates, detail modal, deep-link to Map |
| `/map` | **Map** | Leaflet geographic map, asset layers, fault overlay, URL deep-linking |
| `/fault-injector` | **Fault Injector** | Interactive ground-truth tree, inject/repair faults, noise injection |

## Key Features

- **Temporal buffering** - 60 sim-second window (2s wall at 30x) handles out-of-order events from clock skew (±90s) and radio delay (0-48s)
- **Two-path localization** - Known topology (40%) → span-level; Unknown (60%) → DT-level with lower confidence
- **Geographic inference** - MST + radial ordering for missing topology, 88.9% edge accuracy on synthetic test data
- **Real-time SSE** - Live ticket updates (created/refined/verified) without polling
- **Auto-verification** - Tickets auto-verify when all affected poles restore
- **Deep linking** - Dashboard "View on Map" → `/map?fault=T-XXX` auto-pans to fault

## Testing

```bash
# Run all tests
go test ./...

# Frontend typecheck
cd frontend && pnpm run typecheck
```

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for:
- Data flow diagram (Mermaid)
- Ingestion & deduplication
- Localization algorithm (known/unknown topology)
- Confidence scoring
- API surface
- UI reasoning

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for:
- Prerequisites
- Local Docker Compose
- Railway deployment (single container)
- Environment variables
- Troubleshooting

## Decisions

See [DECISIONS.md](DECISIONS.md) for design decisions and trade-offs.

## AI Workflow

See [AI-WORKFLOW.md](AI-WORKFLOW.md) for AI-assisted development process.
# Architecture

## Overview

Power distribution fault detection system for Karnataka State Power Distribution Board. Detects and localizes faults on LT distribution lines using pole-level telemetry from IoT devices.

## Services

| Service | Stack | Purpose |
|---------|-------|---------|
| **API** | Go (Gin/Echo) + PostgreSQL | Telemetry ingest, fault detection, ticket lifecycle, REST API |
| **Frontend** | Vite + React + TypeScript + shadcn/ui | Operator dashboard (`/`) and fault simulator UI (`/simulator`) |
| **Simulator** | Go | Injects faults by generating realistic telemetry, reads ground truth to determine which poles go dark |
| **PostgreSQL** | 16-alpine | Stores network topology, telemetry, tickets |
| **Seed** | Go (one-shot) | Generates synthetic network data on startup, exits after seeding |

## Data Model

Two-layer storage: **ground truth** (complete topology, simulator-only) and **registry** (incomplete, what the system sees).

### Ground Truth (`gt_topology`)

Complete parent-child relationships, branch points, sequence numbers. Used by simulator to inject faults. The API service has no access to this table.

### Registry (`poles`, `transformers`, `feeders`, `substations`)

Matches real-world data quality:
- 60% of DTs have no recorded pole ordering (`seq_on_line`, `parent_pole_id` are NULL)
- 9% of poles have no device fitted (`device_id` is NULL)
- 3% of poles have no PIN code

### Network Hierarchy

```
Substation → Feeder → Distribution Transformer → Poles (radial tree with branches)
```

Feeders have lat/lon (centroid of downstream DTs) to support feeder-level fault localization.

## Fault Simulator Design

The simulator reads ground truth topology, determines which poles go dark for a given fault (span, DT, or feeder), then POSTs telemetry events to the API's ingest endpoint. The API has no idea whether telemetry came from real devices or the simulator.

## Deployment

Single `docker compose up` brings up all services. Seed service runs once and exits. API starts after seed completes. Deployed to Railway with managed PostgreSQL.

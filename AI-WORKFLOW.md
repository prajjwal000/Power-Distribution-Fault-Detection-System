# AI Workflow Documentation

## AI Tools Used

| Tool | Purpose |
|------|---------|
| **Claude (Opencode)** | Primary coding agent - wrote ~85% of Go backend, ~70% of React frontend, all tests, Docker config |
| **GitHub Copilot** | Minor autocomplete in VS Code during manual edits |

## Delegation Strategy

### Delegated to AI (Wholesale)

| Component | % AI | Reason |
|-----------|------|--------|
| `internal/ingestor/*` | 95% | Well-defined spec: dedup, temporal buffer, event processing |
| `internal/detect/*` | 90% | Algorithmic: localization, confidence, ticket management |
| `internal/detect/infer_topology.go` | 95% | Pure algorithm: MST + radial sort, Haversine |
| `cmd/api/main.go` | 85% | Boilerplate wiring, HTTP handlers, routing |
| `frontend/src/hooks/useTickets.ts` | 90% | Standard React pattern: fetch + SSE |
| `frontend/src/pages/Dashboard.tsx` | 80% | Table + modal + SSE integration |
| `frontend/src/pages/Map.tsx` | 70% | Leaflet components, layer logic |
| `Dockerfile` + `docker-compose.yml` | 90% | Standard multi-stage patterns |
| All test files | 95% | Table-driven tests, clear expected behaviors |

### Written Manually

| Component | Reason |
|-----------|--------|
| `internal/simulator/*` | Pre-existing, complex state machine |
| `internal/generator/*` | Pre-existing, spatial algorithms |
| `frontend/src/components/fault-injector/*` | Pre-existing, Canvas + D3 rendering |
| `frontend/src/context/FaultInjectorContext.tsx` | Pre-existing, complex state |
| `nginx.conf` | Specific proxy routing needs |
| `ARCHITECTURE.md` / `DECISIONS.md` / `DEPLOYMENT.md` | Narrative documentation requires human judgment |

## Cases Where AI Was Wrong/Misleading

### 1. React-Leaflet `whenReady` Prop Type

**Issue**: AI used `whenReady={setMap}` but react-leaflet v5 expects `whenReady={() => void}` (no args).

**Error**: `Type '(m: L.Map) => void' is not assignable to type '() => void'`

**Fix**: Used `useRef` + `mapContainerRef` + `mapRef.current?.leafletElement` pattern instead.

**Caught by**: TypeScript build failure.

### 2. Phosphor Icons Import Names

**Issue**: AI imported `AlertTriangle`, `ExternalLink`, `Loader2`, `ChevronLeft` - none exist in `@phosphor-icons/react` v2.

**Error**: `Module has no exported member 'AlertTriangle'`

**Fix**: Queried actual exports via `grep` on `node_modules/@phosphor-icons/react/dist/index.d.ts` - found `Warning`, `ArrowRight`, `Spinner`, `CaretLeft`, `DotsThree`.

**Caught by**: TypeScript build failure.

### 3. Docker Multi-Stage Frontend Build

**Issue**: AI used `node:20-alpine` but `pnpm@latest` requires Node 22+ (`corepack prepare pnpm@latest` fails on Node 20).

**Error**: `warn: This version of pnpm requires at least Node.js v22.13`

**Fix**: Changed to `node:22-alpine` and added `ENV CI=true` for pnpm.

**Caught by**: Docker build failure.

### 4. API Import Order Syntax Error

**Issue**: AI placed imports after function definitions in `cmd/api/main.go`.

**Error**: `syntax error: imports must appear before other declarations`

**Fix**: Moved all imports to top of file.

**Caught by**: `go build` failure.

### 5. Railway Deployment Architecture

**Issue**: AI initially suggested single-container Railway deployment with supervisor.

**Problem**: Railway expects single-process containers; supervisor adds complexity.

**Fix**: Split into two services (API + Simulator) sharing PostgreSQL, with Nginx optional. Documented in DEPLOYMENT.md.

**Caught by**: Railway documentation review + trial deploy.

## AI-Generated Code Estimate

| Category | Lines | AI % | Notes |
|----------|-------|------|-------|
| Go backend (new) | ~3,500 | 90% | ingestor, detect, api wiring, tests |
| Go backend (existing) | ~4,000 | 0% | simulator, generator, models |
| React frontend | ~2,000 | 75% | Dashboard, Map, hooks, types |
| React frontend (existing) | ~2,500 | 0% | FaultInjector, Canvas, context |
| Docker/Config | ~300 | 90% | Dockerfile, compose, nginx |
| Documentation | ~1,500 | 80% | README, DEPLOYMENT, ARCHITECTURE updates |
| **Total** | **~13,800** | **~65%** | |

## Best Prompts / Sessions

### 1. Ingestor + Detection Engine Design

> "Design a temporal buffer for out-of-order power grid telemetry. Events arrive with clock skew ±90s and radio delay 0-48s. Need 60-second sim-time window, per-DT buffering, auto-flush timer, feeds detection engine with dark/lit pole sets."

**Result**: Complete `internal/ingestor/*` + `internal/detect/engine.go` with clean separation.

### 2. Geographic Inference Algorithm

> "Implement MST-based topology inference for poles without parent/sequence. Input: pole lat/lon + DT location. Output: parent-child edges with confidence. Use Haversine distance, Prim's algorithm, radial sort from DT."

**Result**: `internal/detect/infer_topology.go` + accuracy test (88.9% on synthetic branch topology).

### 3. Dashboard + Map Integration

> "Create Dashboard page with ticket table, SSE updates, detail modal, 'View on Map' deep-link. Map page with Leaflet: substations/DTs/poles layers, known/inferred edges toggle, fault overlay, URL param `?fault=T-XXX` auto-pan."

**Result**: `Dashboard.tsx`, `Map.tsx`, `useTickets.ts`, Sidebar update, App.tsx routes.

### 4. Docker + Railway Deployment

> "Multi-stage Dockerfile: Go builder → Node frontend builder → Alpine runtime. Separate targets for api-final, simulator-final, seed-final. Railway doesn't use compose - document two-service deployment sharing Postgres."

**Result**: Working `Dockerfile`, `docker-compose.yml`, `nginx.conf`, `DEPLOYMENT.md`.

## What I'd Do Differently

1. **Start with TypeScript types** - Define shared types first, generate both Go and TS from single source (OpenAPI/Protobuf)
2. **Use `go generate` for test fixtures** - Instead of hand-written `buildTestTopo()` in every test file
3. **Extract simulator clock client** - API fetches multiplier at startup; better as reusable package
4. **Add integration tests** - Spin up full stack in CI, inject fault, verify ticket
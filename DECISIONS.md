# Decisions

## 2026-08-05 — Dashboard + Map + Geographic Inference

Added the Operations Dashboard and Geographic Map pages to complete the operator UI:

- **Dashboard (`/`)**: Ticket table with real-time SSE updates, detail modal with evidence, acknowledge/resolve actions, deep-link to Map.
- **Map (`/map`)**: Leaflet-based geographic map with toggleable layers (substations, feeders, DTs, poles, known/inferred topology), fault overlay, collapsible asset sidebar, URL deep-linking (`?fault=T-XXX`).
- **Geographic inference for missing topology**: MST + radial ordering algorithm infers pole parent-child relationships from coordinates for the 60% of DTs without `seq_on_line`. Accuracy measured at 88.9% on synthetic test data with branch topology. Lower confidence (0.3–0.9) reported for inferred edges. Algorithm documented in ARCHITECTURE.md with failure modes.
- **Docker integration**: Frontend built into API container via multi-stage Dockerfile (Go builder + Node builder + Alpine runtime). Single `docker compose up` brings up everything. API serves static files from `./static` with SPA fallback routing.
- **API endpoints added**: `/tickets`, `/tickets/stream` (SSE), `/tickets/:id` (PATCH), `/network/inferred-topology`, `/stats`.

## 2026-08-05 — Ingestor and detection engine architecture

Built the core data pipeline: `internal/ingestor/` handles event processing, dedup, and temporal buffering; `internal/detect/` handles localization and ticket management.

Key design decisions:
- **Per-DT temporal buffer with 60s wall-clock delay**: Events arrive out of order due to clock skew (±90s) and radio delay (0-48s). Waiting 60s wall-clock (1800s at 30x sim speed) ensures all events for a fault are collected before localization runs.
- **Two-path localization**: Known topology (40% of DTs) gets span-level localization using `seq_on_line` ordering. Unknown topology (60% of DTs) gets DT-level reporting with lower confidence.
- **In-memory only, no DB persistence**: Tickets exist in the engine's map. Acceptable for the assignment since the simulator can re-inject faults.
- **SSE for real-time ticket updates**: The API's `/tickets/stream` endpoint broadcasts ticket lifecycle events to the frontend. No polling needed.
- **Confidence scoring**: Base 0.7 (known topology) or 0.5 (unknown), boosted by reporting completeness and topology certainty. Refined +0.15 after 15 minutes of additional data.

## 2026-08-05 — Restoration stagger realism

On repair, each device's `power_restored` is delayed by a random 0–20 sim seconds (per spec "typically within 20 seconds"), both `boot` and `power_restored` go through the delivery queue with radio delay. This creates a realistic cascade of restoration events rather than a synchronized burst.

## 2026-08-05 — Noise injection and scheduled outages

`POST /sim/noise` supports three kinds: `device_death` (stops heartbeats, power stays on), `duplicate` (re-emit same seq), `stale_replay` (old `power_lost` with past timestamp). `GET /scheduled-outages` returns a deterministic mock feed (feeder/DT scopes, 20–40 min overruns, ~10% cancellations). The simulator honors outage windows by darkening the scope's devices with normal `power_lost` telemetry and restoring after. The future API will use the feed to suppress tickets.

## 2026-08-05 — Frontend event batching for SSE performance

Steady state at 30× is ~100 events/wall-second; a fault burst is hundreds of events in seconds. The original per-event `setState` caused hundreds of React re-renders + full canvas redraws per second. `useSimulatorEvents` now buffers incoming SSE messages and flushes `poleStates` at ~10 Hz in a single `setState`, while `lastEvent` (which drives pulse animations) is updated per batch. This eliminates the burst performance cliff and ensures no pulses are dropped due to React batching.

## 2026-08-05 — Simulator owns the virtual clock; the API is a clock client

Time is the crux of the whole demo: scheduled-outage windows, fault timing, and "how long ago was this detected" all need one shared time base. The simulator owns it — `sim_time` advances at `multiplier ×` wall time (default 30×), and the API reads `GET /sim/clock` instead of keeping its own clock. This makes runtime multiplier changes in the UI safe: with only one clock, the two services cannot drift.

## 2026-08-05 — Simulator models clock skew and radio quality per device

Real events arrive out of order: two poles that lose power in the same sim instant may report minutes apart, and the downstream pole can arrive first. The simulator models this directly — each device gets a fixed clock skew (±90 s) baked into `ts` and an RSSI-derived transmission delay — and emits events through a min-heap keyed by scheduled wall-clock time. Consequence: the API's ingestion and detection code must tolerate out-of-order arrival; it cannot assume the first dark pole is the fault location.

## 2026-08-04 — Transform and collapsed state persist in sessionStorage; selection does not

Collapsed node IDs and canvas zoom/pan transform are stored in sessionStorage and restored on mount. This means the operator's view (what they expanded, where they were zoomed) is stable if they switch to the dashboard and back. Selection is kept in component state — it resets when navigating away, which is the correct behavior (selection is transient interaction state, not persistent view state).

## 2026-08-04 — Canvas state is imperative and decoupled from React rendering

The canvas uses refs for all values that must be fresh on every frame (`transformRef`, `nodesRef`, `linksRef`, `selectionRef`). A plain `doRender()` function reads from these refs directly. Event handlers update refs and call `doRender()` synchronously — no dependency on React's render cycle. `FaultInjectorProvider` owns all React state (collapsedIds, transform, selection) and reports changes upward via callbacks. This avoids stale-closure bugs where React's batching or concurrent rendering caused the canvas to render with an outdated transform.

## 2026-08-04 — Fault injector canvas uses D3 for layout, HTML Canvas for rendering

The tree layout is computed by `d3.tree()`; the canvas renders nodes and edges directly via the Canvas 2D API. This separates layout concerns (handled by D3) from rendering (handled by Canvas), avoiding SVG DOM overhead at scale and giving full control over hit detection. The `_children` stash pattern (standard D3 collapse/expand) is used: setting `node.children = undefined` removes the subtree from `d3.tree()`'s layout pass, causing remaining nodes to reflow into freed space — no reserved layout space for collapsed subtrees.

## 2026-08-03 — Simulator injects faults through the same telemetry ingest endpoint as real devices

The simulator does not call a special internal API to report faults. It computes which poles go dark for a given poles, then POSTs `power_lost` and `power_restored` events to `/ingest` with realistic timing, sequence numbers, firmware variants, and delivery failures. The API cannot distinguish simulated telemetry from real telemetry. This was chosen because the brief requires the system to handle real-world telemetry as-is; the simulator is more realistic if it obeys the same delivery guarantees (and failures) as real hardware.

## 2026-08-03 — Simulator has omniscient ground-truth view; API has no access to it

The system has two completely separate data stores: `gt_topology` (complete parent-child edges, branch points, sequence numbers — simulator-only) and the registry (`poles`, `transformers`, `feeders` — what the API and operator see). The fault injector is built on top of ground truth so it can target any span, DT, or feeder by ID, and so the injection stats (expected dark poles vs observed reports) are meaningful for demonstrating the telemetry model. The API never queries `gt_topology`; it works only from registry data and telemetry. This keeps the simulator's advantage honest — it can pick any fault target, but the detection itself must work from what the system actually knows.



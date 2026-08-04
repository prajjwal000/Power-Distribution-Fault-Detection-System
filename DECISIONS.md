# Decisions

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

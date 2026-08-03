# Decisions

## 2026-08-03 — Simulator injects faults through the same telemetry ingest endpoint as real devices

The simulator does not call a special internal API to report faults. It computes which poles go dark for a given poles, then POSTs `power_lost` and `power_restored` events to `/ingest` with realistic timing, sequence numbers, firmware variants, and delivery failures. The API cannot distinguish simulated telemetry from real telemetry. This was chosen because the brief requires the system to handle real-world telemetry as-is; the simulator is more realistic if it obeys the same delivery guarantees (and failures) as real hardware.

## 2026-08-03 — Simulator has omniscient ground-truth view; API has no access to it

The system has two completely separate data stores: `gt_topology` (complete parent-child edges, branch points, sequence numbers — simulator-only) and the registry (`poles`, `transformers`, `feeders` — what the API and operator see). The fault injector is built on top of ground truth so it can target any span, DT, or feeder by ID, and so the injection stats (expected dark poles vs observed reports) are meaningful for demonstrating the telemetry model. The API never queries `gt_topology`; it works only from registry data and telemetry. This keeps the simulator's advantage honest — it can pick any fault target, but the detection itself must work from what the system actually knows.

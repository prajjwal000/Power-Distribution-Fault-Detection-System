DROP TABLE IF EXISTS poles CASCADE;
DROP TABLE IF EXISTS gt_topology CASCADE;
DROP TABLE IF EXISTS dt_topology_status CASCADE;
DROP TABLE IF EXISTS transformers CASCADE;
DROP TABLE IF EXISTS feeders CASCADE;
DROP TABLE IF EXISTS substations CASCADE;

CREATE TABLE IF NOT EXISTS substations (
    id TEXT PRIMARY KEY,
    lat FLOAT NOT NULL,
    lon FLOAT NOT NULL
);

CREATE TABLE IF NOT EXISTS feeders (
    id TEXT PRIMARY KEY,
    substation_id TEXT NOT NULL REFERENCES substations(id),
    name TEXT NOT NULL,
    lat FLOAT NOT NULL,
    lon FLOAT NOT NULL
);

CREATE TABLE IF NOT EXISTS transformers (
    id TEXT PRIMARY KEY,
    feeder_id TEXT NOT NULL REFERENCES feeders(id),
    lat FLOAT NOT NULL,
    lon FLOAT NOT NULL,
    capacity_kva INT NOT NULL,
    households_served INT NOT NULL
);

CREATE TABLE IF NOT EXISTS dt_topology_status (
    dt_id TEXT PRIMARY KEY REFERENCES transformers(id),
    has_topology BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS gt_topology (
    pole_id TEXT PRIMARY KEY,
    parent_pole_id TEXT,
    dt_id TEXT NOT NULL REFERENCES transformers(id),
    seq_on_line INT NOT NULL,
    children TEXT[],
    is_branch_point BOOLEAN NOT NULL DEFAULT false,
    lat FLOAT NOT NULL,
    lon FLOAT NOT NULL
);

CREATE TABLE IF NOT EXISTS poles (
    id TEXT PRIMARY KEY,
    dt_id TEXT NOT NULL REFERENCES transformers(id),
    feeder_id TEXT NOT NULL REFERENCES feeders(id),
    lat FLOAT NOT NULL,
    lon FLOAT NOT NULL,
    seq_on_line INT,
    parent_pole_id TEXT,
    pole_type TEXT NOT NULL,
    ward TEXT NOT NULL,
    pincode TEXT,
    device_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_poles_dt_id ON poles(dt_id);
CREATE INDEX IF NOT EXISTS idx_poles_feeder_id ON poles(feeder_id);
CREATE INDEX IF NOT EXISTS idx_gt_topology_dt_id ON gt_topology(dt_id);
CREATE INDEX IF NOT EXISTS idx_gt_topology_parent ON gt_topology(parent_pole_id);

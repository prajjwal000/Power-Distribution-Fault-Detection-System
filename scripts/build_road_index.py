#!/usr/bin/env python3
"""
Download Bangalore road data from Overpass API and build a spatial grid index
for fast nearest-road queries during network generation.

Usage:
    python3 scripts/build_road_index.py

Outputs:
    data/road_segments.json  - Full road segment geometry
    data/road_index.json     - Grid-based spatial index
"""

import json
import math
import os
import sys
import time
from collections import defaultdict

import requests

# Bangalore bounding box (covers the full metro area)
BBOX = (12.84, 77.51, 13.06, 77.70)  # south, west, north, east

# Grid cell size in degrees (~200m at Bangalore latitude)
CELL_SIZE_LAT = 0.0018  # ~200m
CELL_SIZE_LON = 0.0022  # ~200m at 13°N

# Road types to include (all of them for realistic pole placement)
ROAD_TYPES = {"trunk", "primary", "secondary", "tertiary", "residential", "unclassified",
              "tertiary_link", "secondary_link", "primary_link", "trunk_link",
              "living_street", "pedestrian", "road"}

OVERPASS_URL = "https://overpass-api.de/api/interpreter"
HEADERS = {"User-Agent": "PowerFaultDetector/1.0 (generator script)"}
DATA_DIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "data")


def download_roads():
    """Download road data from Overpass API."""
    query = f"""
    [out:json][timeout:120];
    (
      way["highway"]({BBOX[0]},{BBOX[1]},{BBOX[2]},{BBOX[3]});
    );
    out body;
    >;
    out skel qt;
    """
    print("Downloading road data from Overpass API...")
    resp = requests.get(OVERPASS_URL, params={"data": query}, headers=HEADERS, timeout=180)
    resp.raise_for_status()
    data = resp.json()
    elements = data.get("elements", [])
    print(f"  Received {len(elements)} elements")
    return elements


def build_node_lookup(elements):
    """Build a lookup from node ID to coordinates."""
    nodes = {}
    for e in elements:
        if e["type"] == "node":
            nodes[e["id"]] = (e["lat"], e["lon"])
    print(f"  {len(nodes)} unique nodes")
    return nodes


def extract_ways(elements, node_lookup):
    """Extract road ways with geometry, filtering to known road types."""
    ways = []
    for e in elements:
        if e["type"] != "way":
            continue
        tags = e.get("tags", {})
        highway = tags.get("highway", "")
        if highway not in ROAD_TYPES:
            continue

        node_ids = e.get("nodes", [])
        geometry = []
        for nid in node_ids:
            if nid in node_lookup:
                lat, lon = node_lookup[nid]
                geometry.append({"lat": lat, "lon": lon})

        if len(geometry) < 2:
            continue

        ways.append({
            "id": e["id"],
            "highway": highway,
            "name": tags.get("name", ""),
            "geometry": geometry,
        })

    print(f"  {len(ways)} road segments after filtering")
    return ways


def assign_to_cells(segments):
    """Assign road segments to grid cells for spatial indexing."""
    cells = defaultdict(list)
    for idx, seg in enumerate(segments):
        cells_seen = set()
        for pt in seg["geometry"]:
            ci = int(math.floor(pt["lat"] / CELL_SIZE_LAT))
            cj = int(math.floor(pt["lon"] / CELL_SIZE_LON))
            cell_key = (ci, cj)
            if cell_key not in cells_seen:
                cells[cell_key].append(idx)
                cells_seen.add(cell_key)
    return dict(cells)


def main():
    os.makedirs(DATA_DIR, exist_ok=True)

    # Check if we already have the data
    segments_path = os.path.join(DATA_DIR, "road_segments.json")
    index_path = os.path.join(DATA_DIR, "road_index.json")

    if os.path.exists(segments_path) and os.path.exists(index_path):
        print("Road data already exists. Delete files to re-download.")
        return

    elements = download_roads()
    node_lookup = build_node_lookup(elements)
    segments = extract_ways(elements, node_lookup)

    print("Building spatial index...")
    cells = assign_to_cells(segments)

    # Convert cell keys to strings for JSON
    cells_json = {}
    for (ci, cj), seg_indices in cells.items():
        cells_json[f"{ci},{cj}"] = seg_indices

    print(f"  {len(cells_json)} grid cells")

    # Write outputs
    with open(segments_path, "w") as f:
        json.dump(segments, f, separators=(",", ":"))
    print(f"Wrote {segments_path} ({os.path.getsize(segments_path) / 1024:.0f} KB)")

    index_data = {
        "cell_size_lat": CELL_SIZE_LAT,
        "cell_size_lon": CELL_SIZE_LON,
        "cells": cells_json,
    }
    with open(index_path, "w") as f:
        json.dump(index_data, f, separators=(",", ":"))
    print(f"Wrote {index_path} ({os.path.getsize(index_path) / 1024:.0f} KB)")

    print("Done.")


if __name__ == "__main__":
    main()

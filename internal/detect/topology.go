package detect

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Pole struct {
	ID          string
	DTID        string
	FeederID    string
	ParentID    *string
	SeqOnLine   *int
	DeviceID    *string
	Pincode     *string
	Lat         float64
	Lon         float64
}

type Transformer struct {
	ID               string
	FeederID         string
	Lat              float64
	Lon              float64
	CapacityKVA      int
	HouseholdsServed int
}

type Feeder struct {
	ID           string
	SubstationID string
	Name         string
}

type TopologyIndex struct {
	PoleByID        map[string]*Pole
	Children        map[string][]string
	Parent          map[string]string
	DTIDToPoles     map[string][]string
	DeviceToPole    map[string]*Pole
	HasTopology     map[string]bool
	TransformerByID map[string]*Transformer
	FeederByID      map[string]*Feeder
	FeederToSub     map[string]string
}

func LoadTopology(ctx context.Context, conn *pgx.Conn) (*TopologyIndex, error) {
	idx := &TopologyIndex{
		PoleByID:        make(map[string]*Pole),
		Children:        make(map[string][]string),
		Parent:          make(map[string]string),
		DTIDToPoles:     make(map[string][]string),
		DeviceToPole:    make(map[string]*Pole),
		HasTopology:     make(map[string]bool),
		TransformerByID: make(map[string]*Transformer),
		FeederByID:      make(map[string]*Feeder),
		FeederToSub:     make(map[string]string),
	}

	if err := idx.loadFeeders(ctx, conn); err != nil {
		return nil, fmt.Errorf("load feeders: %w", err)
	}
	if err := idx.loadTransformers(ctx, conn); err != nil {
		return nil, fmt.Errorf("load transformers: %w", err)
	}
	if err := idx.loadPoles(ctx, conn); err != nil {
		return nil, fmt.Errorf("load poles: %w", err)
	}
	if err := idx.loadDTTopologyStatus(ctx, conn); err != nil {
		return nil, fmt.Errorf("load dt topology status: %w", err)
	}
	idx.buildChildren()

	return idx, nil
}

func (idx *TopologyIndex) loadFeeders(ctx context.Context, conn *pgx.Conn) error {
	rows, err := conn.Query(ctx, "SELECT id, substation_id, name FROM feeders")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var f Feeder
		if err := rows.Scan(&f.ID, &f.SubstationID, &f.Name); err != nil {
			return err
		}
		idx.FeederByID[f.ID] = &f
		idx.FeederToSub[f.ID] = f.SubstationID
	}
	return rows.Err()
}

func (idx *TopologyIndex) loadTransformers(ctx context.Context, conn *pgx.Conn) error {
	rows, err := conn.Query(ctx, "SELECT id, feeder_id, lat, lon, capacity_kva, households_served FROM transformers")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var t Transformer
		if err := rows.Scan(&t.ID, &t.FeederID, &t.Lat, &t.Lon, &t.CapacityKVA, &t.HouseholdsServed); err != nil {
			return err
		}
		idx.TransformerByID[t.ID] = &t
	}
	return rows.Err()
}

func (idx *TopologyIndex) loadPoles(ctx context.Context, conn *pgx.Conn) error {
	rows, err := conn.Query(ctx, "SELECT id, dt_id, feeder_id, parent_pole_id, seq_on_line, device_id, pincode, lat, lon FROM poles")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var p Pole
		if err := rows.Scan(&p.ID, &p.DTID, &p.FeederID, &p.ParentID, &p.SeqOnLine, &p.DeviceID, &p.Pincode, &p.Lat, &p.Lon); err != nil {
			return err
		}
		idx.PoleByID[p.ID] = &p
		idx.DTIDToPoles[p.DTID] = append(idx.DTIDToPoles[p.DTID], p.ID)
		if p.DeviceID != nil {
			idx.DeviceToPole[*p.DeviceID] = &p
		}
		if p.ParentID != nil {
			idx.Parent[p.ID] = *p.ParentID
		}
	}
	return rows.Err()
}

func (idx *TopologyIndex) loadDTTopologyStatus(ctx context.Context, conn *pgx.Conn) error {
	rows, err := conn.Query(ctx, "SELECT dt_id, has_topology FROM dt_topology_status")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var dtID string
		var has bool
		if err := rows.Scan(&dtID, &has); err != nil {
			return err
		}
		idx.HasTopology[dtID] = has
	}
	return rows.Err()
}

func (idx *TopologyIndex) buildChildren() {
	for poleID, parentID := range idx.Parent {
		idx.Children[parentID] = append(idx.Children[parentID], poleID)
	}
}

func (idx *TopologyIndex) HasKnownTopology(dtID string) bool {
	if idx.HasTopology[dtID] {
		return true
	}
	// Fallback: check if any pole in this DT has a sequence number (GT topology)
	for _, pid := range idx.DTIDToPoles[dtID] {
		if p, ok := idx.PoleByID[pid]; ok && p.SeqOnLine != nil {
			return true
		}
	}
	return false
}

func (idx *TopologyIndex) PolesForDT(dtID string) []string {
	return idx.DTIDToPoles[dtID]
}

func (idx *TopologyIndex) PoleHasDevice(poleID string) bool {
	p, ok := idx.PoleByID[poleID]
	if !ok {
		return false
	}
	return p.DeviceID != nil
}

func (idx *TopologyIndex) SeqOnLine(poleID string) (int, bool) {
	p, ok := idx.PoleByID[poleID]
	if !ok || p.SeqOnLine == nil {
		return 0, false
	}
	return *p.SeqOnLine, true
}

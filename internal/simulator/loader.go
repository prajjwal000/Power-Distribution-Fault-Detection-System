package simulator

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/jackc/pgx/v5/pgxpool"
	"power-fault-detector/internal/model"
)

func LoadFromDB(ctx context.Context, pool *pgxpool.Pool) (*SimulatorState, error) {
	st := NewSimulatorState()

	if err := loadSubstations(ctx, pool, st); err != nil {
		return nil, fmt.Errorf("load substations: %w", err)
	}
	if err := loadFeeders(ctx, pool, st); err != nil {
		return nil, fmt.Errorf("load feeders: %w", err)
	}
	if err := loadTransformers(ctx, pool, st); err != nil {
		return nil, fmt.Errorf("load transformers: %w", err)
	}
	if err := loadGTTopology(ctx, pool, st); err != nil {
		return nil, fmt.Errorf("load gt topology: %w", err)
	}
	if err := loadPoles(ctx, pool, st); err != nil {
		return nil, fmt.Errorf("load poles: %w", err)
	}

	st.BuildIndices()
	seedDevices(st)

	return st, nil
}

func (st *SimulatorState) BuildIndices() {
	for _, p := range st.GTPoles {
		st.Children[p.ID] = []string{}
		if p.ParentPoleID != nil {
			st.Parents[p.ID] = *p.ParentPoleID
			st.Children[*p.ParentPoleID] = append(st.Children[*p.ParentPoleID], p.ID)
		}
	}
	for _, p := range st.PoleByID {
		if p.DeviceID != nil && *p.DeviceID != "" {
			st.PoleByDevice[*p.DeviceID] = p
		}
	}
	st.TransformerByID = make(map[string]*model.Transformer, len(st.Transformers))
	for i := range st.Transformers {
		st.TransformerByID[st.Transformers[i].ID] = &st.Transformers[i]
	}
	st.FeederByID = make(map[string]*model.Feeder, len(st.Feeders))
	for i := range st.Feeders {
		st.FeederByID[st.Feeders[i].ID] = &st.Feeders[i]
	}
}

func seedDevices(st *SimulatorState) {
	rng := rand.New(rand.NewSource(42))
	for _, p := range st.PoleByID {
		if p.DeviceID == nil || *p.DeviceID == "" {
			continue
		}
		fw := "1.4.2"
		if rng.Float64() < 0.08 {
			fw = "1.2.0"
		}
		batteryMV := 3400 + rng.Intn(200)
		rssi := -70 - rng.Intn(30)
		clockSkew := int64(-90 + rng.Intn(181))
		rssiDeficit := abs(rssi) - 75
		if rssiDeficit < 0 {
			rssiDeficit = 0
		}
		radioDelay := int64(rssiDeficit * 2)
		st.Devices[*p.DeviceID] = &DeviceState{
			PoleID:            p.ID,
			DeviceID:          *p.DeviceID,
			Firmware:          fw,
			BatteryMV:         batteryMV,
			RSSI:              rssi,
			Seq:               0,
			Energized:         true,
			ClockSkewSecs:     clockSkew,
			RadioDelaySecs:    radioDelay,
			NextEmitSim:       0,
			KilledAtSim:       0,
			AutoResumeSimSecs: 0,
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func loadSubstations(ctx context.Context, pool *pgxpool.Pool, st *SimulatorState) error {
	rows, err := pool.Query(ctx, "SELECT id, lat, lon FROM substations")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var s model.Substation
		if err := rows.Scan(&s.ID, &s.Lat, &s.Lon); err != nil {
			return err
		}
		st.Substations = append(st.Substations, s)
	}
	return rows.Err()
}

func loadFeeders(ctx context.Context, pool *pgxpool.Pool, st *SimulatorState) error {
	rows, err := pool.Query(ctx, "SELECT id, substation_id, name, lat, lon FROM feeders")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var f model.Feeder
		if err := rows.Scan(&f.ID, &f.SubstationID, &f.Name, &f.Lat, &f.Lon); err != nil {
			return err
		}
		st.Feeders = append(st.Feeders, f)
	}
	return rows.Err()
}

func loadTransformers(ctx context.Context, pool *pgxpool.Pool, st *SimulatorState) error {
	rows, err := pool.Query(ctx, "SELECT id, feeder_id, lat, lon, capacity_kva, households_served FROM transformers")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var t model.Transformer
		if err := rows.Scan(&t.ID, &t.FeederID, &t.Lat, &t.Lon, &t.CapacityKVA, &t.HouseholdsServed); err != nil {
			return err
		}
		st.Transformers = append(st.Transformers, t)
	}
	return rows.Err()
}

func loadGTTopology(ctx context.Context, pool *pgxpool.Pool, st *SimulatorState) error {
	rows, err := pool.Query(ctx, "SELECT pole_id, parent_pole_id, dt_id, seq_on_line, is_branch_point, lat, lon FROM gt_topology ORDER BY seq_on_line")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p model.GTPole
		var parentID *string
		if err := rows.Scan(&p.ID, &parentID, &p.DTID, &p.SeqOnLine, &p.IsBranchPoint, &p.Lat, &p.Lon); err != nil {
			return err
		}
		p.ParentPoleID = parentID
		st.GTPoles = append(st.GTPoles, p)
	}
	return rows.Err()
}

func loadPoles(ctx context.Context, pool *pgxpool.Pool, st *SimulatorState) error {
	rows, err := pool.Query(ctx, "SELECT id, dt_id, feeder_id, lat, lon, seq_on_line, parent_pole_id, pole_type, ward, pincode, device_id FROM poles")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p model.Pole
		var seq *int
		var parentID, pincode, deviceID *string
		if err := rows.Scan(&p.ID, &p.DTID, &p.FeederID, &p.Lat, &p.Lon, &seq, &parentID, &p.PoleType, &p.Ward, &pincode, &deviceID); err != nil {
			return err
		}
		p.SeqOnLine = seq
		p.ParentPoleID = parentID
		p.Pincode = pincode
		p.DeviceID = deviceID
		st.PoleByID[p.ID] = &p
	}
	return rows.Err()
}

package generator

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"power-fault-detector/internal/model"
	"strconv"

	"github.com/jackc/pgx/v5"
)

func ExportCSV(net *GeneratedNetwork, registry *RegistryData, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	if err := exportTransformerCSV(net.Transformers, dir+"/transformer_registry.csv"); err != nil {
		return err
	}
	if err := exportPoleCSV(registry.Poles, dir+"/pole_registry.csv"); err != nil {
		return err
	}

	return nil
}

func exportTransformerCSV(transformers []model.Transformer, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // CSV close in defer, nothing to recover

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{"dt_id", "feeder_id", "lat", "lon", "capacity_kva", "households_served"})

	for _, dt := range transformers {
		_ = w.Write([]string{
			dt.ID,
			dt.FeederID,
			strconv.FormatFloat(dt.Lat, 'f', 6, 64),
			strconv.FormatFloat(dt.Lon, 'f', 6, 64),
			strconv.Itoa(dt.CapacityKVA),
			strconv.Itoa(dt.HouseholdsServed),
		})
	}

	return nil
}

func exportPoleCSV(poles []model.Pole, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // CSV close in defer, nothing to recover

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{"pole_id", "lat", "lon", "feeder_id", "dt_id", "seq_on_line", "parent_pole_id", "pole_type", "ward", "pincode", "device_id"})

	for _, p := range poles {
		seqStr := ""
		if p.SeqOnLine != nil {
			seqStr = strconv.Itoa(*p.SeqOnLine)
		}
		parentStr := ""
		if p.ParentPoleID != nil {
			parentStr = *p.ParentPoleID
		}
		pinStr := ""
		if p.Pincode != nil {
			pinStr = *p.Pincode
		}
		devStr := ""
		if p.DeviceID != nil {
			devStr = *p.DeviceID
		}

		_ = w.Write([]string{
			p.ID,
			strconv.FormatFloat(p.Lat, 'f', 6, 64),
			strconv.FormatFloat(p.Lon, 'f', 6, 64),
			p.FeederID,
			p.DTID,
			seqStr,
			parentStr,
			p.PoleType,
			p.Ward,
			pinStr,
			devStr,
		})
	}

	return nil
}

func SeedDB(ctx context.Context, dbURL string, net *GeneratedNetwork, registry *RegistryData) error {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect to db: %w", err)
	}
	defer conn.Close(ctx) //nolint:errcheck // pgx close in defer, nothing to recover

	schemaSQL, err := os.ReadFile("internal/db/schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}

	if _, err := conn.Exec(ctx, string(schemaSQL)); err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}

	if err := seedSubstations(ctx, conn, net.Substations); err != nil {
		return err
	}
	if err := seedFeeders(ctx, conn, net.Feeders); err != nil {
		return err
	}
	if err := seedTransformers(ctx, conn, net.Transformers); err != nil {
		return err
	}
	if err := seedDTTopologyStatus(ctx, conn, registry.DTTopologyStatus); err != nil {
		return err
	}
	if err := seedGTTopology(ctx, conn, net.GTPoles); err != nil {
		return err
	}
	if err := seedPoles(ctx, conn, registry.Poles); err != nil {
		return err
	}

	return nil
}

func seedSubstations(ctx context.Context, conn *pgx.Conn, subs []model.Substation) error {
	rows := make([][]any, len(subs))
	for i, s := range subs {
		rows[i] = []any{s.ID, s.Lat, s.Lon}
	}
	_, err := conn.CopyFrom(ctx, pgx.Identifier{"substations"}, []string{"id", "lat", "lon"}, pgx.CopyFromRows(rows))
	return err
}

func seedFeeders(ctx context.Context, conn *pgx.Conn, feeders []model.Feeder) error {
	rows := make([][]any, len(feeders))
	for i, f := range feeders {
		rows[i] = []any{f.ID, f.SubstationID, f.Name, f.Lat, f.Lon}
	}
	_, err := conn.CopyFrom(ctx, pgx.Identifier{"feeders"}, []string{"id", "substation_id", "name", "lat", "lon"}, pgx.CopyFromRows(rows))
	return err
}

func seedTransformers(ctx context.Context, conn *pgx.Conn, dts []model.Transformer) error {
	rows := make([][]any, len(dts))
	for i, dt := range dts {
		rows[i] = []any{dt.ID, dt.FeederID, dt.Lat, dt.Lon, dt.CapacityKVA, dt.HouseholdsServed}
	}
	_, err := conn.CopyFrom(ctx, pgx.Identifier{"transformers"}, []string{"id", "feeder_id", "lat", "lon", "capacity_kva", "households_served"}, pgx.CopyFromRows(rows))
	return err
}

func seedDTTopologyStatus(ctx context.Context, conn *pgx.Conn, statuses []model.DTTopologyStatus) error {
	rows := make([][]any, len(statuses))
	for i, s := range statuses {
		rows[i] = []any{s.DTID, s.HasTopology}
	}
	_, err := conn.CopyFrom(ctx, pgx.Identifier{"dt_topology_status"}, []string{"dt_id", "has_topology"}, pgx.CopyFromRows(rows))
	return err
}

func seedGTTopology(ctx context.Context, conn *pgx.Conn, poles []model.GTPole) error {
	for _, p := range poles {
		children := findChildren(poles, p.ID)
		_, err := conn.Exec(ctx,
			`INSERT INTO gt_topology (pole_id, parent_pole_id, dt_id, seq_on_line, children, is_branch_point, lat, lon)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			p.ID, p.ParentPoleID, p.DTID, p.SeqOnLine, children, p.IsBranchPoint, p.Lat, p.Lon)
		if err != nil {
			return fmt.Errorf("insert gt_topology %s: %w", p.ID, err)
		}
	}
	return nil
}

func seedPoles(ctx context.Context, conn *pgx.Conn, poles []model.Pole) error {
	rows := make([][]any, len(poles))
	for i := range poles {
		p := &poles[i]
		rows[i] = []any{
			p.ID, p.DTID, p.FeederID, p.Lat, p.Lon,
			p.SeqOnLine, p.ParentPoleID, p.PoleType, p.Ward, p.Pincode, p.DeviceID,
		}
	}
	_, err := conn.CopyFrom(ctx, pgx.Identifier{"poles"},
		[]string{"id", "dt_id", "feeder_id", "lat", "lon", "seq_on_line", "parent_pole_id", "pole_type", "ward", "pincode", "device_id"},
		pgx.CopyFromRows(rows))
	return err
}

func findChildren(poles []model.GTPole, parentID string) []string {
	var children []string
	for _, p := range poles {
		if p.ParentPoleID != nil && *p.ParentPoleID == parentID {
			children = append(children, p.ID)
		}
	}
	return children
}

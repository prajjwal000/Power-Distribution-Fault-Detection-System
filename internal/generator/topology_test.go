package generator

import (
	"math"
	"os"
	"power-fault-detector/internal/model"
	"strconv"
	"testing"
)

const defaultTestPoleCount = 500

func TestMain(m *testing.M) {
	_ = os.Setenv("GENERATOR_NO_ROADS", "1")
	os.Exit(m.Run())
}

func testConfig() Config {
	poleCount := defaultTestPoleCount
	if envCount := os.Getenv("TEST_POLE_COUNT"); envCount != "" {
		if count, err := strconv.Atoi(envCount); err == nil {
			poleCount = count
		}
	}
	return ConfigForPoleCount(poleCount)
}

func TestOneRootPerDT(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	rootCountByDT := make(map[string]int)
	for _, p := range net.GTPoles {
		if p.ParentPoleID == nil {
			rootCountByDT[p.DTID]++
		}
	}

	for _, dt := range net.Transformers {
		count := rootCountByDT[dt.ID]
		if count != 1 {
			t.Errorf("DT %s has %d root poles, expected 1", dt.ID, count)
		}
	}
}

func TestSingleParent(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	parentCount := make(map[string]int)
	for _, p := range net.GTPoles {
		if p.ParentPoleID != nil {
			parentCount[p.ID]++
		}
	}

	for _, p := range net.GTPoles {
		if p.ParentPoleID == nil {
			continue
		}
		if parentCount[p.ID] != 1 {
			t.Errorf("pole %s has %d parents, expected 1", p.ID, parentCount[p.ID])
		}
	}
}

func TestNoCycles(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	parentMap := make(map[string]*string)
	for _, p := range net.GTPoles {
		parentMap[p.ID] = p.ParentPoleID
	}

	for _, p := range net.GTPoles {
		visited := make(map[string]bool)
		current := p.ID
		for current != "" {
			if visited[current] {
				t.Errorf("cycle detected involving pole %s", p.ID)
				break
			}
			visited[current] = true
			parent := parentMap[current]
			if parent == nil {
				break
			}
			current = *parent
		}
	}
}

func TestTreeIsConnected(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	polesByDT := make(map[string][]string)
	parentMap := make(map[string]*string)
	for _, p := range net.GTPoles {
		polesByDT[p.DTID] = append(polesByDT[p.DTID], p.ID)
		parentMap[p.ID] = p.ParentPoleID
	}

	for dtID, poles := range polesByDT {
		var rootID string
		for _, pid := range poles {
			if parentMap[pid] == nil {
				rootID = pid
				break
			}
		}
		if rootID == "" {
			t.Errorf("DT %s has no root pole", dtID)
			continue
		}

		reachable := make(map[string]bool)
		queue := []string{rootID}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if reachable[current] {
				continue
			}
			reachable[current] = true
			for _, p := range net.GTPoles {
				if p.ParentPoleID != nil && *p.ParentPoleID == current {
					queue = append(queue, p.ID)
				}
			}
		}

		for _, pid := range poles {
			if !reachable[pid] {
				t.Errorf("pole %s under DT %s is not reachable from root", pid, dtID)
			}
		}
	}
}

func TestDTAffiliation(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	poleMap := make(map[string]string)
	for _, p := range net.GTPoles {
		poleMap[p.ID] = p.DTID
	}

	for _, p := range net.GTPoles {
		if p.ParentPoleID == nil {
			continue
		}
		parentDT, ok := poleMap[*p.ParentPoleID]
		if !ok {
			t.Errorf("pole %s has parent %s which doesn't exist", p.ID, *p.ParentPoleID)
			continue
		}
		if parentDT != p.DTID {
			t.Errorf("pole %s (DT=%s) has parent %s (DT=%s) - DT mismatch", p.ID, p.DTID, *p.ParentPoleID, parentDT)
		}
	}
}

func TestParentExists(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	poleIDs := make(map[string]bool)
	for _, p := range net.GTPoles {
		poleIDs[p.ID] = true
	}

	for _, p := range net.GTPoles {
		if p.ParentPoleID == nil {
			continue
		}
		if !poleIDs[*p.ParentPoleID] {
			t.Errorf("pole %s references non-existent parent %s", p.ID, *p.ParentPoleID)
		}
	}
}

func TestRootSeqIsOne(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	for _, p := range net.GTPoles {
		if p.ParentPoleID == nil && p.SeqOnLine != 1 {
			t.Errorf("root pole %s has seq %d, expected 1", p.ID, p.SeqOnLine)
		}
	}
}

func TestMonotonicSeq(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	poleMap := make(map[string]model.GTPole)
	for _, p := range net.GTPoles {
		poleMap[p.ID] = p
	}

	for _, p := range net.GTPoles {
		if p.ParentPoleID == nil {
			continue
		}
		parent, ok := poleMap[*p.ParentPoleID]
		if !ok {
			continue
		}
		if p.SeqOnLine <= parent.SeqOnLine {
			t.Errorf("pole %s (seq=%d) has parent %s (seq=%d) - not strictly increasing", p.ID, p.SeqOnLine, parent.ID, parent.SeqOnLine)
		}
	}
}

func TestBranchCountInRange(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	branchCountByDT := make(map[string]int)
	for _, p := range net.GTPoles {
		if p.IsBranchPoint {
			branchCountByDT[p.DTID]++
		}
	}

	for dtID, count := range branchCountByDT {
		if count < cfg.BranchesPerLine.Min || count > cfg.BranchesPerLine.Max {
			t.Errorf("DT %s has %d branches, expected %d-%d", dtID, count, cfg.BranchesPerLine.Min, cfg.BranchesPerLine.Max)
		}
	}
}

func TestBranchPointsMarked(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	parentMap := make(map[string][]string)
	for _, p := range net.GTPoles {
		if p.ParentPoleID != nil {
			parentMap[*p.ParentPoleID] = append(parentMap[*p.ParentPoleID], p.ID)
		}
	}

	poleMap := make(map[string]model.GTPole)
	for _, p := range net.GTPoles {
		poleMap[p.ID] = p
	}

	for parentID, children := range parentMap {
		if len(children) > 1 {
			pole := poleMap[parentID]
			if !pole.IsBranchPoint {
				t.Errorf("pole %s has %d children but is_branch_point=false", parentID, len(children))
			}
		}
	}
}

func TestPoleCountPerDTInRange(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	countByDT := make(map[string]int)
	for _, p := range net.GTPoles {
		countByDT[p.DTID]++
	}

	for dtID, count := range countByDT {
		if count < cfg.PolesPerDT.Min || count > cfg.PolesPerDT.Max {
			t.Errorf("DT %s has %d poles, expected %d-%d", dtID, count, cfg.PolesPerDT.Min, cfg.PolesPerDT.Max)
		}
	}
}

func TestTotalPoleCount(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	total := len(net.GTPoles)

	expectedDTs := cfg.SubstationCount * cfg.FeedersPerSub * ((cfg.DTsPerFeeder.Min + cfg.DTsPerFeeder.Max) / 2)
	expectedPoles := expectedDTs * ((cfg.PolesPerDT.Min + cfg.PolesPerDT.Max) / 2)

	tolerance := 0.3
	minExpected := int(float64(expectedPoles) * (1 - tolerance))
	maxExpected := int(float64(expectedPoles) * (1 + tolerance))

	if total < minExpected || total > maxExpected {
		t.Errorf("total poles = %d, expected %d-%d (based on config)", total, minExpected, maxExpected)
	}
}

func TestMissingTopologyRatio(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)
	registry := BuildRegistry(net, cfg)

	degradedCount := 0
	for _, s := range registry.DTTopologyStatus {
		if !s.HasTopology {
			degradedCount++
		}
	}

	total := len(registry.DTTopologyStatus)
	ratio := float64(degradedCount) / float64(total)
	expected := cfg.MissingTopologyPct
	tolerance := 0.25

	if math.Abs(ratio-expected) > tolerance {
		t.Errorf("missing topology ratio = %.2f, expected %.2f ± %.2f", ratio, expected, tolerance)
	}
}

func TestNoDeviceRatio(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)
	registry := BuildRegistry(net, cfg)

	noDeviceCount := 0
	for _, p := range registry.Poles {
		if p.DeviceID == nil {
			noDeviceCount++
		}
	}

	total := len(registry.Poles)
	ratio := float64(noDeviceCount) / float64(total)
	expected := cfg.NoDevicePct
	tolerance := 0.05

	if math.Abs(ratio-expected) > tolerance {
		t.Errorf("no-device ratio = %.2f, expected %.2f ± %.2f", ratio, expected, tolerance)
	}
}

func TestMissingPincodeRatio(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)
	registry := BuildRegistry(net, cfg)

	missingCount := 0
	for _, p := range registry.Poles {
		if p.Pincode == nil {
			missingCount++
		}
	}

	total := len(registry.Poles)
	ratio := float64(missingCount) / float64(total)
	expected := cfg.MissingPincodePct
	tolerance := 0.03

	if math.Abs(ratio-expected) > tolerance {
		t.Errorf("missing pincode ratio = %.2f, expected %.2f ± %.2f", ratio, expected, tolerance)
	}
}

func TestUniqueIDs(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	poleIDs := make(map[string]bool)
	for _, p := range net.GTPoles {
		if poleIDs[p.ID] {
			t.Errorf("duplicate pole ID: %s", p.ID)
		}
		poleIDs[p.ID] = true
	}

	dtIDs := make(map[string]bool)
	for _, dt := range net.Transformers {
		if dtIDs[dt.ID] {
			t.Errorf("duplicate DT ID: %s", dt.ID)
		}
		dtIDs[dt.ID] = true
	}

	feederIDs := make(map[string]bool)
	for _, f := range net.Feeders {
		if feederIDs[f.ID] {
			t.Errorf("duplicate feeder ID: %s", f.ID)
		}
		feederIDs[f.ID] = true
	}

	subIDs := make(map[string]bool)
	for _, s := range net.Substations {
		if subIDs[s.ID] {
			t.Errorf("duplicate substation ID: %s", s.ID)
		}
		subIDs[s.ID] = true
	}
}

func TestFKIntegrity(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	dtIDs := make(map[string]bool)
	for _, dt := range net.Transformers {
		dtIDs[dt.ID] = true
	}

	feederIDs := make(map[string]bool)
	for _, f := range net.Feeders {
		feederIDs[f.ID] = true
	}

	subIDs := make(map[string]bool)
	for _, s := range net.Substations {
		subIDs[s.ID] = true
	}

	for _, p := range net.GTPoles {
		if !dtIDs[p.DTID] {
			t.Errorf("pole %s references non-existent DT %s", p.ID, p.DTID)
		}
	}

	for _, dt := range net.Transformers {
		if !feederIDs[dt.FeederID] {
			t.Errorf("DT %s references non-existent feeder %s", dt.ID, dt.FeederID)
		}
	}

	for _, f := range net.Feeders {
		if !subIDs[f.SubstationID] {
			t.Errorf("feeder %s references non-existent substation %s", f.ID, f.SubstationID)
		}
	}
}

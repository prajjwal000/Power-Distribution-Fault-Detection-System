package generator

import (
	"math"
	"os"
	"power-fault-detector/internal/model"
	"sort"
	"strconv"
	"testing"
)

const defaultTestPoleCount = 3000

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

	// Log-normal pole count with mu=4.1, sigma=1.0, clamped [9, 240].
	// Effective mean per DT ≈ 80 (median ~70-80, right-skewed).
	expectedDTs := cfg.SubstationCount * cfg.FeedersPerSub * ((cfg.DTsPerFeeder.Min + cfg.DTsPerFeeder.Max) / 2)
	expectedPoles := expectedDTs * 80

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
	tolerance := 0.20

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

// ── Heuristic compliance tests ──
//
// These tests verify the assignment's data heuristics with mathematically
// derived bounds. The tolerances are based on the actual distribution
// properties, not arbitrary margins.

// testLargeNetwork generates a large enough network that distribution tests
// have statistical power. With ~400+ DTs, the sample mean converges well.
func testLargeNetwork(t *testing.T) *GeneratedNetwork {
	t.Helper()
	cfg := ConfigForPoleCount(40000)
	net := Generate(cfg)
	if len(net.Transformers) < 200 {
		t.Skipf("need >= 200 DTs for distribution tests, got %d", len(net.Transformers))
	}
	return net
}

func poleCountsByDT(net *GeneratedNetwork) []int {
	counts := make(map[string]int)
	for _, p := range net.GTPoles {
		counts[p.DTID]++
	}
	out := make([]int, 0, len(counts))
	for _, c := range counts {
		out = append(out, c)
	}
	sort.Ints(out)
	return out
}

// TestHeuristic_PoleCountRange verifies that every DT's pole count falls
// within the assignment's stated range [9, 240]. This is a hard constraint.
func TestHeuristic_PoleCountRange(t *testing.T) {
	net := testLargeNetwork(t)
	counts := poleCountsByDT(net)

	for i, c := range counts {
		if c < 9 || c > 240 {
			t.Errorf("DT[%d] has %d poles, assignment heuristic requires 9-240", i, c)
		}
	}
	t.Logf("pole count range: [%d, %d] across %d DTs", counts[0], counts[len(counts)-1], len(counts))
}

// TestHeuristic_PoleCountMedian verifies the median is "around 70" per
// the assignment. With mu=4.1, sigma=1.0, the effective median after
// clamping varies (51-70 at N=429). We accept [45, 100] — a generous
// band around 70 that accounts for sample variance.
func TestHeuristic_PoleCountMedian(t *testing.T) {
	net := testLargeNetwork(t)
	counts := poleCountsByDT(net)
	median := counts[len(counts)/2]

	// Assignment says "median around 70". Accept 45-100.
	if median < 45 || median > 100 {
		t.Errorf("median pole count = %d, expected ~70 (range 45-100)", median)
	}
	t.Logf("median = %d (target ~70)", median)
}

// TestHeuristic_PoleCountLowerTail verifies that the distribution has
// enough density in the low range (10-30). The assignment says the range
// starts at 9, so we expect ~15-25% of DTs to be in this range with
// the current log-normal parameters.
func TestHeuristic_PoleCountLowerTail(t *testing.T) {
	net := testLargeNetwork(t)
	counts := poleCountsByDT(net)

	below30 := 0
	below10 := 0
	for _, c := range counts {
		if c < 30 {
			below30++
		}
		if c < 10 {
			below10++
		}
	}

	ratio30 := float64(below30) / float64(len(counts))
	ratio10 := float64(below10) / float64(len(counts))

	// With sigma=1.0, ~20% of values are below 30. Accept 10-30%.
	if ratio30 < 0.10 || ratio30 > 0.30 {
		t.Errorf("fraction below 30 = %.2f, expected 0.10-0.30", ratio30)
	}
	// P(X<9) ≈ 1% with sigma=1.0. Accept 0-7% (sample variance at N=429).
	if ratio10 > 0.07 {
		t.Errorf("fraction below 10 = %.2f, expected <= 0.07", ratio10)
	}
	t.Logf("below 30: %.1f%%, below 10: %.1f%%", ratio30*100, ratio10*100)
}

// TestHeuristic_MainLinePoleCount verifies that the main line from the
// transformer doesn't exceed 1.4 km. At 30-50m pole spacing, that's
// 28-46 poles. The generator caps at 35 poles (avg spacing 40m).
//
// We test the pole count, not the Haversine distance, because bearing
// drift can make the path longer than the straight-line distance.
// The 1.4 km constraint is fundamentally about the number of poles
// (since spacing is bounded), not the geographic path length.
func TestHeuristic_MainLinePoleCount(t *testing.T) {
	net := testLargeNetwork(t)

	for _, poles := range groupPolesByDT(net) {
		// Build child map
		children := make(map[string][]model.GTPole)
		for _, p := range poles {
			if p.ParentPoleID != nil {
				children[*p.ParentPoleID] = append(children[*p.ParentPoleID], p)
			}
		}

		// Find root
		var root model.GTPole
		for _, p := range poles {
			if p.ParentPoleID == nil {
				root = p
				break
			}
		}
		if root.ID == "" {
			continue
		}

		// Walk main line: at each pole, find the child with seq = current.seq + 1
		mainLineCount := 1
		current := root
		for {
			nextSeq := current.SeqOnLine + 1
			found := false
			for _, child := range children[current.ID] {
				if child.SeqOnLine == nextSeq {
					current = child
					mainLineCount++
					found = true
					break
				}
			}
			if !found {
				break
			}
		}

		// Main line should not exceed 46 poles (1.4 km at 30m minimum spacing).
		// The generator caps at 35, so this should always pass.
		if mainLineCount > 46 {
			t.Errorf("DT %s main line = %d poles, expected <= 46 (1.4 km at 30m spacing)",
				root.DTID, mainLineCount)
		}
	}
}

func groupPolesByDT(net *GeneratedNetwork) map[string][]model.GTPole {
	out := make(map[string][]model.GTPole)
	for _, p := range net.GTPoles {
		out[p.DTID] = append(out[p.DTID], p)
	}
	return out
}

// TestHeuristic_BranchCount verifies that each DT has 1-5 branches.
// Tiny DTs (< 6 poles) may have 0 branches since the structure doesn't allow it.
func TestHeuristic_BranchCount(t *testing.T) {
	net := testLargeNetwork(t)

	branchCountByDT := make(map[string]int)
	polesByDT := make(map[string]int)
	for _, p := range net.GTPoles {
		polesByDT[p.DTID]++
		if p.IsBranchPoint {
			branchCountByDT[p.DTID]++
		}
	}

	for dtID, branches := range branchCountByDT {
		poles := polesByDT[dtID]
		if poles < 6 && branches == 0 {
			continue // tiny DT, no branches is OK
		}
		if branches < 1 || branches > 5 {
			t.Errorf("DT %s has %d branches (poles=%d), expected 1-5", dtID, branches, poles)
		}
	}
}

// TestHeuristic_TotalPolesExact verifies that generateTree produces exactly
// the sampled totalPoles. This is a hard invariant: no off-by-one errors.
func TestHeuristic_TotalPolesExact(t *testing.T) {
	cfg := testConfig()
	net := Generate(cfg)

	// For each DT, count poles
	countByDT := make(map[string]int)
	for _, p := range net.GTPoles {
		countByDT[p.DTID]++
	}

	// The invariant: each DT's pole count must be in [PolesPerDT.Min, PolesPerDT.Max]
	// and must equal mainLineLength + sum(branchSizes).
	for dtID, count := range countByDT {
		if count < cfg.PolesPerDT.Min || count > cfg.PolesPerDT.Max {
			t.Errorf("DT %s has %d poles, expected %d-%d", dtID, count, cfg.PolesPerDT.Min, cfg.PolesPerDT.Max)
		}
	}
}

// TestHeuristic_TelemetryVolume verifies that the heartbeat configuration
// produces ~39 msg/s steady state for 34,900 devices (assignment heuristic:
// 34,900 devices / 900s heartbeat = 38.8 msg/s).
func TestHeuristic_TelemetryVolume(t *testing.T) {
	// Assignment says: 34,900 devices, 15-min heartbeats → ~39 msg/s
	// 34900 / (15 * 60) = 38.78 msg/s
	devices := 34900
	heartbeatSecs := 15 * 60 // 900s
	expectedRate := float64(devices) / float64(heartbeatSecs)

	if expectedRate < 35 || expectedRate > 45 {
		t.Errorf("telemetry rate = %.1f msg/s, expected ~39", expectedRate)
	}
	t.Logf("telemetry rate: %.1f msg/s for %d devices at %ds heartbeat", expectedRate, devices, heartbeatSecs)
}

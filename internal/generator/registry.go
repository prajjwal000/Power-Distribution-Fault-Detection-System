package generator

import (
	"fmt"
	"math/rand"
	"power-fault-detector/internal/model"
)

type RegistryData struct {
	Poles            []model.Pole
	DTTopologyStatus []model.DTTopologyStatus
}

func BuildRegistry(net *GeneratedNetwork, cfg Config) *RegistryData {
	result := &RegistryData{}

	dtHasRecordedTopology := make(map[string]bool)
	for _, dt := range net.Transformers {
		hasTopology := rand.Float64() >= cfg.MissingTopologyPct
		dtHasRecordedTopology[dt.ID] = hasTopology
		result.DTTopologyStatus = append(result.DTTopologyStatus, model.DTTopologyStatus{
			DTID:        dt.ID,
			HasTopology: hasTopology,
		})
	}

	feederByDT := make(map[string]string)
	for _, dt := range net.Transformers {
		feederByDT[dt.ID] = dt.FeederID
	}

	for _, gtPole := range net.GTPoles {
		pole := buildRegistryPole(gtPole, dtHasRecordedTopology[gtPole.DTID], feederByDT[gtPole.DTID], cfg)
		result.Poles = append(result.Poles, pole)
	}

	return result
}

func buildRegistryPole(gt model.GTPole, dtHasTopology bool, feederID string, cfg Config) model.Pole {
	p := model.Pole{
		ID:       gt.ID,
		DTID:     gt.DTID,
		FeederID: feederID,
		Lat:      gt.Lat,
		Lon:      gt.Lon,
		PoleType: "LT-9m-PCC",
		Ward:     fmt.Sprintf("W-%03d", rand.Intn(100)+1),
	}

	if dtHasTopology {
		seq := gt.SeqOnLine
		p.SeqOnLine = &seq
		p.ParentPoleID = gt.ParentPoleID
	}

	if rand.Float64() >= cfg.NoDevicePct {
		deviceID := fmt.Sprintf("KSPDB-%s-%s", p.DTID, p.ID)
		p.DeviceID = &deviceID
	}

	if rand.Float64() >= cfg.MissingPincodePct {
		pincode := fmt.Sprintf("5600%02d", rand.Intn(100))
		p.Pincode = &pincode
	}

	return p
}

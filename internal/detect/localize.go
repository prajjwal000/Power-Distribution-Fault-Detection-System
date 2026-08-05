package detect

import (
	"sort"
)

type FaultCandidate struct {
	Scope       TicketScope
	TargetID    string
	AffectedPoles []string
	Confidence  float64
	Location    *Location
}

func Localize(job DetectionJob, topo *TopologyIndex) []FaultCandidate {
	if len(job.DarkPoles) == 0 {
		return nil
	}

	dtHasTopo := topo.HasKnownTopology(job.DTID)

	if dtHasTopo {
		return localizeKnownTopology(job, topo)
	}
	return localizeUnknownTopology(job, topo)
}

func localizeKnownTopology(job DetectionJob, topo *TopologyIndex) []FaultCandidate {
	type poleSeq struct {
		PoleID string
		Seq    int
		IsDark bool
	}

	var poles []poleSeq
	for _, pid := range job.DarkPoles {
		if seq, ok := topo.SeqOnLine(pid); ok {
			poles = append(poles, poleSeq{PoleID: pid, Seq: seq, IsDark: true})
		}
	}
	for _, pid := range job.LitPoles {
		if seq, ok := topo.SeqOnLine(pid); ok {
			poles = append(poles, poleSeq{PoleID: pid, Seq: seq, IsDark: false})
		}
	}

	sort.Slice(poles, func(i, j int) bool {
		return poles[i].Seq < poles[j].Seq
	})

	if len(poles) == 0 {
		return localizeUnknownTopology(job, topo)
	}

	type darkGroup struct {
		Start    int
		End      int
		PoleIDs  []string
	}
	var groups []darkGroup
	var current *darkGroup

	for _, p := range poles {
		if p.IsDark {
			if current == nil {
				current = &darkGroup{Start: p.Seq, PoleIDs: []string{p.PoleID}}
				groups = append(groups, *current)
				current = &groups[len(groups)-1]
			} else {
				current.PoleIDs = append(current.PoleIDs, p.PoleID)
			}
			current.End = p.Seq
		} else {
			current = nil
		}
	}

	if len(groups) == 0 {
		return nil
	}

	var candidates []FaultCandidate
	for _, g := range groups {
		faultSeq := g.Start - 1
		var targetID string
		if faultSeq == 0 {
			targetID = "transformer:" + job.DTID
		} else {
			var upstreamPole string
			for _, p := range poles {
				if p.Seq == faultSeq {
					upstreamPole = p.PoleID
					break
				}
			}
			if upstreamPole != "" && len(g.PoleIDs) > 0 {
				targetID = upstreamPole + "→" + g.PoleIDs[0]
			} else {
				targetID = "span:" + job.DTID
			}
		}

		allPolesInDT := topo.PolesForDT(job.DTID)
		devicePolesInDT := 0
		for _, pid := range allPolesInDT {
			if topo.PoleHasDevice(pid) {
				devicePolesInDT++
			}
		}

		expectedDark := 0
		for _, p := range poles {
			if p.Seq >= g.Start && p.IsDark {
				expectedDark++
			}
		}

		confidence := computeConfidence(
			len(g.PoleIDs),
			devicePolesInDT,
			expectedDark,
			len(job.DarkPoles),
			len(job.LitPoles),
			true,
		)

		var loc *Location
		if len(g.PoleIDs) > 0 {
			if p, ok := topo.PoleByID[g.PoleIDs[0]]; ok {
				loc = &Location{Lat: p.Lat, Lon: p.Lon}
			}
		}

		candidates = append(candidates, FaultCandidate{
			Scope:         ScopeSpan,
			TargetID:      targetID,
			AffectedPoles: g.PoleIDs,
			Confidence:    confidence,
			Location:      loc,
		})
	}

	return candidates
}

func localizeUnknownTopology(job DetectionJob, topo *TopologyIndex) []FaultCandidate {
	if len(job.DarkPoles) == 0 {
		return nil
	}

	allPoles := topo.PolesForDT(job.DTID)
	devicePoles := 0
	for _, pid := range allPoles {
		if topo.PoleHasDevice(pid) {
			devicePoles++
		}
	}

	allDark := len(job.DarkPoles) == devicePoles || len(job.DarkPoles) == len(allPoles)

	var scope TicketScope
	var targetID string
	var affected []string

	if allDark {
		scope = ScopeDT
		targetID = job.DTID
		affected = allPoles
	} else {
		scope = ScopeDT
		targetID = job.DTID
		affected = job.DarkPoles
	}

	confidence := computeConfidence(
		len(affected),
		devicePoles,
		len(job.DarkPoles),
		len(job.DarkPoles),
		len(job.LitPoles),
		false,
	)

	var loc *Location
	if len(job.DarkPoles) > 0 {
		if p, ok := topo.PoleByID[job.DarkPoles[0]]; ok {
			loc = &Location{Lat: p.Lat, Lon: p.Lon}
		}
	}

	return []FaultCandidate{{
		Scope:         scope,
		TargetID:      targetID,
		AffectedPoles: affected,
		Confidence:    confidence,
		Location:      loc,
	}}
}

func computeConfidence(
	affectedCount, deviceCount, expectedDark, reportedDark, litCount int,
	hasTopology bool,
) float64 {
	if deviceCount == 0 {
		return 0.3
	}

	ratio := float64(reportedDark) / float64(deviceCount)
	if ratio > 1.0 {
		ratio = 1.0
	}

	var base float64
	if hasTopology {
		base = 0.7
	} else {
		base = 0.5
	}

	topologyBonus := 0.0
	if hasTopology {
		if expectedDark > 0 && litCount == 0 {
			topologyBonus = 0.25
		} else if expectedDark > 0 {
			topologyBonus = 0.15
		}
	}

	reportingBonus := ratio * 0.1

	confidence := base + topologyBonus + reportingBonus
	if confidence > 0.99 {
		confidence = 0.99
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	return confidence
}

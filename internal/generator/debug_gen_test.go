package generator

import (
	"fmt"
	"os"
	"testing"
)

func TestDebugGeneration(t *testing.T) {
	if os.Getenv("GENERATOR_DEBUG") != "1" {
		t.Skip("set GENERATOR_DEBUG=1 to run")
	}
	cfg := ConfigForPoleCount(3000)
	fmt.Printf("Config: %d subs, %d feeders/sub, DTs %d-%d, Poles %d-%d\n",
		cfg.SubstationCount, cfg.FeedersPerSub,
		cfg.DTsPerFeeder.Min, cfg.DTsPerFeeder.Max,
		cfg.PolesPerDT.Min, cfg.PolesPerDT.Max)
	
	net := Generate(cfg)
	fmt.Printf("Generated: %d subs, %d feeders, %d DTs, %d poles\n",
		len(net.Substations), len(net.Feeders), len(net.Transformers), len(net.GTPoles))
}
